package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestPoePreset(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	p, err := FromPreset("poe")
	if err != nil {
		t.Fatal(err)
	}
	if pr := Preset("poe"); pr.Kind != KindRelay || pr.NoKey || p.Icon != "poe" || p.KeysURL != "https://poe.com/api/keys" {
		t.Fatalf("Poe must be a keyed relay with its icon and key page: %+v", pr)
	}
	if p.Chat != "https://api.poe.com/v1" || p.Responses != p.Chat || p.Anthropic != "https://api.poe.com" {
		t.Fatalf("Poe endpoints: %q %q %q", p.Chat, p.Responses, p.Anthropic)
	}
	// An existing OpenAI-compatible entry is recognized on import and gains
	// the preset's other APIs, with the user's key kept.
	p, why := imported("Poe", "poe-key", endpoints{chat: p.Chat}, nil)
	if why != "" || p.Preset != "poe" || p.Key != "poe-key" || p.Responses == "" || p.Anthropic == "" {
		t.Fatalf("Poe import: %+v, %s", p, why)
	}
	// Selected fields from Poe's public /v1/models on 2026-10-08. Messages
	// is offered for Claude, while GPT stays on Chat and Responses.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer poe-key" {
			t.Errorf("model request: %s, auth %q", r.URL.Path, r.Header.Get("Authorization"))
			http.Error(w, "bad model request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"object":"list","data":[
			{"id":"gpt-5.4","object":"model","owned_by":"OpenAI","context_length":1050000,"supported_endpoints":["/v1/responses","/v1/chat/completions"]},
			{"id":"claude-sonnet-4.6","object":"model","owned_by":"Anthropic","context_length":983040,"supported_endpoints":["/v1/messages","/v1/responses","/v1/chat/completions"]}
		]}`))
	}))
	defer srv.Close()
	p.Chat, p.Responses, p.Anthropic = srv.URL+"/v1", srv.URL+"/v1", srv.URL
	ms, err := p.Fetch(context.Background())
	if err != nil || len(ms) != 2 {
		t.Fatalf("Poe models: %+v, %v", ms, err)
	}
	for _, tc := range []struct {
		model string
		apis  []Protocol
	}{
		{"gpt-5.4", []Protocol{Responses, Chat}},
		{"claude-sonnet-4.6", []Protocol{Anthropic, Responses, Chat}},
	} {
		if got := p.APIs(tc.model); !slices.Equal(got, tc.apis) {
			t.Errorf("%s APIs: %v, want %v", tc.model, got, tc.apis)
		}
	}
}
