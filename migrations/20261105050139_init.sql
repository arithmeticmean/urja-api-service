-- +goose Up

-- Current meter data, overwritten on every sync.
CREATE TABLE meters (
    meter_id          TEXT PRIMARY KEY,
    serial_no         TEXT NOT NULL,
    make              TEXT NOT NULL,
    phase_type        TEXT NOT NULL,
    install_status    TEXT NOT NULL,
    install_type      TEXT,
    build             TEXT,
    zone_code         TEXT,
    zone_name         TEXT,
    circle_code       TEXT,
    circle_name       TEXT,
    division_code     TEXT,
    division_name     TEXT,
    subdivision_code  TEXT,
    subdivision_name  TEXT,
    substation_code   TEXT,
    substation_name   TEXT,
    feeder_code       TEXT,
    feeder_name       TEXT,
    dt_code           TEXT,
    dt_name           TEXT,
    lat               DOUBLE PRECISION,
    lng               DOUBLE PRECISION,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    energy_size       INT,            -- energy response size last time; same size = nothing new
    energy_synced_at  TIMESTAMPTZ     -- NULL = readings never fetched
);

-- Half-hourly readings. kwh / kvah are cumulative registers;
-- kwh_delta / kvah_delta are the usage in that half hour.
CREATE TABLE energy_readings (
    meter_id    TEXT NOT NULL REFERENCES meters (meter_id),
    ts          TIMESTAMPTZ NOT NULL,
    kwh         NUMERIC NOT NULL,
    kvah        NUMERIC NOT NULL,
    volt_r      NUMERIC,
    kwh_delta   NUMERIC,
    kvah_delta  NUMERIC,
    PRIMARY KEY (meter_id, ts)
);

-- +goose Down
DROP TABLE readings;
DROP TABLE meters;
