# VC Sentry — MVP Plan

A single-binary Discord “voice channel sentry” bot built with Go, **SQLite** (no external infra), **Prometheus** metrics, and an in‑memory worker pool. Tracks one target voice channel, notifies subscribers via DM on **joins** and **game changes**, and summarizes when **multiple games** are being played. This doc is your build target.

---

## 1) Goals & Non‑Goals
**Goals (v1):**
- Watch **one** configured voice channel in a specific guild.
- Users can **subscribe** to DMs (via command) to get:
  - DM when someone **joins** the voice channel.
  - DM when an in‑channel user **starts/stops/changes** a game.
  - A compact **summary DM** when there are **mixed games** (more than one distinct game in the channel).
- **Auto-detect games** from Discord presence events (no whitelist config in v1).
- Provide **/status** to view current members + their games.
- Observability: `/metrics` (Prometheus), `/healthz`, `/readyz`, `pprof`.
- Single process, durable state via **SQLite** with WAL.

**Non‑Goals (v1):** multi-guild, multi-VC management, admin UI, persistence of historical timelines, HA/replication, external queues, or Redis.

---

## 2) External Contract (what’s exposed)
- **Binary:** `vc-sentry`
- **Env vars:**
  - `DISCORD_BOT_TOKEN` (required)
  - `GUILD_ID` (required)
  - `VOICE_CHANNEL_ID` (required)
  - `HTTP_ADDR` (default `:8080`)
  - `SQLITE_DSN` (default `file:vc-sentry.db?_busy_timeout=3000&_journal_mode=WAL`)
- **HTTP endpoints:** `/healthz`, `/readyz`, `/metrics`, `/debug/pprof/*`
- **Discord commands / UX:**
  - `/watch set #channel` (owner only in v1)  
  - `/watch list` (echo configured channel)
  - `/subscribe` (add caller to subscriber set)
  - `/unsubscribe`
  - `/status` (who’s in VC + which game per user)

---

## 3) Architecture Overview
**Single process** with five blocks:
1. **Discord Adapter** — connects gateway, receives `VOICE_STATE_UPDATE`, `PRESENCE_UPDATE`, `MESSAGE_CREATE`; translates to app intents.
2. **Core Engine** — state transitions → decides notifications; computes `stateHash` for dedupe; applies rate limits.
3. **Store (SQLite)** — settings, subscribers, live presence, last DM dedupe, optional DM outbox.
4. **DM Worker Pool** — `jobs chan` (bounded), N workers; per-send timeout + retry with jitter; graceful drain on shutdown.
5. **HTTP/Observability** — health, readiness, Prometheus, pprof.

**Cancellation & backpressure:** root `context.WithCancel` bound to SIGTERM; bounded `jobs` channel; per-worker timeouts; queue depth metric.

---

## 4) Data Model (SQLite)
```sql
PRAGMA journal_mode=WAL;       -- set at startup
PRAGMA synchronous=NORMAL;
PRAGMA busy_timeout=3000;      -- ms

CREATE TABLE IF NOT EXISTS settings (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  guild_id TEXT NOT NULL,
  voice_channel_id TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS subscribers (
  user_id TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS vc_presence (
  user_id TEXT PRIMARY KEY,
  game TEXT,                   -- NULL => not playing
  updated_at INTEGER NOT NULL  -- unix seconds
);

CREATE TABLE IF NOT EXISTS last_dm (
  user_id TEXT PRIMARY KEY,
  state_hash TEXT NOT NULL,
  sent_at INTEGER NOT NULL     -- unix seconds
);

-- Optional crash-safe queue (can be v1.1)
CREATE TABLE IF NOT EXISTS dm_outbox (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id TEXT NOT NULL,
  reason TEXT NOT NULL,        -- join|game|mixed
  body TEXT NOT NULL,
  available_at INTEGER NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  picked_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_dm_outbox_available ON dm_outbox(available_at);
```

---

## 5) Event → Behavior Rules
**On Voice Join/Leave (VOICE_STATE_UPDATE):**
- Maintain `vc_presence` membership (insert/remove user row; keep last known `game`).
- On **join**: enqueue DM to all subscribers: “**@X** joined **#voice**”.

**On Presence Change (PRESENCE_UPDATE):**
- If user is **in the watched VC**, and their primary activity/game changes:
  - Enqueue DM: “**@X** now playing **Game** in **#voice**”.
- After any change, compute snapshot:
  - `members := SELECT user_id, game FROM vc_presence ORDER BY user_id`
  - `games := DISTINCT non-null games`
  - If `len(games) > 1`, enqueue **mixed** summary DM (one message) listing `user → game`.

**Dedupe & Cooldown:**
- For each subscriber, compute `stateHash := sha256(JSON(members))`.
- Only DM if `stateHash` differs from `last_dm.state_hash` **or** last sent > 60 seconds.
- Update `last_dm` after successful DM.

---

## 6) Concurrency Plan
- `jobs := make(chan DMJob, 256)`; Gauge exposed as `vc_sentry_queue_depth`.
- Start 4 workers:
  - Each `send` has `context.WithTimeout(3s)`.
  - Retry x3 with exponential jitter (150ms → 300ms → 600ms).
- On shutdown (SIGTERM): stop accepting new jobs, allow 5s drain; cancel workers.

---

## 7) Metrics (Prometheus)
- `vc_sentry_events_total{type="voice|presence|message"}`
- `vc_sentry_dm_sent_total{reason="join|game|mixed"}`
- `vc_sentry_dm_errors_total{reason}`
- `vc_sentry_queue_depth` (gauge)
- (Optional) `vc_sentry_snapshot_duration_seconds` (histogram)

Dash ideas: active connections, queue depth, DM success rate, event rate.

---

## 8) Commands & Permissions (MVP)
- `/subscribe` — add caller to `subscribers`.
- `/unsubscribe` — remove caller.
- `/status` — Pretty print current VC members + game per user.
- `/watch set #channel` — **owner-only**; write `settings` (guild/channel).
- `/watch list` — echo config.

**Note:** For v1 you can parse plain text in a designated text channel if slash command registration is too heavy on day one.

---

## 9) Repo Structure (proposed)
```
/cmd/vc-sentry/main.go           # wire env, store, metrics, bot; start HTTP + gateway
/internal/adapter/discord/       # session init + event handlers (no business logic)
/internal/core/engine.go         # decides what to DM; state hashing; rate limiting
/internal/store/sqlite/          # concrete SQLite impl (CRUD)
/internal/metrics/metrics.go     # prometheus registry + counters/gauges
/internal/http/server.go         # /healthz /readyz /metrics /pprof
```

---

## 10) Day‑by‑Day Build Path (7 short sessions)
**Day 1 – Bootstrap**
- `main.go` with env, pprof, metrics HTTP server.
- Open SQLite with WAL; run migrations.
- Discord session open with intents; print ready.

**Day 2 – Store CRUDs**
- `UpsertPresence(user, game)`; `RemovePresence(user)`.
- `ListPresence()`; `GetSubscribers()`; `Add/RemoveSubscriber()`.
- `Get/SetLastDM(user, hash, t)`.

**Day 3 – Voice events**
- Handle `VOICE_STATE_UPDATE`: maintain membership; enqueue **join** DM.
- Add worker pool & `sendWithRetry` stub; send a test DM to yourself.

**Day 4 – Presence events**
- Extract primary game from activities; update presence.
- If in VC and game changed → enqueue **game** DM.

**Day 5 – Mixed summary**
- Compute snapshot + `stateHash`; dedupe per subscriber; send **mixed** DM only when hash changes or cooldown elapsed.

**Day 6 – Commands**
- `/subscribe`, `/unsubscribe`, `/status`, `/watch set`, `/watch list`.
- Permissions: restrict `/watch set` to owner ID.

**Day 7 – Polish & Observability**
- Queue depth gauge; event counters; success/error counters.
- Readiness gate (Discord ready + DB migrated).
- Race detector, `pprof` check; tidy `go.mod`.

---

## 11) Acceptance Criteria (Definition of Done)
- Running `vc-sentry` with the three required env vars lets the bot connect and:
  - DM on **join** to all subscribers.
  - DM on **game changes** (start/stop/switch) for users inside the watched VC.
  - Send **one** mixed‑games summary when state changes, not spam.
- `/status` returns consistent member+game view.
- `/metrics` exports counters/gauge; `/healthz` and `/readyz` return 200.
- Graceful shutdown drains job queue (≤5s) without panics.

---

## 12) Nice‑to‑Haves (v1.1)
- Replace in‑mem jobs with SQLite **dm_outbox** for crash‑safe retries.
- Admin rate limits (per user token bucket) to prevent spam.
- Pretty embeds for DMs; map app IDs to friendly game names.
- `/history` (persist timeline) and `/report` (last 24h activity).

---

## 13) Future v2 Ideas (voice activation)
- Join VC audio, stream PCM frames; pipeline: **capture → VAD → STT → intents**.
- Pluggable STT backends (Whisper/Deepgram/Vosk); backpressure across goroutines.
- Keyword detection (wake words) and action triggers.

---

## 14) Risks & Mitigations
- **Discord rate limits** → worker backoff + jitter, bounded queue, dedupe.
- **Gateway disconnects** → discordgo auto‑reconnect; idempotent handlers.
- **Spam storms** (rapid presence flaps) → 60s cooldown + stateHash dedupe per subscriber.
- **SQLite locks** → short transactions, WAL, busy_timeout; avoid long writes in handlers.

---

## 15) Testing Notes
- Unit test core engine with an in‑memory fake store.
- Integration: simulate events (voice join, presence changes) and assert queued DMs.
- Benchmark fan‑out strategies if needed; keep pprof snapshots.

