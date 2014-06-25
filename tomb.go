// Copyright (c) 2011 - Gustavo Niemeyer <gustavo@niemeyer.net>
// 
// All rights reserved.
// 
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
// 
//     * Redistributions of source code must retain the above copyright notice,
//       this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above copyright notice,
//       this list of conditions and the following disclaimer in the documentation
//       and/or other materials provided with the distribution.
//     * Neither the name of the copyright holder nor the names of its
//       contributors may be used to endorse or promote products derived from
//       this software without specific prior written permission.
// 
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR
// CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL,
// EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
// PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF
// LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
// NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// The tomb package handles clean goroutine tracking and termination.
//
// A Tomb value tracks the lifecycle of one or more goroutines as alive,
// dying or dead, and the reason for their death.
//
// The zero value of a Tomb is ready to handle the creation of one
// or more tracked goroutines via its Go method, and these goroutines
// may call the Go method again to create additional tracked goroutines
// at any point in their execution.
// 
// If any of the tracked goroutines returns a non-nil error, or the
// Kill or Killf method is called by any goroutine in the system (tracked
// or not), the tomb Err is set, Alive is set to false, and the Dying
// channel is closed to flag that all tracked goroutines are supposed
// to willingly terminate as soon as possible.
//
// Once all tracked goroutines terminate, the Dead channel is closed,
// and Wait unblocks and returns the first non-nil error presented
// to the tomb via a result or an explicit Kill or Killf method call,
// or nil if there were no errors.
//
// Tracked functions and methods that are still running while the tomb
// is in dying mode may choose to return ErrDying as their error value.
// This preserves the well established non-nil error convention, but is
// understood by the tomb as a clean termination. The Err and Wait
// methods will still return nil if all observed errors were either
// nil or ErrDying.
//
// All tomb methods are concurrency-safe. The main non-obvious race
// to be aware about is that calling the Go method twice on a new
// tomb value may lead the second goroutine to never run if the
// first one returns too early and puts the tomb into dead mode.
// If unintended, avoiding this behavior by providing both goroutine
// functions to the same Go method call.
//
// For background and a detailed example, see the following blog post:
//
//   http://blog.labix.org/2011/10/09/death-of-goroutines-under-control
//
// For a more complex code snippet demonstrating the use of multiple
// goroutines with a single Tomb, see:
//
//   http://play.golang.org/p/Xh7qWsDPZP
//
package tomb

import (
	"errors"
	"fmt"
	"sync"
)

// A Tomb tracks the lifecycle of one or more goroutines as alive,
// dying or dead, and the reason for their death.
//
// See the package documentation for details.
type Tomb struct {
	m      sync.Mutex
	alive  int
	dying  chan struct{}
	dead   chan struct{}
	reason error
}

var (
	ErrStillAlive = errors.New("tomb: still alive")
	ErrDying = errors.New("tomb: dying")
)

func (t *Tomb) init() {
	t.m.Lock()
	if t.dead == nil {
		t.dead = make(chan struct{})
		t.dying = make(chan struct{})
		t.reason = ErrStillAlive
	}
	t.m.Unlock()
}

// Dead returns the channel that can be used to wait until
// all goroutines have finished running.
func (t *Tomb) Dead() <-chan struct{} {
	t.init()
	return t.dead
}

// Dying returns the channel that can be used to wait until
// t.Kill is called.
func (t *Tomb) Dying() <-chan struct{} {
	t.init()
	return t.dying
}

// Wait blocks until all goroutines have finished running, and
// then returns the reason for their death.
func (t *Tomb) Wait() error {
	t.init()
	<-t.dead
	t.m.Lock()
	reason := t.reason
	t.m.Unlock()
	return reason
}

// Go runs the f function as a concurrent goroutine if
// the tomb is not already in dead mode.
//
// If f returns a non-nil error, or f is the last goroutine alive
// to return, t.Kill is called with its result as an argument.
//
// It is f's responsibility to monitor the tomb state and
// return appropriately once it is put in dying mode.
func (t *Tomb) Go(f ...func() error) {
	t.init()
	t.m.Lock()
	defer t.m.Unlock()
	select {
	case <-t.dead:
		return
	default:
	}
	t.alive += len(f)
	for _, fi := range f {
		go t.run(fi)
	}
}

func (t *Tomb) run(f func() error) {
	err := f()
	t.m.Lock()
	defer t.m.Unlock()
	t.alive--
	if t.alive == 0 || err != nil {
		t.kill(err)
		if t.alive == 0 {
			close(t.dead)
		}
	}
}

// Kill flags the goroutine as dying for the given reason.
// Kill may be called multiple times, but only the first
// non-nil error is recorded as the reason for termination.
//
// If reason is ErrDying, the previous reason isn't replaced
// even if it is nil. It's a runtime error to call Kill with
// ErrDying if t is not in a dying state.
func (t *Tomb) Kill(reason error) {
	t.init()
	t.m.Lock()
	defer t.m.Unlock()
	t.kill(reason)
}

func (t *Tomb) kill(reason error) {
	if reason == ErrStillAlive {
		panic("tomb: Kill with ErrStillAlive")
	}
	if reason == ErrDying {
		if t.reason == ErrStillAlive {
			panic("tomb: Kill with ErrDying while still alive")
		}
		return
	}
	if t.reason == ErrStillAlive {
		t.reason = reason
		close(t.dying)
		return
	}
	if t.reason == nil {
		t.reason = reason
		return
	}
}

// Killf works like Kill, but builds the reason providing the received
// arguments to fmt.Errorf. The generated error is also returned.
func (t *Tomb) Killf(f string, a ...interface{}) error {
	err := fmt.Errorf(f, a...)
	t.Kill(err)
	return err
}

// Err returns the reason for the goroutine death provided via Kill
// or Killf, or ErrStillAlive if the goroutine is still alive.
func (t *Tomb) Err() (reason error) {
	t.init()
	t.m.Lock()
	reason = t.reason
	t.m.Unlock()
	return
}

// Alive returns whether the goroutine is still alive.
func (t *Tomb) Alive() bool {
	return t.Err() == ErrStillAlive
}
