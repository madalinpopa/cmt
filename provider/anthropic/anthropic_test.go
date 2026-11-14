package anthropic

import (
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"github.com/madalinpopa/cmt/commit"
)

func newTestProvider(t *testing.T, stopReason, text string, gotBody *map[string]any) *Provider {
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

		resp := map[string]any{
			"id":            "msg_test",
			"type":          "message",
			"role":          "assistant",
			"model":         DefaultModel,
			"content":       []map[string]any{{"type": "text", "text": text}},
			"stop_reason":   stopReason,
			"stop_sequence": nil,
			"usage":         map[string]any{"input_tokens": 1, "output_tokens": 1},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.MarshalWrite(w, resp); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	return New(DefaultModel,
		option.WithBaseURL(srv.URL),
		option.WithAPIKey("test-key"),
		option.WithMaxRetries(0),
	)
}

func TestGenerate(t *testing.T) {
	t.Parallel()
	var body map[string]any
	p := newTestProvider(t, "end_turn", `{"type":"feat","scope":"git","subject":"add reader","body":"","breaking":false}`, &body)

	prompt := commit.BuildPrompt(commit.Input{Diff: "+line", Style: commit.Conventional})
	got, err := p.Generate(t.Context(), prompt)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if want := (commit.Message{Type: "feat", Scope: "git", Subject: "add reader"}); got != want {
		t.Errorf("Generate = %+v, want %+v", got, want)
	}

	if body["model"] != DefaultModel {
		t.Errorf("model = %v", body["model"])
	}
	format, _ := body["output_config"].(map[string]any)["format"].(map[string]any)
	if format["type"] != "json_schema" || format["schema"] == nil {
		t.Errorf("output_config.format = %v", format)
	}
}

func TestGenerateStopReasons(t *testing.T) {
	t.Parallel()
	tests := []struct {
		stopReason string
		wantErr    string
	}{
		{"refusal", "refused"},
		{"max_tokens", "token limit"},
	}
	for _, tt := range tests {
		t.Run(tt.stopReason, func(t *testing.T) {
			t.Parallel()
			p := newTestProvider(t, tt.stopReason, `{}`, nil)
			_, err := p.Generate(t.Context(), commit.Prompt{})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Generate error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateInvalidJSON(t *testing.T) {
	t.Parallel()
	p := newTestProvider(t, "end_turn", "not json", nil)
	if _, err := p.Generate(t.Context(), commit.Prompt{}); err == nil || !strings.Contains(err.Error(), "decode message") {
		t.Errorf("Generate error = %v", err)
	}
}
