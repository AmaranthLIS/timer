# Alternative Timer System for Go #

Included is an alternative implementation of a timer system for Go. It uses a
calendar queue to keep track of upcoming events and a configurable tick to
traverse the wheel that defaults to one millisecond (1ms). Despite this being
much coarser than the nanosecond level of the Go standard library, it is
perfectly adequate for most applications.

The timer system itself operates with zero-allocations by caching all allocated
Timeout structures.

## Motivation ##

As of 1.7.4, the timer system of the Go standard library is based on tickless
priority queue data structure. This priority queue is ordered by time in order
to track how long to sleep until the next callout.

Unfortunately, such a design has a few drawbacks. Firstly, the two most common
operations of a timer system, inserting and deleting, are of O(log(n)) time
complexity. Secondly, a priority queue is difficult to lock without using a
global lock which is what the system uses.

```
$ go test -bench=.
BenchmarkInsertDelete-8         	50000000	        30.9 ns/op
BenchmarkStdlibInsertDelete-8   	 5000000	       275 ns/op
```

The impact of this lock contention becomes even more apparent in actual
high-performance clients. The following was taken from a client/server
communicating over UNIX domain sockets:
```
P50(ns):  25068 over 5m requests


(pprof) top
Showing top 10 nodes out of 50 (cum >= 4.89s)
      flat  flat%   sum%        cum   cum%
     7.32s 31.95% 31.95%      7.58s 33.09%  syscall.Syscall
     4.54s 19.82% 51.77%      4.54s 19.82%  runtime.mach_semaphore_signal
     4.02s 17.55% 69.31%      4.02s 17.55%  runtime.kevent
     2.84s 12.40% 81.71%      2.84s 12.40%  runtime.mach_semaphore_timedwait
     1.96s  8.56% 90.27%      1.96s  8.56%  runtime.usleep
     1.74s  7.59% 97.86%      1.74s  7.59%  runtime.freedefer
     0.01s 0.044% 97.90%      4.16s 18.16%  runtime.findrunnable
     0.01s 0.044% 97.95%      4.11s 17.94%  syscall.Write
         0     0% 97.95%      4.36s 19.03%  context.WithDeadline
         0     0% 97.95%      4.89s 21.34%  encoding/binary.Read
```

Drilling down, we see that we actuall are spending all the time contending on the single mutex:

```
ROUTINE ======================== runtime.mach_semrelease in /usr/local/Cellar/go/1.7.3/libexec/src/runtime/os_darwin.go
         OUTINE ======================== time.startTimer in /usr/local/Cellar/go/1.7.3/libexec/src/runtime/time.go
         0      4.26s (flat, cum) 18.59% of Total
         .          .     63://go:linkname startTimer time.startTimer
         .          .     64:func startTimer(t *timer) {
         .          .     65:	if raceenabled {
         .          .     66:		racerelease(unsafe.Pointer(t))
         .          .     67:	}
         .      4.26s     68:	addtimer(t)


...


         .          .    454:}
         .          .    455:
         .          .    456://go:nosplit
         .          .    457:func mach_semrelease(sem uint32) {
         .          .    458:	for {
         .      4.45s    459:		r := mach_semaphore_signal(sem)

```


## Usage API ##

For simplicity, this faster timer system can be used with practically the same
Go standard API. Simply import "github.com/uber-go/timer" into your project and
off you go.
