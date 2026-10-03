package worker

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Job adalah satu tugas berkala yang dijalankan scheduler.
type Job struct {
	Name      string                          // identitas tugas (dipakai untuk kunci singleton)
	Interval  time.Duration                   // jeda antar eksekusi
	Immediate bool                            // jalankan sekali langsung saat start
	Run       func(ctx context.Context) error // pekerjaan; error dicatat, tidak menghentikan loop
}

// Scheduler menjalankan sekumpulan Job secara berkala, masing-masing pada
// goroutine sendiri. Sebelum menjalankan tugas, kunci singleton diambil agar
// hanya satu instance worker yang mengeksekusi pada satu waktu.
type Scheduler struct {
	locker Locker
	loc    *time.Location
	jobs   []Job
}

// NewScheduler membangun scheduler. loc nil = UTC (dipakai jadwal harian).
func NewScheduler(locker Locker, loc *time.Location) *Scheduler {
	if loc == nil {
		loc = time.UTC
	}
	return &Scheduler{locker: locker, loc: loc}
}

// Location mengembalikan zona waktu scheduler (untuk jadwal harian).
func (s *Scheduler) Location() *time.Location {
	return s.loc
}

// Register mendaftarkan sebuah tugas. Interval <= 0 dinormalkan ke 1 menit.
func (s *Scheduler) Register(j Job) {
	if j.Interval <= 0 {
		j.Interval = time.Minute
	}
	s.jobs = append(s.jobs, j)
}

// Run menjalankan semua tugas sampai ctx dibatalkan, lalu menunggu semua
// goroutine tugas berhenti (graceful).
func (s *Scheduler) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, j := range s.jobs {
		wg.Add(1)
		go func(j Job) {
			defer wg.Done()
			s.runJob(ctx, j)
		}(j)
	}
	wg.Wait()
}

// runJob mengatur ticker satu tugas.
func (s *Scheduler) runJob(ctx context.Context, j Job) {
	log.Info().Str("job", j.Name).Dur("interval", j.Interval).Msg("Scheduler: tugas terdaftar")
	if j.Immediate {
		s.fire(ctx, j)
	}
	ticker := time.NewTicker(j.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fire(ctx, j)
		}
	}
}

// fire mengeksekusi satu tugas dengan pengaman kunci singleton (bila ada).
func (s *Scheduler) fire(ctx context.Context, j Job) {
	if ctx.Err() != nil {
		return
	}
	release := func() {}
	if s.locker != nil {
		rel, ok, err := s.locker.TryAcquire(ctx, j.Name)
		if err != nil {
			log.Warn().Err(err).Str("job", j.Name).Msg("Scheduler: gagal mengambil kunci tugas")
			return
		}
		if !ok {
			return // instance lain sedang menjalankan tugas ini
		}
		release = rel
	}
	defer release()

	if err := j.Run(ctx); err != nil {
		log.Warn().Err(err).Str("job", j.Name).Msg("Scheduler: tugas gagal")
	}
}
