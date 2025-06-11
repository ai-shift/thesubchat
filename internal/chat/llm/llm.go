package llm

import (
	"context"
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"log"
)

type LLM struct {
	ctx context.Context
	g   *genkit.Genkit
}

type Response struct {
	Text string
}

func New(ctx context.Context) *LLM {
	g, err := genkit.Init(ctx,
		genkit.WithPlugins(&googlegenai.GoogleAI{}),
		genkit.WithDefaultModel("googleai/gemini-2.0-flash"),
	)
	if err != nil {
		log.Fatalf("could not initialize Genkit: %v", err)
	}
	return &LLM{
		ctx: ctx,
		g:   g,
	}
}

func (m LLM) Eval(prompt string) (*Response, error) {
	resp, err := genkit.Generate(m.ctx, m.g, ai.WithPrompt(prompt))
	if err != nil {
		return nil, err
	}
	return &Response{
		Text: resp.Text(),
	}, nil
}
