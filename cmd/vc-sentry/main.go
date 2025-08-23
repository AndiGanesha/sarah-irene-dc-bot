package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AndiGanesha/sarah-irene-dc-bot/internal/bot"
	"github.com/AndiGanesha/sarah-irene-dc-bot/internal/httpserver"
	"github.com/AndiGanesha/sarah-irene-dc-bot/internal/metrics"
	"github.com/AndiGanesha/sarah-irene-dc-bot/internal/store"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Config from env
	token := os.Getenv("DISCORD_BOT_TOKEN")
	guildID := os.Getenv("GUILD_ID")
	voiceCh := os.Getenv("VOICE_CHANNEL_ID")
	if token == "" || guildID == "" || voiceCh == "" {
		log.Fatal("DISCORD_BOT_TOKEN, GUILD_ID, VOICE_CHANNEL_ID are required")
	}

	// Metrics & pprof HTTP server
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	m := metrics.New()
	hsrv := httpserver.New(addr, m.Registry)
	go func() { _ = hsrv.Start() }()

	// SQLite store
	db, closeDB, err := store.Open("file:vc-sentry.db?_busy_timeout=3000&_journal_mode=WAL")
	if err != nil {
		log.Fatal(err)
	}
	defer closeDB()

	// Bot
	b, err := bot.New(ctx, bot.Config{
		Token:          token,
		GuildID:        guildID,
		VoiceChannelID: voiceCh,
		Store:          db,
		Metrics:        m,
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := b.Start(); err != nil {
		log.Fatal(err)
	}

	<-ctx.Done()
	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	b.Stop(shutdownCtx)
	hsrv.Stop(shutdownCtx)
	log.Println("exited gracefully")
}
