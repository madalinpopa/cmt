package gemini

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"github.com/madalinpopa/cmt/internal/commit"
)

const (
	DefaultModel    = "gemini-3.5-flash"
	maxOutputTokens = 16000
)

type Provider struct {
	client *genai.Client
	model  string
}

func New(ctx context.Context, model string, cfg *genai.ClientConfig) (*Provider, error) {
	client, err := genai.NewClient(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("gemini: %w", err)
	}
	return &Provider{client: client, model: model}, nil
}

func (p *Provider) Generate(ctx context.Context, pr commit.Prompt) (commit.Message, error) {
	resp, err := p.client.Models.GenerateContent(ctx, p.model,
		[]*genai.Content{genai.NewContentFromText(pr.User, genai.RoleUser)},
		&genai.GenerateContentConfig{
			SystemInstruction:  genai.NewContentFromText(pr.System, genai.RoleUser),
			MaxOutputTokens:    maxOutputTokens,
			ResponseMIMEType:   "application/json",
			ResponseJsonSchema: pr.Style.Schema(),
			ThinkingConfig:     &genai.ThinkingConfig{ThinkingLevel: genai.ThinkingLevelLow},
		},
	)
	if err != nil {
		return commit.Message{}, fmt.Errorf("gemini: %w", err)
	}

	if fb := resp.PromptFeedback; fb != nil && fb.BlockReason != "" {
		return commit.Message{}, fmt.Errorf("gemini: request blocked: %s", fb.BlockReason)
	}
	if len(resp.Candidates) == 0 {
		return commit.Message{}, errors.New("gemini: response has no candidates")
	}

	candidate := resp.Candidates[0]
	switch candidate.FinishReason {
	case genai.FinishReasonStop, genai.FinishReasonUnspecified, "":
	case genai.FinishReasonMaxTokens:
		return commit.Message{}, errors.New("gemini: response was cut off at the token limit")
	default:
		return commit.Message{}, fmt.Errorf("gemini: response stopped: %s", candidate.FinishReason)
	}

	var text strings.Builder
	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			if !part.Thought {
				text.WriteString(part.Text)
			}
		}
	}
	if text.Len() == 0 {
		return commit.Message{}, errors.New("gemini: response has no text content")
	}

	var msg commit.Message
	if err := json.Unmarshal([]byte(text.String()), &msg); err != nil {
		return commit.Message{}, fmt.Errorf("gemini: decode message: %w", err)
	}
	return msg, nil
}
