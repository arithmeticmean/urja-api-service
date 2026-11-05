# Reflection

## What assumptions did you make?

That meters are never deleted from the portal: their lifecycle ends with the status
`Decommissioned`, so the service never removes a meter. That energy readings are only ever
added and never edited, which is what lets the service skip a meter whose response is the
same size as last time. That the portal's timestamps, which carry no timezone, are Indian
time, since the utility is in Jaipur. That the brief's "current data" means I don't need a
history of meter changes. That data up to an hour old is fine for product and data teams.
And that only one copy of the service runs at a time, so the scheduler can keep track of
in-flight jobs in memory.

## Which part was the most difficult, and how did you get unstuck?

Deciding how to keep the data in sync with a portal that gives nothing to sync with: 20 rows
per page, no "updated since" filter, no change dates, and a session that expires every hour.
I spent a long time designing around the paged meter list (checking for rows that shift
between pages, re-reading only the last page, rotating through pages, hashing pages) and
every idea still needed to download each page to know if it had changed. What got me unstuck
was going back to investigating instead of designing: clicking every part of the UI with the
Network tab open, and searching the JavaScript in the Debugger. That's how I found the
"Export all meters" button hidden on the Transformers page and its request signing. It
returns every meter in one request and made most of the sync problem disappear.

## If you had another day, what would you improve?

Tests first. Only the portal client is tested, and most of those tests need real credentials.
I'd add a fake portal so the workers can be tested end to end (including session expiry and
429s), and repository tests against a real Postgres. Then an aggregated consumption endpoint
(usage per day per feeder or zone) and a hierarchy endpoint, which is what a data team would
actually ask for. I'd also make `/healthz` report how old the data is, fix the start-up
order so the first energy check waits for the first meter sync, and generate `openapi.json`
from the code so it can't drift.

## What mistake did you make while solving this?

I removed the rate limiter from the portal client to keep it simple, reasoning that retries
would cover any 429s. The first real run proved otherwise: about 128 energy requests
succeeded, then everything failed with 429 for the rest of the minute, and the retries (a
few seconds apart) were far too short to wait it out. Reading those logs is also how I worked
out the limit (roughly 120 requests a minute). The rate limiter went back in, at 1.5 requests a
second. Smaller ones along the way: I first trusted the "16 pages" I'd seen in the UI (it was
21 by the time I looked properly), and I ran the binary directly without loading `.env`, so it
started with no database URL and an empty password.

## If you were reviewing your own submission, what would you criticise?

Test coverage is thin for the parts that matter most: the scheduler and the SQL. The design
went through several rewrites (a Postgres job table, an in-memory timer queue, separate loops)
before settling, which cost time the brief explicitly asks you to spend on writing things up.
The in-memory record of in-flight jobs means two copies of the service would duplicate all
the work. `openapi.json` is hand-written and can fall out of step with the code. And I leaned
heavily on an AI assistant: it wrote a large share of the code and helped draft these
documents. The investigation of the portal was mine (it can't see the portal), and I made the
design calls, but a reviewer should know how the work was split.
