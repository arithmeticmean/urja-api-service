package repositories

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"urja-api-service/models"
)

// DueMeter is a meter whose readings should be fetched again.
type DueMeter struct {
	MeterID string
	Size    int // energy response size last time (0 = never fetched)
}

// DueMeters returns meters never fetched, or last fetched more than `every` ago.
func (r *Repo) DueMeters(ctx context.Context, every time.Duration) ([]DueMeter, error) {
	rows, err := r.db.Query(ctx, `
		SELECT meter_id, COALESCE(energy_size, 0)
		FROM meters
		WHERE energy_synced_at IS NULL OR energy_synced_at < now() - $1::interval
		ORDER BY energy_synced_at NULLS FIRST`, every)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (DueMeter, error) {
		var m DueMeter
		err := row.Scan(&m.MeterID, &m.Size)
		return m, err
	})
}

// Each reading's delta is its value minus the meter's previous reading.
// Rows are inserted oldest first, so the previous one is always already there.
const insertReading = `
INSERT INTO energy_readings (meter_id, ts, kwh, kvah, volt_r, kwh_delta, kvah_delta)
SELECT $1, $2, $3::numeric, $4::numeric, NULLIF($5, '')::numeric,
       $3::numeric - prev.kwh, $4::numeric - prev.kvah
FROM (SELECT 1) AS one
LEFT JOIN LATERAL (
    SELECT kwh, kvah FROM energy_readings
    WHERE meter_id = $1 AND ts < $2
    ORDER BY ts DESC LIMIT 1
) AS prev ON true
ON CONFLICT (meter_id, ts) DO NOTHING`

// SaveReadings stores new readings (skipping ones already saved) and records
// the response size and sync time, in one transaction. `readings` may be empty.
func (r *Repo) SaveReadings(ctx context.Context, meterID string, readings []models.Reading, size int) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		for _, rd := range readings {
			if _, err := tx.Exec(ctx, insertReading, meterID, rd.TS, rd.KWh, rd.KVAh, rd.VoltR); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx,
			`UPDATE meters SET energy_size = $2, energy_synced_at = now() WHERE meter_id = $1`,
			meterID, size)
		return err
	})
}
