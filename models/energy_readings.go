package models

import "time"

// Reading is one half-hourly meter reading.
// KWh and KVAh are cumulative registers, kept as text so no precision is lost.
type Reading struct {
	TS        time.Time `json:"ts"`
	KWh       string    `json:"kwh"`
	KVAh      string    `json:"kvah"`
	VoltR     string    `json:"voltR"`
	KWhDelta  *string   `json:"kwhDelta"`
	KVAhDelta *string   `json:"kvahDelta"`
}
