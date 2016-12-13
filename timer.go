// Copyright (c) 2009 The Go Authors. All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package timer

import (
	"time"

	"github.com/uber-go/timer/twheel"
)

// The Timer type represents a single event.
// When the Timer expires, the current time will be sent on C,
// unless the Timer was created by AfterFunc.
// A Timer must be created with NewTimer or AfterFunc.
type Timer struct {
	C  <-chan time.Time
	to twheel.Timeout
}

var wheel = twheel.NewTimeoutWheel()

// Stop prevents the Timer from firing.
// It returns true if the call stops the timer, false if the timer has already
// expired or been stopped.
// Stop does not close the channel, to prevent a read from the channel succeeding
// incorrectly.
//
// To prevent the timer firing after a call to Stop,
// check the return value and drain the channel. For example:
// 	if !t.Stop() {
// 		<-t.C
// 	}
// This cannot be done concurrent to other receives from the Timer's
// channel.
func (t *Timer) Stop() bool {
	return t.to.Stop()
}

func sendTime(arg interface{}) {
	c := arg.(chan time.Time)
	c <- time.Now()
}

// New creates a new Timer that will send
// the current time on its channel after at least duration d.
func NewTimer(d time.Duration) *Timer {
	c := make(chan time.Time, 1)
	to, err := wheel.Schedule(d, sendTime, c)
	if err != nil {
		return nil
	}

	return &Timer{C: c, to: to}
}

// Reset changes the timer to expire after duration d.
// It returns true if the timer had been active, false if the timer had
// expired or been stopped.
//
// To reuse an active timer, always call its Stop method first and—if it had
// expired—drain the value from its channel. For example:
// 	if !t.Stop() {
// 		<-t.C
// 	}
// 	t.Reset(d)
// This should not be done concurrent to other receives from the Timer's
// channel.
//
// Note that it is not possible to use Reset's return value correctly, as there
// is a race condition between draining the channel and the new timer expiring.
// Reset should always be used in concert with Stop, as described above.
// The return value exists to preserve compatibility with existing programs.
func (t *Timer) Reset(d time.Duration) bool {
	active := t.to.Stop()
	to, _ := wheel.Schedule(d, sendTime, t.C)
	t.to = to
	return active
}

// After waits for the duration to elapse and then sends the current time
// on the returned channel.
// It is equivalent to NewTimer(d).C.
// The underlying Timer is not recovered by the garbage collector
// until the timer fires. If efficiency is a concern, use NewTimer
// instead and call Timer.Stop if the timer is no longer needed.
func After(d time.Duration) <-chan time.Time {
	return NewTimer(d).C
}

func callFunc(arg interface{}) {
	f := arg.(func())
	go f()
}

// AfterFunc waits for the duration to elapse and then calls f
// in its own goroutine. It returns a Timer that can
// be used to cancel the call using its Stop method.
func AfterFunc(d time.Duration, f func()) *Timer {
	to, err := wheel.Schedule(d, callFunc, f)
	if err != nil {
		return nil
	}

	return &Timer{to: to}
}
