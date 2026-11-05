// Package services holds the rules between HTTP and the database:
// which filters are allowed, defaults and limits.
package services

import (
	"context"
	"fmt"
	"time"

	"urja-api-service/models"
	"urja-api-service/repositories"
)

// ErrInvalid wraps a problem with the caller's input (becomes HTTP 400).
type ErrInvalid struct{ Msg string }

func (e ErrInvalid) Error() string { return e.Msg }

// Filters the API accepts on /v1/meters, and the column each one maps to.
var meterFilters = map[string]string{
	"make":          "make",
	"phaseType":     "phase_type",
	"installStatus": "install_status",
	"installType":   "install_type",
	"build":         "build",
	"zone":          "zone_code",
	"circle":        "circle_code",
	"division":      "division_code",
	"subdivision":   "subdivision_code",
	"substation":    "substation_code",
	"feeder":        "feeder_code",
	"dt":            "dt_code",
}

const (
	defaultLimit = 100
	maxLimit     = 1000
)

type MeterService struct {
	repo *repositories.Repo
}

func NewMeterService(repo *repositories.Repo) *MeterService {
	return &MeterService{repo: repo}
}

// MeterPage is one page of meters. Next is the `after` value for the next page, "" if none.
type MeterPage struct {
	Meters []models.Meter
	Next   string
}

// List returns meters matching `filters` (API names, e.g. "installStatus").
func (s *MeterService) List(ctx context.Context, filters map[string]string, after string, limit int) (MeterPage, error) {
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 1 || limit > maxLimit {
		return MeterPage{}, ErrInvalid{fmt.Sprintf("limit must be between 1 and %d", maxLimit)}
	}

	equals := map[string]string{}
	for name, value := range filters {
		col, ok := meterFilters[name]
		if !ok {
			return MeterPage{}, ErrInvalid{fmt.Sprintf("unknown filter %q", name)}
		}
		equals[col] = value
	}

	meters, err := s.repo.ListMeters(ctx, repositories.MeterFilter{Equals: equals, After: after, Limit: limit})
	if err != nil {
		return MeterPage{}, err
	}
	if meters == nil {
		meters = []models.Meter{} // JSON [] rather than null
	}
	page := MeterPage{Meters: meters}
	if len(meters) == limit {
		page.Next = meters[len(meters)-1].MeterID
	}
	return page, nil
}

func (s *MeterService) Get(ctx context.Context, id string) (models.Meter, error) {
	return s.repo.GetMeter(ctx, id)
}

// Readings returns a meter's readings between from and to. Zero times mean
// "from the beginning" / "until now".
func (s *MeterService) Readings(ctx context.Context, id string, from, to time.Time) ([]models.Reading, error) {
	if _, err := s.repo.GetMeter(ctx, id); err != nil {
		return nil, err // ErrNotFound for unknown meters
	}
	if from.IsZero() {
		from = time.Unix(0, 0)
	}
	if to.IsZero() {
		to = time.Now().Add(24 * time.Hour)
	}
	if !from.Before(to) {
		return nil, ErrInvalid{"from must be before to"}
	}
	readings, err := s.repo.ListReadings(ctx, id, from, to)
	if readings == nil && err == nil {
		readings = []models.Reading{} // JSON [] rather than null
	}
	return readings, err
}
