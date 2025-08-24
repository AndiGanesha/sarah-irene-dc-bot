package discord

import (
	"context"
	"strings"
	"time"

	"github.com/AndiGanesha/sarah-irene-dc-bot/internal/core/ask"
	"github.com/bwmarrin/discordgo"
)

type AskHandler struct {
	Svc *ask.Service
}

func (h *AskHandler) Register(s *discordgo.Session) error {
	_, err := s.ApplicationCommandCreate(s.State.User.ID, "", &discordgo.ApplicationCommand{
		Name:        "ask",
		Description: "Ask an AI a question",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "q",
				Description: "Your question",
				Required:    true,
			},
		},
	})
	return err
}

func (h *AskHandler) OnInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	if i.ApplicationCommandData().Name != "ask" {
		return
	}

	q := i.ApplicationCommandData().Options[0].StringValue()
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: "Thinking…"},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ans, err := h.Svc.Answer(ctx, strings.TrimSpace(q))
	if err != nil {
		ans = "Sorry, I couldn’t get an answer right now."
	}

	_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &ans})
}
