# VC Sentry (MVP Skeleton)

Single-binary Discord bot that watches one voice channel and DMs subscribers when:
- Someone joins the voice channel
- A member starts/stops/changes the game they are playing
- Multiple different games are being played (summary DM)

## Quick Start
1. Set environment variables:
   - `DISCORD_BOT_TOKEN` – bot token
   - `GUILD_ID` – target guild
   - `VOICE_CHANNEL_ID` – watched voice channel
   - (optional) `HTTP_ADDR` – default `:8080`
2. `go mod tidy`
3. `go run ./cmd/vc-sentry`

## Endpoints
- `GET /healthz` – liveness
- `GET /readyz` – readiness
- `GET /metrics` – Prometheus metrics
- `pprof` on `:6060`

## Packages
- `internal/store` – SQLite (WAL) for settings, subscribers, presence, last_dm, dm_outbox
- `internal/bot` – Discord session, intents, event handlers, DM worker pool
- `internal/metrics` – Prometheus registry and collectors
- `internal/http` – mux for health, metrics, pprof

