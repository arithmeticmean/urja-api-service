package client

import (
	"context"
	"encoding/json"
	"net/url"
)

// EnergyReading is a time series data with time delta of 30 min
type EnergyReading struct {
	Timestamp string `json:"timestamp"` // "DD/MM/YYYY HH:mm", IST
	KWh       string `json:"kwh"`
	KVAh      string `json:"kvah"`
	VoltR     string `json:"voltR"`
}

// Energy returns a meter's readings and the response size.
// If the size equals previousSize
// readings are time series which implies append only
// so we can skip it
// it returns nil readings without parsing.
func (c *Client) Energy(ctx context.Context, meterID string, previousSize int) ([]EnergyReading, int, error) {
	body, err := c.get(ctx, "/portal/meters/"+url.PathEscape(meterID)+"/energy", "", nil)
	if err != nil {
		return nil, 0, err
	}
	if len(body) == previousSize {
		return nil, previousSize, nil // size of both data is same so there is no use of json parsing
	}
	var resp struct {
		Data []EnergyReading `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, err
	}
	return resp.Data, len(body), nil
}
