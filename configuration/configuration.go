package configuration

import (
	"log"

	env "github.com/Netflix/go-env"
	"github.com/joho/godotenv"
)

func LoadConfiguration() (*Configuration, error) {
	// Try load .env or local.env if present
	if err := godotenv.Load(".env"); err != nil {
		if err2 := godotenv.Load("local.env"); err2 != nil {
			log.Println("no .env or local.env found, using system env")
		}
	}
	config := &Configuration{}
	_, err := env.UnmarshalFromEnviron(config)
	return config, err
}

type Configuration struct {
	Bbolt   Bbolt
	Server  Server
	Discord Discord
	OpenAI  OpenAI
}

type Bbolt struct {
	Path      string `env:"BBOLT_PATH"`
	TimeoutMS int    `env:"BBOLT_TIMEOUT_MS"`
}

type Server struct {
	HTTP string `env:"SERVER_HTTP"`
}

type Discord struct {
	Token          string `env:"DISCORD_BOT_TOKEN"`
	GuildID        string `env:"GUILD_ID"`
	VoiceChannelID string `env:"VOICE_CHANNEL_ID"`
}

type OpenAI struct {
	APIKey string `env:"OPENAI_API_KEY"`
}
