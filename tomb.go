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
	"fmt"
	"os"
	"sync"
)

// A Tomb tracks the lifecycle of a goroutine as alive, dying or dead,
// and the reason for its death.
//
// The initial state of a Tomb informs that a goroutine is about to be
// created or already alive. Once Fatal or Fatalf is called with an
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
	dying  chan struct{}
	dead   chan struct{}
	Dying  <-chan struct{}
	Dead   <-chan struct{}
	reason os.Error
}

// The Stop error is used as a reason for a goroutine to stop cleanly.
var Stop = os.NewError("clean stop")

// New creates a new Tomb to track the lifecycle of a goroutine
// that is already alive or about to be created.
func New() *Tomb {
	t := &Tomb{dying: make(chan struct{}), dead: make(chan struct{})}
	t.Dying = t.dying
	t.Dead = t.dead
	return t
}

// Wait blocks until the goroutine is in a dead state and returns the
// reason for its death. If the reason is Stop, nil is returned.
func (t *Tomb) Wait() os.Error {
	<-t.Dead
	if t.reason == Stop {
		return nil
	}
	return t.reason
}

// Done flags the goroutine as dead, and should be called a single time
// right before the goroutine function or method returns.
// If the goroutine was not already in a dying state before Done is
// called, it will flagged as dying and dead at once with Stop as the
// reason for death.
func (t *Tomb) Done() {
	t.Fatal(Stop)
	close(t.dead)
}

// Fatal flags the goroutine as dying for the given reason.
// Fatal may be called multiple times, but only the first error is
// recorded as the reason for termination.
// The Stop value may be used to terminate a goroutine cleanly.
func (t *Tomb) Fatal(reason os.Error) {
	if reason == nil {
		panic("Fatal with nil reason")
	}
	t.m.Lock()
	if t.reason == nil || t.reason == Stop {
		t.reason = reason
	}
	select {
	case <-t.Dying:
	default:
		close(t.dying)
	}
	t.m.Unlock()
}

// Fatalf works like Fatal, but builds the reason providing the received
// arguments to fmt.Errorf. The generated error is also returned.
func (t *Tomb) Fatalf(format string, args ...interface{}) os.Error {
	err := fmt.Errorf(format, args...)
	t.Fatal(err)
	return err
}

// Err returns the reason for the goroutine death provided via Fatal
// or Fatalf, or nil in case the goroutine is still alive.
func (t *Tomb) Err() (reason os.Error) {
	t.m.Lock()
	reason = t.reason
	t.m.Unlock()
	return
}
