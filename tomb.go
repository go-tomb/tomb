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

// The tomb package helps with clean goroutine termination.
//
// See the Tomb type for details.
package tomb

import (
	"errors"
	"fmt"
	"sync"
)

// A Tomb tracks the lifecycle of a goroutine as alive, dying or dead,
// and the reason for its death.
//
// The zero value of a Tomb assumes that a goroutine is about to be
// created or already alive. Once Stop is called with an
// argument that informs the reason for death, the goroutine is in
// a dying state and is expected to terminate soon. Right before the
// goroutine function or method returns, Done must be called to inform
// that the goroutine is indeed dead and about to stop running.
//
// A Tomb exposes Dying and Dead channels. These channels are closed
// when the Tomb state changes in the respective way. They enable
// explicit blocking until the state changes, and also to selectively
// unblock select statements accordingly.
//
// For background and a detailed example, see the following blog post:
//
//   http://blog.labix.org/2011/10/09/death-of-goroutines-under-control
//
type Tomb struct {
	m      sync.Mutex
	state  state
	dying  chan struct{}
	dead   chan struct{}
	reason error
}

type state int

const (
	uninitialized = state(iota)
	running
	dying
)

var errCleanStop = errors.New("clean stop")
var ErrStillRunning = errors.New("tomb: goroutine is still running")

func (t *Tomb) init() {
	t.m.Lock()
	if t.state == uninitialized {
		t.dead = make(chan struct{})
		t.dying = make(chan struct{})
		t.state = running
	}
	t.m.Unlock()
}

// Dead returns the channel that can be used to wait
// until t.Done has been called.
func (t *Tomb) Dead() <-chan struct{} {
	t.init()
	return t.dead
}

// Dying returns the channel that can be used to wait
// until t.Kill or t.Done has been called.
func (t *Tomb) Dying() <-chan struct{} {
	t.init()
	return t.dying
}

// Wait blocks until the goroutine is in a dead state and returns the
// reason for its death.
func (t *Tomb) Wait() error {
	t.init()
	<-t.dead
	if t.reason == errCleanStop {
		return nil
	}
	return t.reason
}

// Done flags the goroutine as dead, and should be called a single time
// right before the goroutine function or method returns.
// If the goroutine was not already in a dying state before Done is
// called, it will be flagged as dying and dead at once with no
// error.
func (t *Tomb) Done() {
	t.init()
	t.Kill(nil)
	close(t.dead)
}

// Kill flags the goroutine as dying for the given reason.
// Kill may be called multiple times, but only the first non-nil error is
// recorded as the reason for termination.
func (t *Tomb) Kill(reason error) {
	t.init()
	if reason == nil {
		reason = errCleanStop
	}
	t.m.Lock()
	if t.reason == nil || t.reason == errCleanStop {
		t.reason = reason
	}
	if t.state == running {
		close(t.dying)
		t.state = dying
	}
	t.m.Unlock()
}

// Killf works like Kill, but builds the reason providing the received
// arguments to fmt.Errorf. The generated error is also returned.
func (t *Tomb) Killf(f string, a ...interface{}) error {
	err := fmt.Errorf(f, a...)
	t.Kill(err)
	return err
}

// Err returns the reason for the goroutine death provided via Stop
// or Fatalf, or ErrStillRunning when the goroutine is still alive.
func (t *Tomb) Err() (reason error) {
	t.init()
	t.m.Lock()
	defer t.m.Unlock()
	switch {
	case t.state == running:
		return ErrStillRunning
	case t.reason == errCleanStop:
		return nil
	}
	return t.reason
}
