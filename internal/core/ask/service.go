package ask

import (
	"context"
)

type LLM interface {
	Ask(ctx context.Context, q string) (string, error)
}

type Service struct {
	LLM LLM
}

func New(llm LLM) *Service { return &Service{LLM: llm} }

func (s *Service) Answer(ctx context.Context, question string) (string, error) {
	// place for prompt guards, trims, etc.
	return s.LLM.Ask(ctx, question)
}
