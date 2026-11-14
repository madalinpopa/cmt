package openai

import (
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openai/openai-go/v3/option"

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

	return New(DefaultModel,
		option.WithBaseURL(srv.URL),
		option.WithAPIKey("test-key"),
		option.WithMaxRetries(0),
	)
}

func response(status string, incompleteReason string, content ...map[string]any) map[string]any {
	resp := map[string]any{
		"id":         "resp_test",
		"object":     "response",
		"created_at": 0,
		"model":      DefaultModel,
		"status":     status,
		"output": []map[string]any{{
			"id":      "msg_test",
			"type":    "message",
			"role":    "assistant",
			"status":  "completed",
			"content": content,
		}},
	}
	if incompleteReason != "" {
		resp["incomplete_details"] = map[string]any{"reason": incompleteReason}
	}
	return resp
}

func outputText(text string) map[string]any {
	return map[string]any{"type": "output_text", "text": text, "annotations": []any{}}
}

func TestGenerate(t *testing.T) {
	t.Parallel()
	var body map[string]any
	p := newTestProvider(t, response("completed", "", outputText(`{"type":"feat","scope":"git","subject":"add reader","body":"","breaking":false}`)), &body)

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
	if body["store"] != false {
		t.Errorf("store = %v", body["store"])
	}
	format, _ := body["text"].(map[string]any)["format"].(map[string]any)
	if format["type"] != "json_schema" || format["schema"] == nil || format["strict"] != true {
		t.Errorf("text.format = %v", format)
	}
}

func TestGenerateErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		resp    map[string]any
		wantErr string
	}{
		{"refusal", response("completed", "", map[string]any{"type": "refusal", "refusal": "no"}), "refused"},
		{"max tokens", response("incomplete", "max_output_tokens"), "token limit"},
		{"content filter", response("incomplete", "content_filter"), "content_filter"},
		{"failed", response("failed", ""), "status failed"},
		{"invalid json", response("completed", "", outputText("not json")), "decode message"},
		{"no text", response("completed", ""), "no text content"},
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
