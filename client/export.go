package client

import (
	"context"
	"encoding/json"
	"strconv"
	"time"
)

// ExportMeter is one row of /portal/export, sent by urja portal.
type ExportMeter struct {
	MeterID       string `json:"meterId"`
	SerialNo      string `json:"serialNo"`
	Make          string `json:"make"`
	PhaseType     string `json:"phaseType"`
	InstallStatus string `json:"installStatus"`
	InstallType   string `json:"installType"`
	Build         string `json:"build"`
	DTCode        string `json:"dtCode"`
	Hierarchy     struct {
		Zone        Node `json:"zone"`
		Circle      Node `json:"circle"`
		Division    Node `json:"division"`
		Subdivision Node `json:"subdivision"`
		Substation  Node `json:"substation"`
		Feeder      Node `json:"feeder"`
		DT          Node `json:"dt"`
	} `json:"hierarchy"`
	Geo struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"geo"`
}

type Node struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

// Export returns every meter from the signed export, page by page until `total`.
func (c *Client) Export(ctx context.Context) ([]ExportMeter, error) {
	body, err := c.get(ctx, "/portal/keys", "", nil)
	if err != nil {
		return nil, err
	}
	var keys struct {
		Data struct {
			SigningSecret string `json:"signingSecret"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &keys); err != nil {
		return nil, err
	}
	secret := keys.Data.SigningSecret

	var all []ExportMeter
	for page := 1; ; page++ {
		query := "page=" + strconv.Itoa(page)
		body, err := c.get(ctx, "/portal/export", query, func() map[string]string {
			ts := time.Now().Unix()
			return map[string]string{
				"x-timestamp": strconv.FormatInt(ts, 10),
				"x-signature": Sign(secret, "GET", "/portal/export", query, ts),
			}
		})
		if err != nil {
			return nil, err
		}

		var resp struct {
			Data []ExportMeter `json:"data"`
		}

		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Data...)

		return all, nil
	}
}
