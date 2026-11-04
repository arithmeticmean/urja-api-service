package workers

import (
	"context"
	"log/slog"
)

// StartPool starts n goroutines that take jobs from `jobs` and process them
// with w. They stop when ctx is cancelled.
func StartPool(ctx context.Context, n int, jobs <-chan Job, w *Worker) {
	for range n {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case job := <-jobs:
					if err := w.Process(ctx, job); err != nil {
						slog.Warn("job failed", "kind", job.Kind, "meter", job.MeterID, "err", err)
					}
					job.Done()
				}
			}
		}()
	}
}
