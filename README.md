# Sarah Irene Discord Bot

A single-binary Discord bot written in Go.
Two main features:

1. **VC Sentry** – Watches one configured voice channel, notifies subscribers via DM when:

   * Someone joins the voice channel.
   * A member starts/stops/changes the game they are playing.
   * Multiple different games are being played → summary DM.

2. **Ask Command** – `/ask` connects to OpenAI **GPT-5** to answer user questions.

   * Keeps short-term **per-user memory** (last 10 turns, max 24h) using bbolt.

---

## Features

* Slash commands:

  * `/subscribe` → subscribe to DM alerts.
  * `/unsubscribe` → unsubscribe from alerts.
  * `/status` → show current VC members + games.
  * `/ask q:<question>` → AI answer with GPT-5, remembers context per user.
* Observability endpoints:

  * `GET /healthz`, `/readyz`, `/metrics`, `/debug/pprof/*`
* Persistent state in **bbolt** (`.db` file, no external infra).
* Prometheus metrics: event counts, DM sent/errors, job queue depth.
* Graceful shutdown on SIGINT/SIGTERM.

---

## Requirements

* Go 1.25
* A Discord bot token (from Developer Portal).
* OpenAI API key with access.

---

## Setup

1. Clone repo and install deps:

   ```bash
   go mod tidy
   ```

2. Create `.env` or `local.env`:

   ```env
   DISCORD_BOT_TOKEN=xxx
   GUILD_ID=123456789012345678
   VOICE_CHANNEL_ID=987654321098765432

   SERVER_HTTP=:8080

   BBOLT_PATH=./vc-sentry.db
   BBOLT_TIMEOUT_MS=3000

   OPENAI_API_KEY=sk-xxxx
   OPENAI_MODEL=gpt-xx-xxx
   ```

3. Run:

   ```bash
   go run ./cmd/vc-sentry
   ```

4. Invite the bot to your server with proper intents and permissions
   (Presence Intent enabled in Developer Portal).

---

## Repo Structure

```
/main.go                  # entrypoint
/application              # wire config, db, bot, httpserver
/configuration            # env loader
/internal
--/core                   # business logic 
--/metrics                # Prometheus setup
--/httpserver             # health/metrics/pprof server
--/adapter
----/discord              # Discord slash handlers 
--/integrations
----/openai               # OpenAI client
```

---

## Roadmap

* **v1 (now)**: VC Sentry + /ask with GPT-5
* **v1.1**: Durable DM outbox in bbolt, admin rate limits, pretty embeds
* **v2**: Voice activation (capture → STT → trigger)

---

## Notes

* bbolt buckets used: `settings`, `subscribers`, `vc_presence`, `last_dm`, `chat_history`.
* Chat history is per user, max 10 turns, expires after 24h.
* Rotate your Discord bot token & OpenAI key if they ever leak.
