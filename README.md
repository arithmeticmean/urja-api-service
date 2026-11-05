# Urja Meter API

A clean, documented API over the **Urja Meter Ops** portal: an internal web portal with no
API of its own.

The service logs in to the portal, copies its data (meters, their place in the network, and
half-hourly energy readings) into a Postgres database in the background, and serves it
through a small REST API. Anyone using the API never touches the portal.

| Document | What's in it |
|---|---|
| **README.md** (this file) | What it is, how to run it, how to use it |
| [PROTOCOL.md](PROTOCOL.md) | How the portal actually works, as I found it: login, endpoints, the hidden export, quirks |
| [ARCHITECTURE.md](ARCHITECTURE.md) | How the service is structured, design decisions and trade-offs, what I tried and rejected, what I left out, limits |
| [REFLECTION.md](REFLECTION.md) | Assumptions, the hardest part, my mistakes, what I'd improve with more time, self-review |
| [openapi.json](openapi.json) | OpenAPI 3.0 description of this API (also browsable at `/docs` when running) |

---

## Run it with Docker (recommended)

You only need **Docker** (with Docker Compose).

```bash
git clone <this repo>
cd urja-api-service

cp .env.example .env
# open .env and fill in PORTAL_EMAIL and PORTAL_PASSWORD (the portal login)

docker compose up --build
```

This starts three containers:

1. **db**: Postgres.
2. **migrate**: creates the tables, then exits.
3. **api**: the API on <http://localhost:8080>, plus the background sync.

Within a few seconds the log shows `"msg":"meters synced","meters":403`. Energy readings
start arriving about a minute later and take ~5 minutes to fill for all meters (the portal
only allows about 2 requests a second).

Then open **<http://localhost:8080/docs>** to browse and try the API.

Stop with `Ctrl+C`, or `docker compose down` (add `-v` to also delete the database).

## Run it without Docker (for development)

You need **Go 1.24+**, **make**, and Postgres (the easiest way is still the `db` container).

```bash
cp .env.example .env            # fill in PORTAL_EMAIL and PORTAL_PASSWORD
docker compose up -d db         # just the database
make migrate                    # create the tables (uses goose via `go run`)
make run                        # build and start the service
```

The Makefile loads `.env` automatically. Other targets: `make build`, `make test`,
`make migrate-down`, `make migration name=<name>`, `make clean`.

## Configuration

Every setting comes from an environment variable and **all of them are required**. There
are no hidden defaults. If one is missing, the service refuses to start and lists what's wrong.
`.env.example` has working values for everything except the login.

| Variable | Example | Meaning |
|---|---|---|
| `PORTAL_BASE_URL` | `https://urja-ops.flockenergy.tech` | The portal |
| `PORTAL_EMAIL` | | Portal login |
| `PORTAL_PASSWORD` | | Portal password |
| `PORTAL_RPS` | `1.5` | Max requests per second to the portal (it allows about 2) |
| `DATABASE_URL` | `postgres://urja:urja@localhost:5432/urja?sslmode=disable` | Postgres (Docker overrides this to use the `db` container) |
| `WORKERS` | `2` | Background workers |
| `METER_SYNC_EVERY` | `1h` | How often all meters are re-synced |
| `ENERGY_SYNC_EVERY` | `1h` | How old a meter's readings may get before they're re-fetched |
| `ENERGY_CHECK_EVERY` | `1m` | How often to look for meters whose readings are due |
| `HTTP_ADDR` | `:8080` | Where the API listens |

## Using the API

| Endpoint | What it returns |
|---|---|
| `GET /v1/meters` | Meters, 100 per page, with filters |
| `GET /v1/meters/{id}` | One meter, including its full network position and location |
| `GET /v1/meters/{id}/readings?from=&to=` | Half-hourly readings, with usage per half hour |
| `GET /healthz` | `{"status":"ok"}` |
| `GET /docs` | Interactive API docs (Swagger UI) |
| `GET /openapi.json` | The OpenAPI description |

**Filters on `/v1/meters`** (exact match, combine freely): `make`, `phaseType`,
`installStatus`, `installType`, `build`, `zone`, `circle`, `division`, `subdivision`,
`substation`, `feeder`, `dt`.
**Paging:** `limit` (1–1000, default 100) and `after`: pass the `next` value from one page to
get the following page.

### Sample requests

```bash
# Faulty meters in zone Z-01
curl 'http://localhost:8080/v1/meters?installStatus=Faulty&zone=Z-01&limit=2'
```

```json
{
  "data": [
    {
      "meterId": "J100018",
      "serialNo": "GE50943",
      "make": "HPL",
      "phaseType": "single",
      "installStatus": "Faulty",
      "installType": "CT Operated",
      "build": "v2",
      "dtCode": "DT-019",
      "hierarchy": {
        "zone":        { "code": "Z-01",  "name": "Jaipur Zone 1" },
        "circle":      { "code": "C-01",  "name": "Circle 1" },
        "division":    { "code": "D-09",  "name": "Division 9" },
        "subdivision": { "code": "SD-05", "name": "Subdivision 5" },
        "substation":  { "code": "SS-01", "name": "Substation 1" },
        "feeder":      { "code": "F-019", "name": "Feeder 19" },
        "dt":          { "code": "DT-019", "name": "Sikar Road DT 19" }
      },
      "geo": { "lat": 27.023530279896317, "lng": 75.74833874320397 }
    },
    { "meterId": "J100039", "...": "..." }
  ],
  "next": "J100039"
}
```

```bash
# One day of readings for one meter
curl 'http://localhost:8080/v1/meters/J100000/readings?from=2026-06-24&to=2026-06-25'
```

```json
{
  "meterId": "J100000",
  "data": [
    { "ts": "2026-06-24T00:00:00+05:30", "kwh": "…", "kvah": "…", "voltR": "…", "kwhDelta": "…", "kvahDelta": "…" },
    { "ts": "2026-06-24T00:30:00+05:30", "kwh": "…", "kvah": "…", "voltR": "…", "kwhDelta": "…", "kvahDelta": "…" }
  ]
}
```

- `kwh` / `kvah` are the meter's running totals, as the portal reports them. `kwhDelta` /
  `kvahDelta` are the **usage in that half hour**, worked out by the service.
- Numbers are strings so no precision is lost.
- Errors are always `{"error": "..."}` with status 400 (bad input), 404 (unknown meter) or 500.

## Testing

```bash
make test
```

The `client` tests run against the **real portal** using `PORTAL_EMAIL` / `PORTAL_PASSWORD`
from `.env`, and are skipped if those aren't set. `TestSign` always runs: it checks the export
signature against a value produced with `openssl`.

## Troubleshooting

| Problem | Fix |
|---|---|
| Service exits with `X is not set` | Fill in that variable in `.env` |
| `login failed ... Invalid email or password` | Check `PORTAL_EMAIL` / `PORTAL_PASSWORD` |
| `port 5432 is already allocated` | Another Postgres is running. Stop it, or change the left side of `5432:5432` in `docker-compose.yml` |
| `job failed ... HTTP 429` in logs | The portal is rate limiting. Lower `PORTAL_RPS` |
