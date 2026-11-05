// Package controllers turns HTTP requests into service calls and results into JSON.
package controllers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"urja-api-service/repositories"
	"urja-api-service/services"
)

type MeterController struct {
	service *services.MeterService
}

func NewMeterController(s *services.MeterService) *MeterController {
	return &MeterController{service: s}
}

// GET /v1/meters?installStatus=Faulty&zone=Z-01&limit=100&after=J100099
func (c *MeterController) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 0
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be a number")
			return
		}
		limit = n
	}
	filters := map[string]string{}
	for key := range q {
		if key != "limit" && key != "after" {
			filters[key] = q.Get(key)
		}
	}

	page, err := c.service.List(r.Context(), filters, q.Get("after"), limit)
	if err != nil {
		handleError(w, err)
		return
	}
	var next *string
	if page.Next != "" {
		next = &page.Next
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": page.Meters, "next": next})
}

// GET /v1/meters/{id}
func (c *MeterController) Get(w http.ResponseWriter, r *http.Request) {
	meter, err := c.service.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": meter})
}

// GET /v1/meters/{id}/readings?from=2026-06-24&to=2026-06-25
// from/to accept a date (2026-06-24, Indian time) or a full RFC 3339 timestamp.
func (c *MeterController) Readings(w http.ResponseWriter, r *http.Request) {
	from, err := parseTime(r.URL.Query().Get("from"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "from: "+err.Error())
		return
	}
	to, err := parseTime(r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "to: "+err.Error())
		return
	}

	id := r.PathValue("id")
	readings, err := c.service.Readings(r.Context(), id, from, to)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"meterId": id, "data": readings})
}

// GET /healthz
func Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------- helpers ----------

var ist = time.FixedZone("IST", 5*3600+30*60)

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, ist); err == nil {
		return t, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, errors.New("use YYYY-MM-DD or RFC 3339, e.g. 2026-06-24T10:00:00+05:30")
	}
	return t, nil
}

func handleError(w http.ResponseWriter, err error) {
	var invalid services.ErrInvalid
	switch {
	case errors.As(err, &invalid):
		writeError(w, http.StatusBadRequest, invalid.Msg)
	case errors.Is(err, repositories.ErrNotFound):
		writeError(w, http.StatusNotFound, "meter not found")
	default:
		slog.Error("request failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
