// Package config reads every setting from environment variables.
// All of them are required; Load fails with a clear message if one is missing.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL string // DATABASE_URL

	PortalBaseURL  string  // PORTAL_BASE_URL
	PortalEmail    string  // PORTAL_EMAIL
	PortalPassword string  // PORTAL_PASSWORD
	PortalRPS      float64 // PORTAL_RPS: max requests per second to the portal

	Workers          int           // WORKERS: background worker goroutines
	MeterSyncEvery   time.Duration // METER_SYNC_EVERY: how often the full meter export runs
	EnergySyncEvery  time.Duration // ENERGY_SYNC_EVERY: how often each meter's readings are re-fetched
	EnergyCheckEvery time.Duration // ENERGY_CHECK_EVERY: how often the scheduler looks for due meters

	HTTPAddr string // HTTP_ADDR
}

// Load reads the environment. It reports every problem at once.
func Load() (Config, error) {
	var errs []error
	str := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is not set", key))
		}
		return v
	}
	dur := func(key string) time.Duration {
		d, err := time.ParseDuration(str(key))
		if err != nil && os.Getenv(key) != "" {
			errs = append(errs, fmt.Errorf("%s: %v (use e.g. 30s, 5m, 1h)", key, err))
		}
		return d
	}

	c := Config{
		DatabaseURL:      str("DATABASE_URL"),
		PortalBaseURL:    str("PORTAL_BASE_URL"),
		PortalEmail:      str("PORTAL_EMAIL"),
		PortalPassword:   str("PORTAL_PASSWORD"),
		MeterSyncEvery:   dur("METER_SYNC_EVERY"),
		EnergySyncEvery:  dur("ENERGY_SYNC_EVERY"),
		EnergyCheckEvery: dur("ENERGY_CHECK_EVERY"),
		HTTPAddr:         str("HTTP_ADDR"),
	}

	if v := str("PORTAL_RPS"); v != "" {
		rps, err := strconv.ParseFloat(v, 64)
		if err != nil || rps <= 0 {
			errs = append(errs, fmt.Errorf("PORTAL_RPS must be a positive number, got %q", v))
		}
		c.PortalRPS = rps
	}
	if v := str("WORKERS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			errs = append(errs, fmt.Errorf("WORKERS must be a whole number >= 1, got %q", v))
		}
		c.Workers = n
	}

	return c, errors.Join(errs...)
}
