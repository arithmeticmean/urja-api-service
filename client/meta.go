package client

import (
	"context"
	"encoding/json"
)

// Meta is what the first page of the meter search reports about the whole list
// this gives us very important information total no. of meters which exports doesn't have
type Meta struct {
	Total    int `json:"total"`    // number of meters in the portal
	PageSize int `json:"pageSize"` // rows per search page for now it is 20
}

// use it to see how many meters exist, or whether the
// count changed since the last sync.
func (c *Client) Meta(ctx context.Context) (Meta, error) {
	body, err := c.get(ctx, "/portal/meters/search", "q=&page=1", nil)
	if err != nil {
		return Meta{}, err
	}
	var m Meta
	if err := json.Unmarshal(body, &m); err != nil {
		return Meta{}, err
	}
	return m, nil
}
