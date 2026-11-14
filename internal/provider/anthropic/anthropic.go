package anthropic

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"

	sdk "github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"github.com/madalinpopa/cmt/internal/commit"
)

const (
	DefaultModel = "claude-haiku-5-5"
	maxTokens    = 16000
)

type Provider struct {
	client sdk.Client
	model  string
}

func New(model string, opts ...option.RequestOption) *Provider {
	return &Provider{client: sdk.NewClient(opts...), model: model}
}

func (p *Provider) Generate(ctx context.Context, pr commit.Prompt) (commit.Message, error) {
	resp, err := p.client.Messages.New(ctx, sdk.MessageNewParams{
		Model:     p.model,
		MaxTokens: maxTokens,
		System:    []sdk.TextBlockParam{{Text: pr.System}},
		Messages: []sdk.MessageParam{
			sdk.NewUserMessage(sdk.NewTextBlock(pr.User)),
		},
		OutputConfig: sdk.OutputConfigParam{
			Effort: sdk.OutputConfigEffortLow,
			Format: sdk.JSONOutputFormatParam{Schema: pr.Style.Schema()},
		},
	})
	if err != nil {
		return commit.Message{}, fmt.Errorf("anthropic: %w", err)
	}

	switch resp.StopReason {
	case sdk.StopReasonRefusal:
		return commit.Message{}, fmt.Errorf("anthropic: request refused: %s", resp.StopDetails.Explanation)
	case sdk.StopReasonMaxTokens:
		return commit.Message{}, errors.New("anthropic: response was cut off at the token limit")
	}

	for _, block := range resp.Content {
		text, ok := block.AsAny().(sdk.TextBlock)
		if !ok {
			continue
		}
		var msg commit.Message
		if err := json.Unmarshal([]byte(text.Text), &msg); err != nil {
			return commit.Message{}, fmt.Errorf("anthropic: decode message: %w", err)
		}
		return msg, nil
	}
	return commit.Message{}, errors.New("anthropic: response has no text content")
}
