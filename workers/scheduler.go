package workers

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"urja-api-service/repositories"
)

// Scheduler decides when work happens and puts jobs on the channel:
//   - a SyncMeters job every `meterEvery` (e.g. 1h or 4h)
//   - every `energyTick` (e.g. 1m), a FetchEnergy job for each meter whose
//     readings are older than `energyEvery` (e.g. 1h)
//
// A job that is already queued or running is not queued again.
type Scheduler struct {
	repo        *repositories.Repo
	meterEvery  time.Duration
	energyEvery time.Duration
	energyTick  time.Duration
	jobs        chan Job

	mu       sync.Mutex
	inFlight map[string]bool
}

func NewScheduler(repo *repositories.Repo, meterEvery, energyEvery, energyTick time.Duration) *Scheduler {
	return &Scheduler{
		repo:        repo,
		meterEvery:  meterEvery,
		energyEvery: energyEvery,
		energyTick:  energyTick,
		jobs:        make(chan Job),
		inFlight:    map[string]bool{},
	}
}

// Jobs is the channel the worker pool reads from.
func (s *Scheduler) Jobs() <-chan Job {
	return s.jobs
}

// Run schedules work until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	go s.every(ctx, s.meterEvery, s.scheduleMeterSync)
	s.every(ctx, s.energyTick, s.scheduleEnergy)
}

// every calls fn now, then on each tick.
func (s *Scheduler) every(ctx context.Context, d time.Duration, fn func(context.Context)) {
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		fn(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Scheduler) scheduleMeterSync(ctx context.Context) {
	s.enqueue(ctx, Job{Kind: SyncMeters})
}

func (s *Scheduler) scheduleEnergy(ctx context.Context) {
	due, err := s.repo.DueMeters(ctx, s.energyEvery)
	if err != nil {
		slog.Warn("loading due meters failed", "err", err)
		return
	}
	for _, m := range due {
		s.enqueue(ctx, Job{Kind: FetchEnergy, MeterID: m.MeterID, Size: m.Size})
	}
}

// enqueue sends the job unless the same job is already in flight.
// It blocks until a worker takes it (or shutdown).
func (s *Scheduler) enqueue(ctx context.Context, job Job) {
	key := job.key()
	s.mu.Lock()
	if s.inFlight[key] {
		s.mu.Unlock()
		return
	}
	s.inFlight[key] = true
	s.mu.Unlock()

	job.done = func() {
		s.mu.Lock()
		delete(s.inFlight, key)
		s.mu.Unlock()
	}

	select {
	case s.jobs <- job:
	case <-ctx.Done():
		job.Done()
	}
}
