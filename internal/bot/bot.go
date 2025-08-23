package bot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"

	"github.com/AndiGanesha/sarah-irene-dc-bot/internal/metrics"
	"github.com/AndiGanesha/sarah-irene-dc-bot/internal/store"
)

type Config struct {
	Token          string
	GuildID        string
	VoiceChannelID string
	Store          *store.DB
	Metrics        *metrics.Metrics
}

type Bot struct {
	cfg     Config
	session *discordgo.Session
	jobs    chan DMJob
	log     *zap.Logger
}

type DMJob struct {
	UserID string
	Reason string
	Body   string
}

func New(ctx context.Context, cfg Config) (*Bot, error) {
	s, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, err
	}
	s.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuildPresences | discordgo.IntentsDirectMessages

	logger, _ := zap.NewProduction()
	b := &Bot{cfg: cfg, session: s, jobs: make(chan DMJob, 256), log: logger}
	return b, nil
}

func (b *Bot) Start() error {
	b.session.AddHandler(b.onVoiceState)
	b.session.AddHandler(b.onPresence)
	b.session.AddHandler(b.onMessage)

	if err := b.session.Open(); err != nil {
		return err
	}

	// workers
	for i := 0; i < 4; i++ {
		go b.worker()
	}
	return nil
}

func (b *Bot) Stop(ctx context.Context) {
	b.session.Close()
	// allow workers to exit naturally (omitted for brevity)
}

func (b *Bot) onVoiceState(s *discordgo.Session, e *discordgo.VoiceStateUpdate) {
	b.cfg.Metrics.EventsTotal.WithLabelValues("voice").Inc()
	// TODO: update vc_presence, detect join, enqueue DMs
}

func (b *Bot) onPresence(s *discordgo.Session, e *discordgo.PresenceUpdate) {
	b.cfg.Metrics.EventsTotal.WithLabelValues("presence").Inc()
	// TODO: update game for user if in VC, detect mixed/whitelist, enqueue
}

func (b *Bot) onMessage(s *discordgo.Session, e *discordgo.MessageCreate) {
	b.cfg.Metrics.EventsTotal.WithLabelValues("message").Inc()
	// TODO: implement /status, /watch set, /watch list (or plain text cmds)
}

func (b *Bot) worker() {
	for j := range b.jobs {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = b.sendWithRetry(ctx, j)
		cancel()
	}
}

func (b *Bot) sendWithRetry(ctx context.Context, j DMJob) error {
	var err error
	backoff := 150 * time.Millisecond
	for attempt := 0; attempt < 3; attempt++ {
		_, err = b.session.UserChannelCreate(j.UserID)
		if err == nil {
			_, err = b.session.ChannelMessageSendComplex("@me", &discordgo.MessageSend{Content: j.Body})
		}
		if err == nil {
			b.cfg.Metrics.DMSentTotal.WithLabelValues(j.Reason).Inc()
			return nil
		}
		t := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			b.cfg.Metrics.DMErrorsTotal.WithLabelValues(j.Reason).Inc()
			return ctx.Err()
		case <-t.C:
			backoff *= 2
		}
	}
	b.cfg.Metrics.DMErrorsTotal.WithLabelValues(j.Reason).Inc()
	return err
}

func hashState(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
