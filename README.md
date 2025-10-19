# Click Counter — HTTP service with per-minute click statistics

## 1. What the service does

A simple service that counts banner clicks and returns aggregated per-minute statistics.

**Main endpoints:**
1. `GET /counter/{bannerID}` — registers a click, returns `204 No Content`.  
2. `POST /stats/{bannerID}` — returns JSON statistics for the `[from, to)` range (UTC).

---

## 2. Environment variables

| Variable | Default | Description |
|-----------|----------|-------------|
| `LISTEN_ADDR` | `:3000` | HTTP server address |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `DATABASE_URL` | `postgres://postgres:postgres@db:5432/clicks?sslmode=disable` | PostgreSQL connection |
| `FLUSH_EVERY` | `1s` | Interval to flush data to DB |
| `SHARDS` | `64` | Number of in-memory shards |
| `READ_MAX_RANGE_DAYS` | `90` | Max range for `/stats` |
| `SHUTDOWN_WAIT` | `5s` | Graceful shutdown timeout |
| `MAX_CPU` | `0` | GOMAXPROCS (0 = auto) |

---

## 3. Run with Docker Compose

```bash
docker compose -f dev/docker-compose.yml up -d --build
````

Check container status:

```bash
docker ps | grep clicks-
```

Health check:

```bash
curl -i http://localhost:3000/healthz
# → HTTP/1.1 204 No Content
```

Stop services:

```bash
docker compose -f dev/docker-compose.yml down -v
```

---

## 4. Local run (alternative)

```bash
docker compose -f dev/docker-compose.yml up -d db

LISTEN_ADDR=:3000 LOG_LEVEL=debug \
DATABASE_URL="postgres://postgres:postgres@localhost:5432/clicks?sslmode=disable" \
go run ./cmd/clicks-api
```

---

## 5. API test

**1. Clicks**

```bash
curl -i http://localhost:3000/counter/1
curl -i http://localhost:3000/counter/1
```

**2. Get stats**

```bash
FROM=$(date -u -v-1M +"%Y-%m-%dT%H:%M:00Z")
TO=$(date -u +"%Y-%m-%dT%H:%M:00Z")
echo "$FROM -> $TO"

curl -s -X POST http://localhost:3000/stats/1 \
  -H 'Content-Type: application/json' \
  -d "{\"from\":\"$FROM\",\"to\":\"$TO\"}" | jq
```

Expected output:

```json
{
  "stats": [
    {"ts":"2025-10-19T00:29:00Z","v":2}
  ]
}
```

---

## 6. Load testing (optional)

Install [hey](https://github.com/rakyll/hey):

```bash
brew install hey
```

Run:

```bash
# about 1000 rps
hey -z 10s -c 20 -q 50 http://localhost:3000/counter/1
```

Check data in stats:

```bash
FROM=$(date -u -v-1M +"%Y-%m-%dT%H:%M:00Z")
TO=$(date -u +"%Y-%m-%dT%H:%M:00Z")
curl -s -X POST http://localhost:3000/stats/1 \
  -H 'Content-Type: application/json' \
  -d "{\"from\":\"$FROM\",\"to\":\"$TO\"}" | jq
```

---

## 7. Makefile commands

```bash
make dev-up      # build and start (db + app)
make dev-down    # stop and remove containers
make build       # build binary locally
make tidy        # tidy dependencies
```

---

## 8. Project structure

```
cmd/clicks-api/main.go         # entry point
internal/app/...               # app lifecycle
internal/adapter/transport/http# HTTP server (chi)
internal/adapter/store/postgres# PostgreSQL store
internal/service/...           # click aggregator
internal/entity/...            # DTO models
pkg/config, pkg/logger         # config and zap logger
dev/docker-compose.yml, .env   # dev environment
migrations/001_init.sql        # DB schema
```

---

## 9. Common issues

| Error                                        | Solution                                                    |
| -------------------------------------------- | ----------------------------------------------------------- |
| `/app/clicks-api: no such file or directory` | Remove `- ..:/app` from `dev/docker-compose.yml`.           |
| `port already in use`                        | Change ports in `dev/docker-compose.yml`.                   |
| `/stats` returns `null`                      | No data in range → widen the window or try previous minute. |
| `/stats` empty right after click             | Wait 1s (`FLUSH_EVERY=1s`).                                 |

---

## 10. Quick test checklist

1. Start services

   ```bash
   docker compose -f dev/docker-compose.yml up -d --build
   ```
2. Check `/healthz`

   ```bash
   curl -i http://localhost:3000/healthz
   ```
3. Click a few times

   ```bash
   curl -i http://localhost:3000/counter/1
   ```
4. Get stats

   ```bash
   FROM=$(date -u -v-1M +"%Y-%m-%dT%H:%M:00Z")
   TO=$(date -u +"%Y-%m-%dT%H:%M:00Z")
   curl -s -X POST http://localhost:3000/stats/1 \
     -H 'Content-Type: application/json' \
     -d "{\"from\":\"$FROM\",\"to\":\"$TO\"}" | jq
   ```
5. Verify response contains `v > 0`.

---

## If all steps above work — the service is fully functional and ready for demo.
