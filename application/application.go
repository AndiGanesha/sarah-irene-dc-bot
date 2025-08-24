package application

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/AndiGanesha/sarah-irene-dc-bot/configuration"
	"github.com/AndiGanesha/sarah-irene-dc-bot/internal/adapter/discord"
	"github.com/AndiGanesha/sarah-irene-dc-bot/internal/bot"
	"github.com/AndiGanesha/sarah-irene-dc-bot/internal/core/ask"
	"github.com/AndiGanesha/sarah-irene-dc-bot/internal/httpserver"
	"github.com/AndiGanesha/sarah-irene-dc-bot/internal/integrations/openai"
	"github.com/AndiGanesha/sarah-irene-dc-bot/internal/metrics"

	// external packages
	bolt "go.etcd.io/bbolt"
)

const (
	AppName = "sarah-irene-dc-bot"

	// Buckets used by Sarah Irene Sentry VC (create if not exist)
	bktSettings    = "settings"    // singleton app.Configuration (guild_id, voice_channel_id)
	bktSubscribers = "subscribers" // user_id -> 1
	bktPresence    = "vc_presence" // user_id -> JSON{game, updated_at}
	bktLastDM      = "last_dm"     // user_id -> JSON{state_hash, sent_at}
	bktDMOutbox    = "dm_outbox"   // optional durable queue (v1.1)
	bktChatHistory = "chat_history"
)

type App struct {
	Name          string
	Configuration *configuration.Configuration

	// lifecycle
	Context       context.Context
	ContextCancel context.CancelFunc

	// infrastructure
	DB      *bolt.DB
	Bot     *bot.Bot
	Metrics *metrics.Metrics
	HTTP    *httpserver.Server
}

func NewApp(ctx context.Context, ctxCancel context.CancelFunc) (*App, error) {
	// initiate app
	app := &App{
		Name:          AppName,
		Context:       ctx,
		ContextCancel: ctxCancel,
	}

	// load config from env
	appConfig, err := configuration.LoadConfiguration()
	if err != nil {
		log.Println("load config error", err)
		return nil, err
	}
	app.Configuration = appConfig

	// Set sensible defaults if env not provided
	if app.Configuration.Bbolt.Path == "" {
		app.Configuration.Bbolt.Path = "./vc-sentry.db"
	}
	if app.Configuration.Bbolt.TimeoutMS <= 0 {
		app.Configuration.Bbolt.TimeoutMS = 3000
	}

	// open database bbolt
	db, err := bolt.Open(app.Configuration.Bbolt.Path, 0o600, &bolt.Options{
		Timeout: time.Duration(app.Configuration.Bbolt.TimeoutMS) * time.Millisecond,
	})
	if err != nil {
		log.Println("load config error", err)
		return nil, err
	}
	log.Println("connected to DB", appConfig.Bbolt.Path)
	app.DB = db

	if err := db.Update(func(tx *bolt.Tx) error {
		buckets := []string{
			bktSettings,
			bktSubscribers,
			bktPresence,
			bktLastDM,
			bktDMOutbox,
			bktChatHistory,
		}
		for _, name := range buckets {
			if _, e := tx.CreateBucketIfNotExists([]byte(name)); e != nil {
				return fmt.Errorf("create bucket %q: %w", name, e)
			}
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, err
	}

	// Initialize metrics and server
	m := metrics.New()
	app.Metrics = m
	app.HTTP = httpserver.New(app.Configuration.Server.HTTP, m.Registry)
	go func() {
		if err := app.HTTP.Start(); err != nil && err.Error() != "http: Server closed" {
			log.Println("http server error:", err)
			app.ContextCancel()
		}
	}()
	log.Println("server is up at", appConfig.Server.HTTP)

	// Start OpenAI client
	oa := openai.New(app.Configuration.OpenAI.APIKey, app.Configuration.OpenAI.Model)
	askSvc := ask.New(oa, app.DB)
	askCmd := &discord.AskHandler{Svc: askSvc}

	// Initialize Bot
	b, err := bot.New(ctx, bot.Config{
		Token:          app.Configuration.Discord.Token,
		GuildID:        app.Configuration.Discord.GuildID,
		VoiceChannelID: app.Configuration.Discord.VoiceChannelID,
		Store:          db,
		Metrics:        m,
		AskCmd:         askCmd,
	})
	if err != nil {
		return nil, fmt.Errorf("bot.New: %w", err)
	}
	app.Bot = b

	// Start Bot
	if err := app.Bot.Start(); err != nil {
		return nil, fmt.Errorf("bot.Start: %w", err)
	}
	log.Println("Discord bot is running")

	return app, nil
}

func (app *App) Close() {
	// Root context cancel
	if app.ContextCancel != nil {
		app.ContextCancel()
	}

	// Stop bot (gateway, workers) with a small timeout context
	if app.Bot != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		app.Bot.Stop(ctx)
		cancel()
	}

	// Stop HTTP
	if app.HTTP != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = app.HTTP.Stop(ctx)
		cancel()
	}

	// Close DB
	if app.DB != nil {
		if err := app.DB.Close(); err != nil {
			log.Println("error closing bbolt:", err)
		}
	}

	log.Println("exited gracefully")
}
