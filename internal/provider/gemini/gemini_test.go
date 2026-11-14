package gemini

import (
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/genai"

	"github.com/madalinpopa/cmt/internal/commit"
)

func newTestProvider(t *testing.T, resp map[string]any, gotBody *map[string]any) *Provider {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		if gotBody != nil {
			if err := json.Unmarshal(body, gotBody); err != nil {
				t.Errorf("decode body: %v", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.MarshalWrite(w, resp); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	p, err := New(t.Context(), DefaultModel, &genai.ClientConfig{
		APIKey:      "test-key",
		Backend:     genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{BaseURL: srv.URL},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

func response(finishReason string, parts ...map[string]any) map[string]any {
	return map[string]any{
		"candidates": []map[string]any{{
			"content":      map[string]any{"role": "model", "parts": parts},
			"finishReason": finishReason,
		}},
	}
}

func TestGenerate(t *testing.T) {
	t.Parallel()
	var body map[string]any
	p := newTestProvider(t, response("STOP",
		map[string]any{"text": "thinking", "thought": true},
		map[string]any{"text": `{"type":"feat","scope":"git","subject":"add reader","body":"","breaking":false}`},
	), &body)

	prompt := commit.BuildPrompt(commit.Input{Diff: "+line", Style: commit.Conventional})
	got, err := p.Generate(t.Context(), prompt)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if want := (commit.Message{Type: "feat", Scope: "git", Subject: "add reader"}); got != want {
		t.Errorf("Generate = %+v, want %+v", got, want)
	}

	cfg, _ := body["generationConfig"].(map[string]any)
	if cfg["responseMimeType"] != "application/json" || cfg["responseJsonSchema"] == nil {
		t.Errorf("generationConfig = %v", cfg)
	}
	if body["systemInstruction"] == nil {
		t.Error("systemInstruction is missing")
	}
}

func TestGenerateErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		resp    map[string]any
		wantErr string
	}{
		{"blocked", map[string]any{"promptFeedback": map[string]any{"blockReason": "SAFETY"}}, "blocked"},
		{"no candidates", map[string]any{}, "no candidates"},
		{"max tokens", response("MAX_TOKENS", map[string]any{"text": "{"}), "token limit"},
		{"safety", response("SAFETY"), "stopped: SAFETY"},
		{"invalid json", response("STOP", map[string]any{"text": "not json"}), "decode message"},
		{"no text", response("STOP"), "no text content"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := newTestProvider(t, tt.resp, nil)
			_, err := p.Generate(t.Context(), commit.Prompt{})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Generate error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
