package copilot

import (
	"bufio"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	sdk "github.com/github/copilot-sdk/go"

	"github.com/madalinpopa/cmt/commit"
)

const (
	envFakeReply      = "CMT_FAKE_COPILOT_REPLY"
	envFakeLog        = "CMT_FAKE_COPILOT_LOG"
	envFakeSkipDelete = "CMT_FAKE_COPILOT_SKIP_DELETE"
	replyError        = "!error"
	replyWait         = "!wait"
)

func TestMain(m *testing.M) {
	if reply, ok := os.LookupEnv(envFakeReply); ok {
		if err := runFakeRuntime(os.Stdin, os.Stdout, reply, os.Getenv(envFakeLog)); err != nil && !errors.Is(err, io.EOF) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type rpcRequest struct {
	ID     jsontext.Value `json:"id,omitzero"`
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

func runFakeRuntime(r io.Reader, w io.Writer, reply, logPath string) (err error) {
	logFile, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, logFile.Close()) }()

	in := textproto.NewReader(bufio.NewReader(r))
	for {
		header, err := in.ReadMIMEHeader()
		if err != nil {
			return err
		}
		n, err := strconv.Atoi(header.Get("Content-Length"))
		if err != nil {
			return err
		}
		body := make([]byte, n)
		if _, err := io.ReadFull(in.R, body); err != nil {
			return err
		}

		var req rpcRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return err
		}
		if req.ID == nil {
			continue
		}
		if err := json.MarshalWrite(logFile, req); err != nil {
			return err
		}
		if _, err := logFile.WriteString("\n"); err != nil {
			return err
		}

		resp := map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": nil}
		switch req.Method {
		case "connect":
			resp["result"] = map[string]any{"ok": true, "protocolVersion": sdk.SDKProtocolVersion}
		case "session.create":
			resp["result"] = map[string]any{"sessionId": req.Params["sessionId"]}
		case "session.sendAndWait":
			if reply == replyWait {
				continue
			}
			if reply == replyError {
				delete(resp, "result")
				resp["error"] = map[string]any{"code": -32603, "message": "model unavailable"}
				break
			}
			resp["result"] = map[string]any{
				"messageId": "request-1",
				"aborted":   false,
				"response": map[string]any{
					"id":        "event-1",
					"timestamp": "2026-01-01T00:00:00Z",
					"type":      "assistant.message",
					"data":      map[string]any{"messageId": "response-1", "content": reply},
				},
			}
		case "mcp.config.list":
			if reply == replyError {
				delete(resp, "result")
				resp["error"] = map[string]any{"code": -32603, "message": "config unavailable"}
				break
			}
			resp["result"] = map[string]any{"servers": map[string]any{
				"playwright": map[string]any{"command": "npx"},
				"context7":   map[string]any{"command": "npx"},
			}}
		case "session.delete":
			if os.Getenv(envFakeSkipDelete) == "1" {
				continue
			}
			resp["result"] = map[string]any{"success": true}
		case "session.options.update", "session.detach":
			resp["result"] = map[string]any{"success": true}
		}

		out, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(out), out); err != nil {
			return err
		}
	}
}

func newFakeProvider(t *testing.T, reply string) (*Provider, string) {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "requests.jsonl")
	t.Setenv("COPILOT_CLI_PATH", os.Args[0])
	t.Setenv(envFakeReply, reply)
	t.Setenv(envFakeLog, logPath)

	p, err := New(DefaultModel)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p, logPath
}

func readRequests(t *testing.T, logPath string) map[string]map[string]any {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	requests := make(map[string]map[string]any)
	for line := range strings.Lines(string(data)) {
		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			t.Fatalf("decode request %q: %v", line, err)
		}
		requests[req.Method] = req.Params
	}
	return requests
}

func TestGenerate(t *testing.T) {
	p, logPath := newFakeProvider(t, `{"type":"feat","scope":"git","subject":"add reader","body":"","breaking":false}`)

	prompt := commit.BuildPrompt(commit.Input{Diff: "+line", Style: commit.Conventional})
	got, err := p.Generate(t.Context(), prompt)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if want := (commit.Message{Type: "feat", Scope: "git", Subject: "add reader"}); got != want {
		t.Errorf("Generate = %+v, want %+v", got, want)
	}
	if err := p.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}

	requests := readRequests(t, logPath)
	create := requests["session.create"]
	if create["model"] != DefaultModel {
		t.Errorf("model = %v", create["model"])
	}
	if tools, ok := create["availableTools"].([]any); !ok || len(tools) != 0 {
		t.Errorf("availableTools = %v, want []", create["availableTools"])
	}
	if got, want := fmt.Sprint(create["disabledMcpServers"]), "[github-mcp-server githubiq context7 playwright]"; got != want {
		t.Errorf("disabledMcpServers = %s, want %s", got, want)
	}
	system, _ := create["systemMessage"].(map[string]any)
	if system["mode"] != "replace" || system["content"] != prompt.System {
		t.Errorf("systemMessage = %v", system)
	}

	send := requests["session.sendAndWait"]
	if send["prompt"] != prompt.User {
		t.Errorf("prompt = %v", send["prompt"])
	}
	format, _ := send["responseFormat"].(map[string]any)
	schema, _ := format["jsonSchema"].(map[string]any)
	if format["type"] != "json_schema" || schema["schema"] == nil {
		t.Errorf("responseFormat = %v", format)
	}

	for _, method := range []string{"session.delete", "runtime.shutdown"} {
		if _, ok := requests[method]; !ok {
			t.Errorf("%s was not called", method)
		}
	}
}

func TestGenerateErrors(t *testing.T) {
	tests := []struct {
		name    string
		reply   string
		wantErr string
	}{
		{"invalid json", "not json", "decode message"},
		{"runtime error", replyError, "model unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, logPath := newFakeProvider(t, tt.reply)
			t.Cleanup(func() {
				if err := p.Close(); err != nil {
					t.Errorf("Close: %v", err)
				}
			})

			_, err := p.Generate(t.Context(), commit.Prompt{User: "diff"})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Generate error = %v, want %q", err, tt.wantErr)
			}
			requests := readRequests(t, logPath)
			if _, ok := requests["session.delete"]; !ok {
				t.Error("session.delete was not called")
			}
			if _, ok := requests["session.create"]["disabledMcpServers"]; !ok {
				t.Error("session.create did not disable MCP servers")
			}
		})
	}
}

func TestNewWithoutCLI(t *testing.T) {
	t.Setenv("COPILOT_CLI_PATH", "")
	t.Setenv("PATH", t.TempDir())

	if _, err := New(DefaultModel); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("New error = %v", err)
	}
}

func TestGenerateCleanupTimeout(t *testing.T) {
	t.Setenv(envFakeSkipDelete, "1")
	p, logPath := newFakeProvider(t, `{"type":"fix","scope":"","subject":"handle input","body":"","breaking":false}`)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	guardCtx, cancel := context.WithTimeout(t.Context(), 3*cleanupTimeout)
	defer cancel()
	stop := context.AfterFunc(guardCtx, p.client.ForceStop)
	defer stop()

	_, err := p.Generate(t.Context(), commit.Prompt{User: "diff"})
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "delete session") {
		t.Fatalf("Generate error = %v; want cleanup deadline exceeded", err)
	}
	if _, ok := readRequests(t, logPath)["session.delete"]; !ok {
		t.Error("session.delete was not called")
	}
}

func TestGenerateCanceledStillDeletesSession(t *testing.T) {
	p, logPath := newFakeProvider(t, replyWait)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	_, err := p.Generate(ctx, commit.Prompt{User: "diff"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Generate error = %v; want deadline exceeded", err)
	}
	if strings.Contains(err.Error(), "delete session") {
		t.Errorf("cleanup failed after cancellation: %v", err)
	}
	if _, ok := readRequests(t, logPath)["session.delete"]; !ok {
		t.Error("session.delete was not called after cancellation")
	}
}
