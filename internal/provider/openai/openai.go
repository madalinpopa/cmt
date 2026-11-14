package openai

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"

	sdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/madalinpopa/cmt/internal/commit"
)

const (
	DefaultModel    = sdk.ChatModelGPT5_4Mini
	maxOutputTokens = 16000
)

type Provider struct {
	client sdk.Client
	model  string
}

func New(model string, opts ...option.RequestOption) *Provider {
	return &Provider{client: sdk.NewClient(opts...), model: model}
}

func (p *Provider) Generate(ctx context.Context, pr commit.Prompt) (commit.Message, error) {
	resp, err := p.client.Responses.New(ctx, responses.ResponseNewParams{
		Model:           p.model,
		Instructions:    sdk.String(pr.System),
		Input:           responses.ResponseNewParamsInputUnion{OfString: sdk.String(pr.User)},
		MaxOutputTokens: sdk.Int(maxOutputTokens),
		Store:           sdk.Bool(false),
		Reasoning:       shared.ReasoningParam{Effort: shared.ReasoningEffortLow},
		Text: responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigUnionParam{
				OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{
					Name:   "commit_message",
					Schema: pr.Style.Schema(),
					Strict: sdk.Bool(true),
				},
			},
		},
	})
	if err != nil {
		return commit.Message{}, fmt.Errorf("openai: %w", err)
	}

	switch resp.Status {
	case responses.ResponseStatusCompleted:
	case responses.ResponseStatusIncomplete:
		if resp.IncompleteDetails.Reason == "max_output_tokens" {
			return commit.Message{}, errors.New("openai: response was cut off at the token limit")
		}
		return commit.Message{}, fmt.Errorf("openai: response is incomplete: %s", resp.IncompleteDetails.Reason)
	default:
		return commit.Message{}, fmt.Errorf("openai: response status %s", resp.Status)
	}

	for _, item := range resp.Output {
		if item.Type != "message" || (item.Phase != "" && item.Phase != responses.ResponseOutputMessagePhaseFinalAnswer) {
			continue
		}
		for _, part := range item.Content {
			switch part.Type {
			case "refusal":
				return commit.Message{}, fmt.Errorf("openai: request refused: %s", part.Refusal)
			case "output_text":
				var msg commit.Message
				if err := json.Unmarshal([]byte(part.Text), &msg); err != nil {
					return commit.Message{}, fmt.Errorf("openai: decode message: %w", err)
				}
				return msg, nil
			}
		}
	}
	return commit.Message{}, errors.New("openai: response has no text content")
}
