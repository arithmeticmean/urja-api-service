# Architecture

## The shape of it

The portal is slow to read from, rate limited, and logs you out every hour. None of that
should be the problem of whoever uses this API. So the service keeps **its own copy of the
portal's data in Postgres**, refreshes it in the background, and answers every API request
from that copy.

```
            ┌─────────────────────────────────────────────────────┐
            │                  urja-api-service                   │
            │                                                     │
  Portal ◄──┼── client ◄── workers ──► repositories ──► Postgres  │
            │                                            ▲        │
  Users ────┼── routes → controllers → services ─────────┘        │
            │                                                     │
            └─────────────────────────────────────────────────────┘
```

One process does two jobs that never call each other. The **workers** write to the
database; the **API** only reads from it. The database is the only thing they share.

## The pattern

This is a known pattern, not something invented for this task:

- The service is an **anti-corruption layer**: a clean API in front of a legacy system, so
  the legacy system's quirks (the login, the paging, the signatures, the rate limit) stay
  hidden behind it.
- It is kept up to date by **change data capture (CDC) through polling**. The usual kind of
  CDC reads a database's own change log, which we don't have access to. When all you have
  is an API, the alternative is to ask it again on a schedule and store what's new or
  different. That's what the workers do.

Two good introductions:

- [Anti-corruption Layer pattern](https://learn.microsoft.com/en-us/azure/architecture/patterns/anti-corruption-layer) (Microsoft Azure Architecture Center)
- [Change Data Capture: The Magic Wand We Forgot](https://www.youtube.com/watch?v=ZAZJqEKUl3U) (Martin Kleppmann, Berlin Buzzwords 2015)

## Getting data in

All background work goes through one **scheduler** and one **pool of workers**. The
scheduler decides *what* needs doing and *when*; the workers do whatever job they're handed.

There are two kinds of job:

- **Sync meters.** Every `METER_SYNC_EVERY` (1 hour), fetch the portal's export and save all
  403 meters. It's two requests, so there's no reason to be clever about it.
- **Fetch energy.** Every `ENERGY_CHECK_EVERY` (1 minute), the scheduler asks the database
  which meters haven't had their readings fetched in the last `ENERGY_SYNC_EVERY` (1 hour),
  and queues a job for each. A worker fetches that meter's readings and saves the new ones.

The database records when each meter's readings were last fetched (`energy_synced_at`).
That one column does a lot of work: after a restart there's nothing to rebuild, a new meter
is due straight away, and a fetch that fails is still due on the next check a minute later,
so it is retried without any retry code.

The scheduler also remembers which jobs are already queued or running, so a slow job is
never queued twice.

## Answering a request

```
GET /v1/meters?installStatus=Faulty&zone=Z-01
   routes → middleware → controllers → services → repositories → Postgres → JSON
```

Each layer does one thing. The **middleware** logs the request. The **controller** reads the
request and writes the response. The **service** holds the rules: which filters exist, the
largest allowed page, date defaults. The **repository** holds the SQL. Nothing in this path
calls the portal.

## Code layout

| Folder | Responsibility |
|---|---|
| `client/` | The only code that talks to the portal: login, rate limit, retries, export signing |
| `workers/` | Background work: scheduler, worker pool, and the two jobs |
| `repositories/` | All SQL |
| `services/` | API rules and validation |
| `controllers/` | HTTP in, JSON out |
| `routes/`, `middleware/` | Endpoint list and request logging |
| `models/` | The service's own types, kept separate from the portal's |
| `config/` | Settings, all from environment variables |
| `migrations/` | The database schema |

The database has two tables: `meters` (one row per meter, overwritten on each sync) and
`energy_readings` (one row per meter per half hour).

## Decisions worth explaining

### Store a copy, don't proxy

The obvious design is a thin proxy: an API request comes in, the service asks the portal,
reshapes the answer and sends it back. I rejected it because every API user would then share
the portal's limit of about 120 requests a minute, every request would be as slow as the
portal, and the API would go down whenever the portal did. Readings rule it out anyway:
there's no way to answer "readings for these meters" without one portal request per meter.

Keeping the copy only in memory was also out: it would be empty after every restart, and
the first full load of readings takes a few minutes.

### Use the hidden export for meters

The Meters page lists 20 meters at a time, so a full read is 21 requests, and meters can
shift between pages while you're reading them. Before I found the export I worked through
ways around that: checking page boundaries for shifted rows, re-reading only the last page
for new meters, refreshing a few pages at a time in rotation, and hashing pages to spot
changes. Each helped a little, but none could avoid downloading a page to find out whether
it had changed.

The export made all of that unnecessary. Two requests return every meter at once, with more
fields than the list, as one consistent snapshot. The portal's own button only ever asks for
page 1; the service keeps asking until a page comes back empty, so it won't silently lose
meters once there are more than one page holds.

### Keep the current state of meters only

Each meter sync overwrites the meter's row with what the portal says now. The service does
not keep a history of meter changes (for example, when a meter went from Installed to
Faulty), because the brief asks for current data. Readings are different: they are only ever
added, so the `energy_readings` table is naturally a full history.

### One scheduler, one worker pool

I went through three designs before this one. A **Postgres job table** with row locking would
let several processes share the work and survive crashes mid-job, but that's a lot of
machinery for one process and 403 meters. An **in-memory queue with a timer per meter**
worked, but was hard to reason about. **Two separate loops** for meters and energy were
simple, but used two different patterns for the same kind of problem.

What's left is the plainest version: one place decides timing, one pool does the work, and
the database remembers what's been done.

### Skip energy responses that haven't changed

The portal sends a meter's entire reading history on every request and offers no
"last changed" header. Since readings are only ever added, a response of the same size as
last time can't contain anything new, so the worker doesn't parse or insert readings; it
only records that the meter was checked. Inserting is also safe to repeat: a reading that
already exists is skipped.

I considered parsing only the new bytes at the end of each response, but it would break the
moment the portal changed its formatting, to save very little work.

### Limit the request rate in one place

My first full run fetched energy as fast as possible and hit the portal's limit after about
128 requests. Now the client holds a single rate limiter (`golang.org/x/time/rate`, set by
`PORTAL_RPS`, 1.5 requests a second in `.env.example`), and every request from every worker
waits its turn there. If the portal still answers 429, the client waits (for `Retry-After`,
or 30 seconds) before trying again. Workers don't know the limit exists; they just
occasionally wait.

This only really matters for the first full load, which takes about four and a half minutes.
After that, hourly refreshes need around seven requests a minute.

### Smaller choices

- **Usage is stored, not just meter totals.** The portal reports ever-growing totals. When
  saving, Postgres also stores the difference from the previous reading (`kwh_delta`,
  `kvah_delta`), so users get half-hourly usage directly. Numbers stay exact decimals throughout.
- **Network position is stored per meter, not as a tree.** Above the feeder level, the same
  code appears under different parents, so a tree would either reject real data or merge
  unrelated branches.
- **Pagination uses `?after=<meterId>`**, not page numbers, so results don't shift when
  meters are added.
- **Blank values from the portal become `null`.**
- **Every setting is a required environment variable.** If one is missing, the service
  refuses to start and says which.

## What I left out

- **History of meter changes**: see [Keep the current state of meters only](#keep-the-current-state-of-meters-only).
- **Transformers as their own endpoint.** Every meter already includes its transformer's code
  and name.
- **Aggregated consumption** (e.g. usage per day per feeder). Useful for data teams, but
  per-meter readings cover what was asked.
- **Authentication and rate limiting on this API.** Not asked for; each would be one middleware.
- **Metrics and a web front-end.**

## Limits

- **Run one copy at a time.** The scheduler's record of queued and running jobs lives in the
  process, so a second copy would repeat all the work.
- **Energy costs one portal request per meter.** That's comfortable for hundreds or a few
  thousand meters. At 100,000 meters, one full pass would take about 14 hours. Only the
  portal can fix that, with a bulk readings endpoint or a "changed since" filter.
- **Data can be up to an hour old** with the default settings. The intervals are
  configurable, but the portal gives no way to learn about a change sooner than the next fetch.
