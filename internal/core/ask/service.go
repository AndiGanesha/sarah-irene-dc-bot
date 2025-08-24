package ask

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	maxTurns          = 20
	historyTTL        = 24 * time.Hour
	promptHeader      = "You are a helpful assistant. Answer briefly and clearly. If you don't get reliable source, say you don't know."
	separator         = "\n\n---\n\n"
	bucketChatHistory = "chat_history"
)

type LLM interface {
	Ask(ctx context.Context, q string) (string, error)
}

type Service struct {
	LLM LLM
	DB  *bolt.DB
}

func New(llm LLM, db *bolt.DB) *Service {
	return &Service{LLM: llm, DB: db}
}

type msg struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp int64  `json:"ts"`
}

// Answer builds context from per-user history (≤24h), asks LLM, and saves the turn.
func (s *Service) Answer(ctx context.Context, userID, question string) (string, error) {
	if strings.TrimSpace(question) == "" {
		return "", fmt.Errorf("empty question")
	}

	h, _ := s.loadHistory(userID)
	h = pruneTTL(h, historyTTL)
	h = trimMax(h, maxTurns-1)

	prompt := renderPrompt(h, question)

	answer, err := s.LLM.Ask(ctx, prompt)
	if err != nil {
		return "", err
	}

	now := time.Now().Unix()
	h = append(h, msg{Role: "user", Content: question, Timestamp: now})
	h = append(h, msg{Role: "assistant", Content: answer, Timestamp: now})
	h = trimMax(h, maxTurns)
	_ = s.saveHistory(userID, h)

	return answer, nil
}

// ---- storage helpers (bbolt) ----

func (s *Service) loadHistory(userID string) ([]msg, error) {
	var out []msg
	err := s.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketChatHistory))
		if b == nil {
			return nil
		}
		v := b.Get([]byte(userID))
		if v == nil {
			return nil
		}
		return json.Unmarshal(v, &out)
	})
	return out, err
}

func (s *Service) saveHistory(userID string, history []msg) error {
	data, _ := json.Marshal(history)
	return s.DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketChatHistory))
		if b == nil {
			return fmt.Errorf("bucket %q not found", bucketChatHistory)
		}
		return b.Put([]byte(userID), data)
	})
}

// ---- utils ----

func pruneTTL(h []msg, ttl time.Duration) []msg {
	cut := time.Now().Add(-ttl).Unix()
	out := h[:0]
	for _, m := range h {
		if m.Timestamp >= cut {
			out = append(out, m)
		}
	}
	return out
}

func trimMax(h []msg, n int) []msg {
	if n <= 0 || len(h) <= n {
		return h
	}
	return h[len(h)-n:]
}

func renderPrompt(history []msg, question string) string {
	var b strings.Builder
	b.WriteString(promptHeader)
	if len(history) > 0 {
		b.WriteString(separator)
		for _, m := range history {
			switch m.Role {
			case "user":
				b.WriteString("User: ")
			case "assistant":
				b.WriteString("Assistant: ")
			default:
				b.WriteString("User: ")
			}
			b.WriteString(strings.TrimSpace(m.Content))
			b.WriteString("\n")
		}
	}
	b.WriteString(separator)
	b.WriteString("User: ")
	b.WriteString(strings.TrimSpace(question))
	b.WriteString("\nAssistant:")
	return b.String()
}
