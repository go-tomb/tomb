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

type nothing struct{}

// A Tomb tracks the lifecycle of a goroutine as alive, dying or dead,
// and the reason for its death.
//
// The clean state of a Tomb informs that a goroutine is about to be
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
type Tomb struct {
	m      sync.Mutex
	Dying  chan nothing
	Dead   chan nothing
	reason os.Error
}

// New creates a new Tomb to track the lifecycle of a goroutine
// that is already alive or about to be created.
func New() *Tomb {
	return &Tomb{Dying: make(chan nothing), Dead: make(chan nothing)}
}

// IsDying returns true if the goroutine is in a dying or already dead state.
func (t *Tomb) IsDying() bool {
	select {
	case <-t.Dying:
		return true
	default:
	}
	return false
}

// IsDead returns true if the goroutine is in a dead state.
func (t *Tomb) IsDead() bool {
	select {
	case <-t.Dead:
		return true
	default:
	}
	return false
}

// Wait blocks until the goroutine is in a dead state and returns the
// reason for its death. The reason may be nil.
func (t *Tomb) Wait() os.Error {
	<-t.Dead
	return t.reason
}

// Done puts the goroutine in a dead state, and should be called a
// single time right before the goroutine function or method returns.
// If the goroutine was not already in a dying state before Done is
// called, it will flagged as dying and dead at once.
func (t *Tomb) Done() {
	t.Fatal(nil)
	close(t.Dead)
}

// Fatal puts the goroutine in a dying state.
// The first non-nil reason parameter to Fatal or the first Fatalf-generated
// error is recorded as the reason for the goroutine death.
// This method may be safely called concurrently, and may be called both from
// within the goroutine and/or from outside to request the goroutine termination.
func (t *Tomb) Fatal(reason os.Error) {
	t.m.Lock()
	if t.reason == nil {
		t.reason = reason
	}
	select {
	case <-t.Dying:
	default:
		close(t.Dying)
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

// Err returns the reason for the goroutine death provided via Fatal or Fatalf.
func (t *Tomb) Err() (reason os.Error) {
	t.m.Lock()
	reason = t.reason
	t.m.Unlock()
	return
}
