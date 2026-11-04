package repositories

import (
	"context"

	"github.com/jackc/pgx/v5"

	"urja-api-service/models"
)

const upsertMeter = `
INSERT INTO meters (
    meter_id, serial_no, make, phase_type, install_status, install_type, build,
    zone_code, zone_name, circle_code, circle_name, division_code, division_name,
    subdivision_code, subdivision_name, substation_code, substation_name,
    feeder_code, feeder_name, dt_code, dt_name, lat, lng, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23, now())
ON CONFLICT (meter_id) DO UPDATE SET
    serial_no = EXCLUDED.serial_no, make = EXCLUDED.make, phase_type = EXCLUDED.phase_type,
    install_status = EXCLUDED.install_status, install_type = EXCLUDED.install_type, build = EXCLUDED.build,
    zone_code = EXCLUDED.zone_code, zone_name = EXCLUDED.zone_name,
    circle_code = EXCLUDED.circle_code, circle_name = EXCLUDED.circle_name,
    division_code = EXCLUDED.division_code, division_name = EXCLUDED.division_name,
    subdivision_code = EXCLUDED.subdivision_code, subdivision_name = EXCLUDED.subdivision_name,
    substation_code = EXCLUDED.substation_code, substation_name = EXCLUDED.substation_name,
    feeder_code = EXCLUDED.feeder_code, feeder_name = EXCLUDED.feeder_name,
    dt_code = EXCLUDED.dt_code, dt_name = EXCLUDED.dt_name,
    lat = EXCLUDED.lat, lng = EXCLUDED.lng, updated_at = now()`

// UpsertMeters inserts new meters and overwrites existing ones, in one transaction.
func (r *Repo) UpsertMeters(ctx context.Context, meters []models.Meter) error {
	batch := &pgx.Batch{}
	for _, m := range meters {
		h := m.Hierarchy
		batch.Queue(upsertMeter,
			m.MeterID, m.SerialNo, m.Make, m.PhaseType, m.InstallStatus, m.InstallType, m.Build,
			h.Zone.Code, h.Zone.Name, h.Circle.Code, h.Circle.Name, h.Division.Code, h.Division.Name,
			h.Subdivision.Code, h.Subdivision.Name, h.Substation.Code, h.Substation.Name,
			h.Feeder.Code, h.Feeder.Name, m.DTCode, h.DT.Name, m.Geo.Lat, m.Geo.Lng)
	}
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		return tx.SendBatch(ctx, batch).Close()
	})
}
