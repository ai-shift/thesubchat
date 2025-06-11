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

type Message struct {
	Text string
	Role string
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

func (m LLM) Eval(msgs []Message) ([]Message, error) {
	mapped := make([]*ai.Message, len(msgs))
	for i, msg := range msgs {
		mapped[i] = ai.NewTextMessage(ai.Role(msg.Role), msg.Text)
	}
	resp, err := genkit.Generate(m.ctx, m.g, ai.WithMessages(mapped...))
	if err != nil {
		return msgs, err
	}
	msg := Message{
		Text: resp.Text(),
		Role: "assistant",
	}
	msgs = append(msgs, msg)
	return msgs, nil
}
