# go-uptime-checker

A fast, concurrent **website availability checker** written in Go. Feed it a
list of URLs and it will probe them all in parallel, honor per-request timeouts
and a global rate limit, and export a report as **text**, **JSON**, or **CSV**.

This is a small but complete project that demonstrates idiomatic use of:

- **goroutines** for concurrent work
- **channels** for a bounded worker pool (semaphore)
- **`context`** for cancellation and per-request deadlines
- **`net/http`** clients with redirect and User-Agent handling
- **graceful shutdown** on Ctrl-C / SIGTERM

---

## ✨ Features

| Feature | Details |
|---|---|
| Concurrent checks | Bounded worker pool via a semaphore channel |
| Per-request timeout | `-timeout 5s` (uses `context.WithTimeout`) |
| Global rate limit | `-rate 50` (requests/second, token-bucket via `time.Ticker`) |
| Graceful cancellation | Ctrl-C cancels in-flight requests; remaining URLs marked down |
| Stable ordering | Results returned in the same order as the input |
| Multiple output formats | `text`, `json`, `csv` |
| Flexible input | Positional args, a file, or `-` for stdin |
| CI-friendly exit code | Exits `1` if any URL is down, `0` if all up |

---

## 📦 Install

```bash
git clone https://github.com/mefaisalraheem/go-uptime-checker.git
cd go-uptime-checker
go build -o bin/uptime-checker .
```

Or run directly:

```bash
go run . https://example.com https://golang.org
```

---

## 🚀 Usage

```bash
uptime-checker [flags] URL [URL...]
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `-file` | *(none)* | Read URLs from a file. Use `-` for stdin. Blank lines and `#` comments are ignored. |
| `-format` | `text` | Output format: `text`, `json`, or `csv`. |
| `-concurrency` | `20` | Maximum simultaneous in-flight requests. |
| `-timeout` | `5s` | Per-request timeout. |
| `-rate` | `0` | Max requests started per second. `0` = unlimited. |

### Examples

**Check a few URLs:**

```bash
uptime-checker https://example.com https://golang.org
```

**Check a file of URLs:**

```bash
uptime-checker -file urls.txt -concurrency 50 -timeout 3s
```

**Rate-limited JSON report:**

```bash
uptime-checker -file urls.txt -rate 20 -format json > report.json
```

**CSV export for spreadsheets:**

```bash
uptime-checker -file urls.txt -format csv > report.csv
```

**From stdin:**

```bash
cat urls.txt | uptime-checker -file -
```

### Sample output (text)

```
Uptime Report — 2024-05-01T12:34:56Z
Total: 3  Up: 2  Down: 1  Cumulative: 245ms

[UP  ] 200  https://example.com                         120ms
[UP  ] 200  https://httpbin.org/status/200              85ms
[DOWN] 404  https://httpbin.org/status/404              40ms  (unexpected status: 404)
```

### Exit codes

| Code | Meaning |
|---|---|
| `0` | All URLs up |
| `1` | At least one URL down, or an I/O error |
| `2` | Invalid usage |

---

## 🧪 Testing

```bash
make test
# or
go test ./... -race -count=1
```

The test suite covers:

- ✅ Successful 200 responses
- ✅ Non-2xx/3xx treated as down
- ✅ Per-request timeouts
- ✅ Concurrency limit is respected
- ✅ Context cancellation aborts in-flight work
- ✅ Order preservation with mixed latencies
- ✅ Empty input returns no results
- ✅ All output formats (text, JSON, CSV)

Tests use `httptest.NewServer` — no network access required.

---

## 🏗️ Architecture

```
main.go
  └── reads URLs (args / file / stdin)
  └── creates signal.NotifyContext (Ctrl-C)
  └── calls checker.Check(ctx, urls)

checker/
  └── Check()
        • rate limiter (time.Ticker) — before semaphore to avoid holding slots
        • semaphore chan struct{}    — bounded concurrency
        • goroutine per URL          — writes into results[i] (no lock needed)
        • context checks            — early return on cancellation

report/
  └── Build()       — computes totals
  └── WriteText()   — human readable
  └── WriteJSON()   — machine readable
  └── WriteCSV()    — spreadsheet friendly
```

### Why a semaphore channel?

A `chan struct{}` of size `N` gives us a bounded worker pool without the
complexity of a fixed worker fleet — each URL spawns a goroutine that first
**acquires** a slot, does its work, then **releases** the slot. This is
idiomatic Go for "at most N of these at once".

### Why no mutex around `results`?

Each goroutine writes to a **distinct** index of the pre-sized `results`
slice, so writes never race. `wg.Wait()` guarantees all writes complete
before the slice is returned — safe publication.

---

## 📄 License

MIT © Malik Faisal Raheem