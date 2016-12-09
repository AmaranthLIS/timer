// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package twheel

import (
	"math/rand"
	"sync/atomic"
	"testing"
	"time"
)

func TestLinkedList(t *testing.T) {
	var list timeoutList

	timeout1 := &timeout{}
	timeout2 := &timeout{}

	timeout1.prependLocked(&list)
	timeout2.prependLocked(&list)

	if list.head != timeout2 {
		t.FailNow()
		return
	}

	timeout2.removeLocked()
	if list.head != timeout1 {
		t.FailNow()
		return
	}
	timeout1.removeLocked()
	if list.head != nil {
		t.FailNow()
		return
	}
}

func TestStartStop(t *testing.T) {
	tw := NewTimeoutWheel()
	for i := 0; i < 10; i++ {
		tw.Stop()
		time.Sleep(5 * time.Millisecond)
		tw.Start()
	}
	tw.Stop()
}

func TestStartStopConcurrent(t *testing.T) {
	tw := NewTimeoutWheel()

	for i := 0; i < 10; i++ {
		tw.Stop()
		go tw.Start()
		time.Sleep(1 * time.Millisecond)
	}
	tw.Stop()
}

func TestExpire(t *testing.T) {
	tw := NewTimeoutWheel()
	ch := make(chan int, 3)

	_, err := tw.Schedule(20*time.Millisecond, func(_ interface{}) { ch <- 20 }, nil)
	if err != nil {
		t.Fail()
		return
	}

	_, err = tw.Schedule(10*time.Millisecond, func(_ interface{}) { ch <- 10 }, nil)
	if err != nil {
		t.Fail()
		return
	}

	_, err = tw.Schedule(5*time.Millisecond, func(_ interface{}) { ch <- 5 }, nil)
	if err != nil {
		t.Fail()
		return
	}

	output := make([]int, 0, 3)
	for i := 0; i < 3; i++ {
		d := time.Duration(((i+1)*5)+1) * time.Millisecond
		select {
		case d := <-ch:
			output = append(output, d)
		case <-time.After(d):
			t.Fail()
			return
		}
	}
	if output[0] != 5 || output[1] != 10 || output[2] != 20 {
		t.Fail()
	}
	tw.Stop()
}

func BenchmarkInsertDelete(b *testing.B) {
	tw := NewTimeoutWheel(WithLocksExponent(11))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for pb.Next() {
			timeout, _ := tw.Schedule(time.Second+time.Millisecond*time.Duration(r.Intn(1000)),
				nil, nil)
			timeout.Stop()
		}
	})
	tw.Stop()
}

func BenchmarkReplacement(b *testing.B) {
	// uses resevoir sampling to randomly replace timers, so that some have a
	// chance of expiring
	tw := NewTimeoutWheel(WithLocksExponent(11))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		mine := make([]Timeout, 0, 10000)
		n, k := 0, 0

		// fill
		for pb.Next() && len(mine) < cap(mine) {
			dur := time.Second + time.Millisecond*time.Duration(r.Intn(1000))
			n++

			timeout, _ := tw.Schedule(dur, nil, nil)
			mine = append(mine, timeout)
			k++
		}

		// replace randomly
		for pb.Next() {
			dur := time.Second + time.Millisecond*time.Duration(r.Intn(1000))
			n++

			if r.Float32() <= float32(k)/float32(n) {
				i := r.Intn(len(mine))
				timeout := mine[i]
				timeout.Stop()
				mine[i], _ = tw.Schedule(dur, nil, nil)
				k++
			}
		}
	})
	tw.Stop()
}

func BenchmarkExpiration(b *testing.B) {
	tw := NewTimeoutWheel()
	d := time.Millisecond
	b.ResetTimer()
	var sum int64
	f := func(interface{}) {
		atomic.AddInt64(&sum, 1)
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tw.Schedule(d, f, nil)
		}
	})
	//time.Sleep(5 * time.Millisecond)
	tw.Stop()
}