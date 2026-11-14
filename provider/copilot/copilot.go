package copilot

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"slices"
	"time"

	sdk "github.com/github/copilot-sdk/go"

	"github.com/madalinpopa/cmt/commit"
)

const (
	DefaultModel   = "auto"
	cleanupTimeout = 5 * time.Second
)

type Provider struct {
	client *sdk.Client
	model  string
}

func New(model string) (*Provider, error) {
	path, err := cliPath()
	if err != nil {
		return nil, err
	}
	client := sdk.NewClient(&sdk.ClientOptions{
		Connection: sdk.StdioConnection{Path: path},
		LogLevel:   "error",
	})
	return &Provider{client: client, model: model}, nil
}

func cliPath() (string, error) {
	if path := os.Getenv("COPILOT_CLI_PATH"); path != "" {
		return path, nil
	}
	path, err := exec.LookPath("copilot")
	if err != nil {
		return "", errors.New("copilot: GitHub Copilot CLI not found in PATH; install it or set COPILOT_CLI_PATH")
	}
	return path, nil
}

func builtinMCPServers() []string {
	return []string{"github-mcp-server", "githubiq"}
}

func (p *Provider) disabledMCPServers(ctx context.Context) []string {
	names := builtinMCPServers()
	config, err := p.client.RPC.MCP.Config().List(ctx)
	if err != nil {
		return names
	}
	return append(names, slices.Sorted(maps.Keys(config.Servers))...)
}

func (p *Provider) Generate(ctx context.Context, pr commit.Prompt) (msg commit.Message, err error) {
	if err := p.client.Start(ctx); err != nil {
		return commit.Message{}, fmt.Errorf("copilot: start runtime: %w", err)
	}

	session, err := p.client.CreateSession(ctx, &sdk.SessionConfig{
		ClientName:             "cmt",
		Model:                  p.model,
		AvailableTools:         []string{},
		DisabledMCPServers:     p.disabledMCPServers(ctx),
		EnableConfigDiscovery:  new(false),
		SkipCustomInstructions: new(true),
		SystemMessage:          &sdk.SystemMessageConfig{Mode: "replace", Content: pr.System},
	})
	if err != nil {
		return commit.Message{}, fmt.Errorf("copilot: create session: %w", err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		if derr := p.client.DeleteSession(cleanupCtx, session.SessionID); derr != nil {
			err = errors.Join(err, fmt.Errorf("copilot: delete session: %w", derr))
		}
	}()

	event, err := session.SendAndWait(ctx, sdk.MessageOptions{
		Prompt:         pr.User,
		ResponseSchema: pr.Style.Schema(),
	})
	if err != nil {
		return commit.Message{}, fmt.Errorf("copilot: %w", err)
	}

	if event == nil {
		return commit.Message{}, errors.New("copilot: response has no assistant message")
	}
	data, ok := event.Data.(*sdk.AssistantMessageData)
	if !ok {
		return commit.Message{}, errors.New("copilot: response has no assistant message")
	}
	if err := json.Unmarshal([]byte(data.Content), &msg); err != nil {
		return commit.Message{}, fmt.Errorf("copilot: decode message: %w", err)
	}
	return msg, nil
}

func (p *Provider) Close() error {
	if err := p.client.Stop(); err != nil {
		return fmt.Errorf("copilot: stop runtime: %w", err)
	}
	return nil
}
