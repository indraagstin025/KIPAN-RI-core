package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeLocker adalah Locker in-memory untuk test scheduler.
type fakeLocker struct {
	allow bool
	mu    sync.Mutex
	calls int
}

// TryAcquire selalu mengembalikan keputusan tetap (allow) dan mencatat panggilan.
func (f *fakeLocker) TryAcquire(_ context.Context, _ string) (func(), bool, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return func() {}, f.allow, nil
}

// waitFor menunggu hingga cond() true atau timeout.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(2 * time.Millisecond)
	}
	return cond()
}

// TestSchedulerRunsImmediateJob memastikan tugas Immediate dieksekusi sekali
// segera setelah start.
func TestSchedulerRunsImmediateJob(t *testing.T) {
	var n int32
	s := NewScheduler(&fakeLocker{allow: true}, time.UTC)
	s.Register(Job{Name: "x", Interval: time.Hour, Immediate: true, Run: func(context.Context) error {
		atomic.AddInt32(&n, 1)
		return nil
	}})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()

	if !waitFor(t, time.Second, func() bool { return atomic.LoadInt32(&n) >= 1 }) {
		t.Fatal("tugas Immediate tidak dijalankan")
	}
	cancel()
	<-done
}

// TestSchedulerRunsOnInterval memastikan tugas berulang dijalankan tiap interval.
func TestSchedulerRunsOnInterval(t *testing.T) {
	var n int32
	s := NewScheduler(&fakeLocker{allow: true}, time.UTC)
	s.Register(Job{Name: "y", Interval: 10 * time.Millisecond, Run: func(context.Context) error {
		atomic.AddInt32(&n, 1)
		return nil
	}})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()

	if !waitFor(t, time.Second, func() bool { return atomic.LoadInt32(&n) >= 2 }) {
		t.Fatalf("tugas interval tidak berulang, n=%d", atomic.LoadInt32(&n))
	}
	cancel()
	<-done
}

// TestSchedulerSkipsWhenLockHeld memastikan tugas TIDAK dijalankan bila kunci
// singleton dipegang instance lain.
func TestSchedulerSkipsWhenLockHeld(t *testing.T) {
	var n int32
	locker := &fakeLocker{allow: false}
	s := NewScheduler(locker, time.UTC)
	s.Register(Job{Name: "z", Interval: 5 * time.Millisecond, Immediate: true, Run: func(context.Context) error {
		atomic.AddInt32(&n, 1)
		return nil
	}})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()

	time.Sleep(60 * time.Millisecond)
	cancel()
	<-done

	if got := atomic.LoadInt32(&n); got != 0 {
		t.Fatalf("tugas seharusnya di-skip saat kunci dipegang, tapi n=%d", got)
	}
	if locker.calls == 0 {
		t.Fatal("locker seharusnya tetap dicoba")
	}
}

// TestLockKeyStable memastikan kunci turunan stabil dan berbeda antar-nama.
func TestLockKeyStable(t *testing.T) {
	if lockKey("a") != lockKey("a") {
		t.Fatal("lockKey tidak stabil untuk nama sama")
	}
	if lockKey("a") == lockKey("b") {
		t.Fatal("lockKey harus berbeda untuk nama berbeda")
	}
}

// TestResolveLocation memastikan fallback UTC untuk zona waktu tidak valid.
func TestResolveLocation(t *testing.T) {
	if loc := resolveLocation(""); loc != time.UTC {
		t.Fatalf("zona waktu kosong harus UTC, dapat %v", loc)
	}
	if loc := resolveLocation("Asia/Jakarta"); loc.String() != "Asia/Jakarta" {
		t.Fatalf("zona waktu valid salah, dapat %v", loc)
	}
	if loc := resolveLocation("Tidak/Ada"); loc != time.UTC {
		t.Fatalf("zona waktu tidak valid harus fallback UTC, dapat %v", loc)
	}
}
