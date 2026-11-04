package workers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"urja-api-service/client"
	"urja-api-service/models"
	"urja-api-service/repositories"
)

// Worker knows how to do every kind of job.
type Worker struct {
	client *client.Client
	repo   *repositories.Repo
}

func New(c *client.Client, repo *repositories.Repo) *Worker {
	return &Worker{client: c, repo: repo}
}

func (w *Worker) Process(ctx context.Context, job Job) error {
	switch job.Kind {
	case SyncMeters:
		return w.syncMeters(ctx)
	case FetchEnergy:
		return w.fetchEnergy(ctx, job.MeterID, job.Size)
	}
	return fmt.Errorf("unknown job kind %q", job.Kind)
}

// syncMeters fetches the export and upserts every meter.
func (w *Worker) syncMeters(ctx context.Context) error {
	exported, err := w.client.Export(ctx)
	if err != nil {
		return err
	}
	meters := make([]models.Meter, len(exported))
	for i, e := range exported {
		meters[i] = toMeter(e)
	}
	if err := w.repo.UpsertMeters(ctx, meters); err != nil {
		return err
	}
	slog.Info("meters synced", "meters", len(meters))
	return nil
}

// fetchEnergy fetches one meter's readings and saves them.
func (w *Worker) fetchEnergy(ctx context.Context, meterID string, previousSize int) error {
	raw, size, err := w.client.Energy(ctx, meterID, previousSize) // raw is nil if the size didn't change
	if err != nil {
		return err
	}
	return w.repo.SaveReadings(ctx, meterID, toReadings(raw), size)
}

// ---------- converting the portal's shapes into ours ----------

// toMeter: blank strings from the portal become nil.
func toMeter(e client.ExportMeter) models.Meter {
	h := e.Hierarchy
	return models.Meter{
		MeterID:       e.MeterID,
		SerialNo:      e.SerialNo,
		Make:          e.Make,
		PhaseType:     e.PhaseType,
		InstallStatus: e.InstallStatus,
		InstallType:   e.InstallType,
		Build:         e.Build,
		DTCode:        e.DTCode,
		Hierarchy: models.Hierarchy{
			Zone:        node(h.Zone),
			Circle:      node(h.Circle),
			Division:    node(h.Division),
			Subdivision: node(h.Subdivision),
			Substation:  node(h.Substation),
			Feeder:      node(h.Feeder),
			DT:          node(h.DT),
		},
		Geo: models.Geo{Lat: e.Geo.Lat, Lng: e.Geo.Lng},
	}
}

func node(n client.Node) models.Node {
	return models.Node{Code: nilIfBlank(n.Code), Name: nilIfBlank(n.Name)}
}

func nilIfBlank(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// The portal's timestamps are "DD/MM/YYYY HH:mm" in Indian time (no DST).
var ist = time.FixedZone("IST", 5*3600+30*60)

func toReadings(raw []client.EnergyReading) []models.Reading {
	var out []models.Reading
	for _, r := range raw {
		ts, err := time.ParseInLocation("02/01/2006 15:04", r.Timestamp, ist)
		if err != nil || r.KWh == "" || r.KVAh == "" {
			continue // skip rows we can't use
		}
		out = append(out, models.Reading{TS: ts, KWh: r.KWh, KVAh: r.KVAh, VoltR: r.VoltR})
	}
	return out
}
