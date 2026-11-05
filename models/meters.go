// Package models holds the service's own types.
package models

// Meter is one energy meter.
type Meter struct {
	MeterID       string    `json:"meterId"`
	SerialNo      string    `json:"serialNo"`
	Make          string    `json:"make"`
	PhaseType     string    `json:"phaseType"`     // "single" or "three"
	InstallStatus string    `json:"installStatus"` // "Installed", "Faulty" or "Decommissioned"
	InstallType   string    `json:"installType"`
	Build         string    `json:"build"`
	DTCode        string    `json:"dtCode"`
	Hierarchy     Hierarchy `json:"hierarchy"`
	Geo           Geo       `json:"geo"`
}

// Hierarchy is where the meter sits in the network, from zone down to its
// distribution transformer (DT).
type Hierarchy struct {
	Zone        Node `json:"zone"`
	Circle      Node `json:"circle"`
	Division    Node `json:"division"`
	Subdivision Node `json:"subdivision"`
	Substation  Node `json:"substation"`
	Feeder      Node `json:"feeder"`
	DT          Node `json:"dt"`
}

// Node is one level of the hierarchy. Code or Name is nil when the portal left it blank.
type Node struct {
	Code *string `json:"code"`
	Name *string `json:"name"`
}

type Geo struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}
