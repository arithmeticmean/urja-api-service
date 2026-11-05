# How the Urja Meter Ops portal works

## Short version

- The pages themselves don't contain data. After a page loads, it asks the server for
  **JSON** from URLs under `/portal/`. My service calls those same URLs.
- You **log in once** and get a **cookie**. Every request needs that cookie. After about an
  hour it stops working and the portal answers **401**, so you have to log in again.
- There is a **hidden "Export all meters" button** that returns **all 403 meters in one request**,
  with more details than the normal meter list. It needs a **signature** on each request.
- Energy readings come **one meter at a time**.
- The portal allows about **120 requests a minute**. After that it answers **429 (too many requests)**.

## How I found things

1. **Clicked through every page** of the portal (Meters, a meter's detail page, Transformers,
   next/previous page, search) with Firefox's **Network tab** open, and noted every request it made.
2. **Searched the portal's JavaScript** in Firefox's **Debugger tab** (`Ctrl+Shift+F`, with regex
   on, e.g. `/portal/`, `fetch(`). This is how I found the export button's code and how its
   signature is made.
3. **Repeated the requests with `curl`** to see how each URL behaves (bad page numbers,
   pages past the end, many requests in a row).
4. **Hit a 401** after leaving it for a while: the login had expired. So the service has to
   log in by itself and log in again when that happens.
5. **Hit 429s** when my service fetched energy for all meters too fast. A small `curl` script
   (below) measured the limit.

## Logging in

```
POST /login
Content-Type: application/x-www-form-urlencoded
Origin: https://urja-ops.flockenergy.tech
Accept: application/json
x-sveltekit-action: true

email=...&password=...
```

- The answer is **always HTTP 200**, even for a wrong password. You have to read the body:
  - logged in: `{"type":"redirect","status":303,"location":"/meters"}`
  - wrong password: `{"type":"failure","status":401,...,"Invalid email or password."}`
- On success the portal sets the cookie `__Secure-better-auth.session_token`.
- When the cookie expires, requests return **401** with
  `{"error":"unauthorized","message":"A valid session is required."}`. Log in again and retry.

## The URLs

| URL | What it returns |
|---|---|
| `GET /portal/meters/search?q=&page=N` | 20 meters per page, plus `total` (403 → 21 pages) |
| `GET /portal/dts?page=N` | Transformers, 20 per page |
| `GET /portal/meters/{id}/energy` | All readings for one meter |
| `GET /portal/meters/{id}/geo` | One meter's location |
| `GET /portal/keys` | The secret used to sign export requests |
| `GET /portal/export?page=1` | **All meters**, full details (needs a signature) |

### Meter list example

```json
{
  "data": [{"meterId":"J100000","serialNo":"SE33962","make":"HPL","phaseType":"single",
            "installStatus":"Decommissioned","dtCode":"DT-001"}],
  "total": 403, "page": 1, "pageSize": 20
}
```

### Energy example

```json
{"data":[
  {"timestamp":"23/06/2026 23:30","kwh":"48438.74","kvah":"52313.84","voltR":"226"},
  {"timestamp":"24/06/2026 00:00","kwh":"48439.16","kvah":"52314.29","voltR":"227"}
]}
```

- One reading every **30 minutes**, oldest first.
- Times look like `23/06/2026 23:30` with **no timezone** (I treat them as Indian time).
- Numbers come as **text**.
- `kwh` / `kvah` are **meter totals that keep growing**, not usage. Usage for a half hour =
  this reading minus the previous one.
- Readings stop on **30 June 2026**, so the data is old.

## The export (the bulk path)

Found by searching the JavaScript: the **Transformers page** has an "Export all meters" button.

Steps:

1. `GET /portal/keys` → `{"data":{"signingSecret":"..."}}`
2. Build this text (lines joined with a newline, current time in seconds):
   ```
   GET
   /portal/export
   page=1
   1762310000
   ```
3. Sign it with **HMAC-SHA256** using the secret, written as hex.
4. Call the export with two extra headers:
   ```
   GET /portal/export?page=1
   x-timestamp: 1762310000
   x-signature: <signature>
   ```

With `curl`:

```bash
B=https://urja-ops.flockenergy.tech
COOKIE="Cookie: __Secure-better-auth.session_token=$(tr -d '[:space:]' < ./token)"

SECRET=$(curl -s "$B/portal/keys" -H "$COOKIE" | jq -r .data.signingSecret)
TS=$(date +%s)
SIG=$(printf 'GET\n/portal/export\npage=1\n%s' "$TS" | openssl dgst -sha256 -hmac "$SECRET" -hex | awk '{print $NF}')

curl -s "$B/portal/export?page=1" -H "$COOKIE" -H "x-timestamp: $TS" -H "x-signature: $SIG"
```

What it gives you:

- **All 403 meters in one request** (the meter list needs 21).
- **More fields**: `installType`, `build`, the full network position (zone → circle → division
  → subdivision → substation → feeder → transformer) and the location (`lat`, `lng`).
- The button only ever asks for `page=1`. That's fine today, but once there are more meters
  than one page holds, it would quietly export only part of them. My service keeps asking
  for the next page until one comes back empty.

## Rate limit

When my service fetched energy for every meter as fast as it could, the first ~128 requests
worked, then the portal answered **429**. About a minute later another ~120 worked, then 429
again. So the limit is roughly **120 requests a minute**.

The service now sends at most **1.5 requests a second** (setting `PORTAL_RPS`), and if it still
gets a 429 it waits before trying again.

Script to measure it:

```bash
API="https://urja-ops.flockenergy.tech/portal/meters/search"
COOKIE="Cookie: __Secure-better-auth.session_token=$(tr -d '[:space:]' < ./token)"

# 1. Send requests until the first 429, then show its headers and body
start=$(date +%s.%N)
for i in $(seq 1 300); do
  code=$(curl -s "$API?q=&page=1" -H "$COOKIE" -D hdr.txt -o body.txt -w '%{http_code}')
  printf "req=%-3s t=%6.2fs HTTP %s\n" "$i" "$(echo "$(date +%s.%N) - $start" | bc)" "$code"
  if [ "$code" = "429" ]; then cat hdr.txt; head -c 300 body.txt; echo; break; fi
done

# 2. Check every 5 seconds until it works again
start=$(date +%s)
while true; do
  code=$(curl -s "$API?q=&page=1" -H "$COOKIE" -o /dev/null -w '%{http_code}')
  echo "after $(( $(date +%s) - start ))s: HTTP $code"
  [ "$code" = "200" ] && break
  sleep 5
done
```

## Things that surprised me

- **Wrong page numbers don't fail.** `page=0`, `page=-1` or `page=abc` quietly return page 1.
  Pages past the end return an empty list. Both with HTTP 200.
- **Page size is fixed at 20.** Adding `pageSize=500` (or `limit`, `size`) changes nothing.
- **The number of meters changed**: 16 pages when I first looked, 21 later.
- **Blank values** come as empty text (`""`), not as missing. 22 meters have a blank code or
  name somewhere in their network position.
- **Same transformer, two names**: meters J100400–J100402 call DT-007 "Old Malviya Nagar Xfmr",
  the other 10 meters on DT-007 call it "Sanganer DT 7".
- **The network position isn't a clean tree** above the feeder level: the same division code
  shows up under different circles, for example. So I store each meter's position as given,
  instead of building one tree.
- **Nothing says what changed or when** (no "last updated" field or header), so the service
  re-reads the data on a schedule.

## Not checked
- How old a signature's timestamp can be before it's rejected.
- Whether the signing secret changes over time.
