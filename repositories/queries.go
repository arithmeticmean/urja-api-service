package repositories

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"urja-api-service/models"
)

// ErrNotFound is returned when a meter doesn't exist.
var ErrNotFound = errors.New("not found")

// MeterFilter narrows down ListMeters. Empty fields are ignored.
type MeterFilter struct {
	Equals map[string]string // column -> value, e.g. "install_status" -> "Faulty"
	After  string            // return meters with meter_id > After (pagination)
	Limit  int
}

const meterColumns = `meter_id, serial_no, make, phase_type, install_status,
    COALESCE(install_type, ''), COALESCE(build, ''), COALESCE(dt_code, ''),
    zone_code, zone_name, circle_code, circle_name, division_code, division_name,
    subdivision_code, subdivision_name, substation_code, substation_name,
    feeder_code, feeder_name, dt_code, dt_name, COALESCE(lat, 0), COALESCE(lng, 0)`

func scanMeter(row pgx.Row) (models.Meter, error) {
	var m models.Meter
	h := &m.Hierarchy
	err := row.Scan(&m.MeterID, &m.SerialNo, &m.Make, &m.PhaseType, &m.InstallStatus,
		&m.InstallType, &m.Build, &m.DTCode,
		&h.Zone.Code, &h.Zone.Name, &h.Circle.Code, &h.Circle.Name, &h.Division.Code, &h.Division.Name,
		&h.Subdivision.Code, &h.Subdivision.Name, &h.Substation.Code, &h.Substation.Name,
		&h.Feeder.Code, &h.Feeder.Name, &h.DT.Code, &h.DT.Name, &m.Geo.Lat, &m.Geo.Lng)
	return m, err
}

// ListMeters returns meters matching the filter, ordered by meter_id.
// Column names in f.Equals must come from a fixed list (the service checks this).
func (r *Repo) ListMeters(ctx context.Context, f MeterFilter) ([]models.Meter, error) {
	var where []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.Replace(cond, "?", "$"+strconv.Itoa(len(args)), 1))
	}
	for col, v := range f.Equals {
		add(col+" = ?", v)
	}
	if f.After != "" {
		add("meter_id > ?", f.After)
	}

	sql := "SELECT " + meterColumns + " FROM meters"
	if len(where) > 0 {
		sql += " WHERE " + strings.Join(where, " AND ")
	}
	args = append(args, f.Limit)
	sql += " ORDER BY meter_id LIMIT $" + strconv.Itoa(len(args))

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.Meter, error) {
		return scanMeter(row)
	})
}

func (r *Repo) GetMeter(ctx context.Context, id string) (models.Meter, error) {
	m, err := scanMeter(r.db.QueryRow(ctx, "SELECT "+meterColumns+" FROM meters WHERE meter_id = $1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return m, ErrNotFound
	}
	return m, err
}

// ListReadings returns a meter's readings with from <= ts < to, oldest first.
func (r *Repo) ListReadings(ctx context.Context, meterID string, from, to time.Time) ([]models.Reading, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ts, kwh::text, kvah::text, COALESCE(volt_r::text, ''), kwh_delta::text, kvah_delta::text
		FROM energy_readings
		WHERE meter_id = $1 AND ts >= $2 AND ts < $3
		ORDER BY ts`, meterID, from, to)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.Reading, error) {
		var rd models.Reading
		err := row.Scan(&rd.TS, &rd.KWh, &rd.KVAh, &rd.VoltR, &rd.KWhDelta, &rd.KVAhDelta)
		return rd, err
	})
}
