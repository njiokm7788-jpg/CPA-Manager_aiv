# CLI Proxy API Management Center

[中文文档](README_CN.md)

A single-file Web UI for **CLI Proxy API (CPA)** plus an optional **Usage Service** for persistent usage analytics.

Since v6.10.0, CPA no longer includes built-in usage statistics. This project now supports usage analytics through a long-running Usage Service that consumes the CPA usage queue, persists request events to SQLite by default or PostgreSQL when configured, and exposes panel-compatible usage APIs.

- **CPA Main project**: https://github.com/router-for-me/CLIProxyAPI
- **Recommended CPA version**: >= v6.10.8

## Panel Preview

![Request monitoring dashboard showing filters, usage KPIs, account summaries, and export/import actions](img/screenshot-20260507-194947.png)
![Account usage detail with Codex quota bars and model-level cost breakdown](img/screenshot-20260507-195041.png)
![Realtime monitoring table showing request status, latency, token usage, and cost](img/screenshot-20260507-195154.png)
![Codex account inspection progress with live probe logs and cleanup recommendations](img/screenshot-20260507-194738.png)

## What This Provides

- A single-file React management panel for CPA Management API (`/v0/management`)
- A Dockerized Usage Service for SQLite-backed or PostgreSQL-backed usage persistence
- Two deployment modes:
  - **Full Docker mode**: open the built-in panel from Usage Service and only enter the CPA URL + Management Key
  - **CPA panel mode**: keep using CPA's `/management.html`, then configure a separately deployed Usage Service inside the panel
- Runtime monitoring, account/model/channel breakdowns, model pricing, estimated token cost, imports/exports, auth-file operations, quota views, logs, config editing, and system utilities

## Choose a Deployment Mode

| Mode | Entry URL | What the user configures | Best for |
|---|---|---|---|
| Full Docker mode | `http://<host>:18317/management.html` | CPA URL + Management Key on login | New deployments, one entry point, least browser/CORS complexity |
| CPA panel mode | `http://<cpa-host>:8317/management.html` | Usage Service URL under **Management Center Info -> External Usage Service** | Existing CPA automatic panel loading |
| Frontend only | Vite dev server or `dist/index.html` | CPA URL, optionally Usage Service URL | Development |

Full Docker mode does not bundle CPA itself. CPA still runs as the upstream service; the Docker image provides the Usage Service plus an embedded copy of this management panel.

## CPA Prerequisites

Request statistics require the CPA usage queue:

- CPA Management must be enabled because the usage queue uses the same availability and Management Key as `/v0/management`.
- Enable usage publishing in CPA with `usage-statistics-enabled: true`, or through `PUT /usage-statistics-enabled` with `{ "value": true }`.
- CPA `v6.10.8+` is preferred because it exposes the HTTP usage queue endpoint `/v0/management/usage-queue`, which can pass through regular HTTP reverse proxies.
- Older CPA versions use the RESP queue protocol. Usage Service falls back to RESP in `auto` mode when the HTTP queue endpoint is unavailable. RESP listens on the CPA API port, usually `8317`, and cannot pass through a regular HTTP reverse proxy.
- CPA keeps queue items in memory for `redis-usage-queue-retention-seconds`, default `60` seconds and maximum `3600` seconds. Keep Usage Service running continuously.
- Exactly one Usage Service should consume the same CPA usage queue.

## Architecture

### Full Docker Mode

```text
Browser
  -> Usage Service :18317
      -> built-in management.html
      -> /v0/management/usage and /v0/management/model-prices from Usage Service DB
      -> other /v0/management/* proxied to CPA
      -> HTTP/RESP consumer -> CPA API port
      -> SQLite /data/usage.sqlite or configured PostgreSQL
```

The login page detects that it is hosted by Usage Service. You enter the CPA URL and Management Key. Usage Service validates the CPA Management API, stores the setup in the configured database, starts the collector with the configured mode (`auto` by default: HTTP queue first, RESP fallback), and serves the panel from the same origin.

### CPA Panel Mode

```text
Browser
  -> CPA /management.html
      -> normal CPA Management API calls stay on CPA
      -> usage calls go to configured Usage Service URL

Usage Service
  -> HTTP/RESP consumer -> CPA API port
  -> SQLite /data/usage.sqlite or configured PostgreSQL
```

Use this when CPA still auto-downloads and serves the panel. Deploy Usage Service separately, then open **Management Center Info -> External Usage Service**, enable it, enter the Usage Service URL, and save.

## Quick Start: Full Docker Mode

### Docker Hub Image

```bash
docker run -d \
  --name cpa-manager \
  --restart unless-stopped \
  -p 18317:18317 \
  -v cpa-manager-data:/data \
  seakee/cpa-manager:latest
```

Open:

```text
http://<host>:18317/management.html
```

Enter:

- CPA URL:
  - Docker Desktop host CPA: `http://host.docker.internal:8317`
  - Same compose network: `http://cli-proxy-api:8317`
  - Remote CPA: `https://your-cpa.example.com`
- Management Key

The published image supports `linux/amd64` and `linux/arm64`. If your image is published under another Docker Hub namespace, replace `seakee/cpa-manager:latest`.

### Docker Compose

```yaml
services:
  cpa-manager:
    image: seakee/cpa-manager:latest
    restart: unless-stopped
    ports:
      - "18317:18317"
    volumes:
      - cpa-manager-data:/data

volumes:
  cpa-manager-data:
```

Start:

```bash
docker compose up -d
```

### Linux Host CPA

If CPA runs directly on a Linux host and Usage Service runs in Docker, add a host gateway:

```bash
docker run -d \
  --name cpa-manager \
  --restart unless-stopped \
  --add-host=host.docker.internal:host-gateway \
  -p 18317:18317 \
  -v cpa-manager-data:/data \
  seakee/cpa-manager:latest
```

Then enter `http://host.docker.internal:8317` as the CPA URL.

## Quick Start: CPA Panel Mode

1. Start CPA as usual and open:

   ```text
   http://<cpa-host>:8317/management.html
   ```

2. Deploy Usage Service:

   ```bash
   docker run -d \
     --name cpa-manager \
     --restart unless-stopped \
     -p 18317:18317 \
     -v cpa-manager-data:/data \
     seakee/cpa-manager:latest
   ```

3. In the CPA panel, go to:

   ```text
   Management Center Info -> External Usage Service
   ```

4. Enable it and enter:

   ```text
   http://<usage-service-host>:18317
   ```

5. Click **Save and connect**.

The panel sends the current CPA URL and Management Key to Usage Service. After that, monitoring reads usage data from Usage Service while other management calls continue to use CPA.

## Build Locally

```bash
docker compose -f docker-compose.usage.yml up --build
```

This builds the React panel and embeds it into the Go Usage Service binary.

## Usage Service Configuration

Most users can configure CPA URL and Management Key from the panel. Environment variables are useful for automated deployments.

| Variable | Default | Description |
|---|---:|---|
| `HTTP_ADDR` | `0.0.0.0:18317` | Usage Service HTTP listen address |
| `USAGE_DB_DRIVER` | `sqlite` | Storage driver: `sqlite` or `postgres` |
| `USAGE_DB_PATH` | `/data/usage.sqlite` | SQLite database path |
| `USAGE_DATA_DIR` | `/data` | Base data directory when `USAGE_DB_PATH` is not overridden |
| `USAGE_DATABASE_URL` | empty | PostgreSQL DSN when `USAGE_DB_DRIVER=postgres`; Aiven URLs usually include `sslmode=require` |
| `USAGE_DB_MAX_OPEN_CONNS` | `10` | Maximum open PostgreSQL connections |
| `USAGE_DB_MAX_IDLE_CONNS` | `5` | Maximum idle PostgreSQL connections |
| `USAGE_DB_CONN_MAX_LIFETIME_MINUTES` | `30` | PostgreSQL connection lifetime in minutes |
| `CPA_UPSTREAM_URL` | empty | Optional CPA base URL for unattended startup |
| `CPA_MANAGEMENT_KEY` | empty | Optional CPA Management Key for unattended startup |
| `CPA_MANAGEMENT_KEY_FILE` | `/run/secrets/cpa_management_key` | Optional file containing the Management Key |
| `USAGE_COLLECTOR_MODE` | `auto` | Collection mode: `auto` prefers the HTTP usage queue and falls back to RESP for older CPA; `http` forces HTTP; `resp` forces RESP |
| `USAGE_RESP_QUEUE` | `usage` | RESP key argument; CPA currently ignores it, leave the default unless upstream changes |
| `USAGE_RESP_POP_SIDE` | `right` | `right` uses `RPOP`; `left` uses `LPOP` |
| `USAGE_BATCH_SIZE` | `100` | Maximum queue records per pop |
| `USAGE_POLL_INTERVAL_MS` | `500` | Idle polling interval |
| `USAGE_QUERY_LIMIT` | `50000` | Maximum recent events returned through compatible `/usage` |
| `USAGE_CORS_ORIGINS` | `*` | Allowed browser origins for CPA panel mode |
| `USAGE_RESP_TLS_SKIP_VERIFY` | `false` | Skip TLS verification for RESP connection |
| `PANEL_PATH` | empty | Serve a custom `management.html` instead of the embedded one |

If `CPA_UPSTREAM_URL` and `CPA_MANAGEMENT_KEY` are set, collection starts automatically on boot. Otherwise, use the web panel setup flow.

For Aiven for PostgreSQL, start the container with `USAGE_DB_DRIVER=postgres` and pass the Aiven service URI through `USAGE_DATABASE_URL`, for example `postgres://USER:PASSWORD@HOST:PORT/DB?sslmode=require`. Existing SQLite data is not migrated automatically.

## Data and Security Notes

- SQLite data is stored under `/data`; mount it to persistent storage.
- In full Docker mode, CPA URL and Management Key are stored in the configured Usage Service database so collection can resume after restart.
- Protect the SQLite `/data` volume or PostgreSQL credentials. They contain usage metadata and the saved Management Key.
- Usage Service redacts key-like fields before storing raw JSON payload snapshots, but request metadata may still expose models, endpoints, account labels, and token usage.
- RESP queue consumption is pop-based. Do not run multiple Usage Service consumers against the same CPA instance.
- If Usage Service is down longer than CPA's queue retention window, that period's usage cannot be recovered without CPA-side persistence.

## Runtime Endpoints

| Endpoint | Purpose |
|---|---|
| `GET /health` | Basic health check |
| `GET /status` | Collector, storage backend, event count, and error status |
| `GET /usage-service/info` | Allows the frontend to detect full Docker mode |
| `POST /setup` | Save CPA URL + Management Key and start collection |
| `GET /v0/management/usage` | Compatible usage payload for the panel |
| `GET /v0/management/usage/export` | Export usage events as JSONL |
| `POST /v0/management/usage/import` | Import JSONL usage events or legacy JSON snapshots |
| `GET /v0/management/model-prices` | Read Usage Service database-backed model pricing |
| `PUT /v0/management/model-prices` | Replace saved model pricing |
| `POST /v0/management/model-prices/sync` | Sync model prices from LiteLLM pricing metadata |
| `GET /models`, `GET /v1/models` | Proxy model-list requests to CPA after setup |
| `/v0/management/*` | Proxied to CPA except usage endpoints |

After setup, `/status`, usage, model-pricing, and `/v0/management/*` proxy endpoints require the same Management Key as a Bearer token.

Usage import accepts two file families: JSONL/NDJSON event files exported by Usage Service, and legacy JSON snapshots produced by older CPA `/usage/export`. Legacy JSON can be converted only when `usage.apis.*.models.*.details[]` request details are present. Files that contain only aggregate totals are rejected because request-level monitoring data cannot be reconstructed. Legacy import is a migration/recovery path, not a perfect continuation of newly collected Usage Service data: old files may miss metadata such as `api_key_hash`, channel, request ID, method/path, latency, cache tokens, or failure reason, so account matching, API Key level analysis, and detail accuracy may be lower. Importing legacy files affects totals, trend charts, and account/key breakdowns; use a test or backup database first when accuracy matters.

## Feature Overview

- **Dashboard**: connection state, backend version, quick health summary
- **Configuration**: visual and source editing for CPA configuration
- **AI Providers**: Gemini, Codex, Claude, Vertex, OpenAI-compatible providers, and Ampcode
- **Auth Files**: upload, download, delete, status, OAuth exclusions, model aliases
- **Quota**: quota views for supported providers
- **Request Monitoring**: persisted usage KPIs, model/channel/account breakdowns, model pricing, estimated token cost, failure analysis, realtime tables
- **Codex Account Inspection**: batch probing and cleanup suggestions for Codex auth pools
- **Logs**: incremental file log reading and filtering
- **Management Center Info**: model list, version checks, local state tools, external Usage Service configuration

## Development

Frontend:

```bash
npm install
npm run dev
npm run type-check
npm run lint
npm run build
```

Usage Service:

```bash
cd usage-service
go test ./...
go run ./cmd/cpa-manager
```

## Build and Release

- Vite builds a single-file `dist/index.html`.
- Tagging `vX.Y.Z` triggers `.github/workflows/release.yml`.
- The release workflow uploads `dist/management.html` to GitHub Releases.
- The same workflow builds `Dockerfile.usage-service` and pushes `seakee/cpa-manager`.
- The Docker image is published for `linux/amd64` and `linux/arm64`.
- The workflow syncs `README.md` to the Docker Hub overview.
- Required GitHub secrets:
  - `DOCKERHUB_USERNAME`
  - `DOCKERHUB_TOKEN`

## Troubleshooting

- **Cannot connect in full Docker mode**: verify the CPA URL from inside the Usage Service container. For host CPA on Linux, use `--add-host=host.docker.internal:host-gateway`.
- **Monitoring is empty**: enable CPA usage publishing, verify Usage Service `/status`, and confirm only one consumer is running.
- **`unsupported RESP prefix 'H'`**: upgrade CPA to `v6.10.8+` and keep the default `USAGE_COLLECTOR_MODE=auto` so Usage Service uses the HTTP usage queue first. On older CPA or forced RESP mode, the CPA URL must be a container/host direct address for port `8317`, not a regular HTTP reverse-proxy domain.
- **401 from Usage Service**: use the same Management Key that was saved during setup.
- **Docker panel shows stale data**: check `/status` for `lastConsumedAt`, `lastInsertedAt`, and `lastError`.
- **CPA panel mode has CORS errors**: set `USAGE_CORS_ORIGINS` to the CPA panel origin or keep the default `*` for private deployments.
- **Data disappears after container rebuild**: mount `/data` to a Docker volume or host directory.
- **Detailed FAQ**: see [FAQ and Troubleshooting](https://github.com/seakee/CPA-Manager/wiki/CPA-Manager-FAQ-and-Troubleshooting) or the [Chinese FAQ](https://github.com/seakee/CPA-Manager/wiki/CPA%E2%80%90Manager-%E5%B8%B8%E8%A7%81%E9%97%AE%E9%A2%98%E4%B8%8E%E8%A7%A3%E5%86%B3%E6%96%B9%E6%A1%88).

## References

- CLIProxyAPI: https://github.com/router-for-me/CLIProxyAPI
- Redis usage queue documentation: https://help.router-for.me/management/redis-usage-queue.html

## Acknowledgements

- Thanks to the upstream projects [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) and [Cli-Proxy-API-Management-Center](https://github.com/router-for-me/Cli-Proxy-API-Management-Center) for the foundation and inspiration.
- Thanks to the [Linux.do](https://linux.do/) community for project promotion and feedback.

## License

MIT
