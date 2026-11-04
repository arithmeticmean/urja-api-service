// Package workers runs the background work: syncing meters and fetching
// energy readings from the portal into the database.
package workers

// Kind says what a job does.
type Kind string

const (
	SyncMeters  Kind = "sync-meters"  // fetch the export and upsert every meter
	FetchEnergy Kind = "fetch-energy" // fetch one meter's readings and save them
)

// Job is one unit of work. The pool calls Done when it has finished.
type Job struct {
	Kind    Kind
	MeterID string // FetchEnergy only
	Size    int    // FetchEnergy only: response size last time (0 = never fetched)

	done func()
}

func (j Job) Done() {
	if j.done != nil {
		j.done()
	}
}

// key identifies a job, so the same work isn't queued twice while it's in flight.
func (j Job) key() string {
	if j.Kind == SyncMeters {
		return "meters"
	}
	return "energy:" + j.MeterID
}
