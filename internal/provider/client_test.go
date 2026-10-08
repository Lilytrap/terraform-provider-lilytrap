package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCheckPolicySendsPathsAndExtraRules(t *testing.T) {
	var got map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/policy/check" || r.Header.Get("authorization") != "Bearer wsk_test" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"ignored":["vault/secret/prod/admin"]}`))
	}))
	defer srv.Close()
	c := &Client{APIURL: srv.URL, APIKey: "wsk_test", HTTP: srv.Client()}

	ignored, err := c.CheckPolicy(context.Background(), []string{"vault/secret/prod/admin", "vault/secret/dev/admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ignored, []string{"vault/secret/prod/admin"}) {
		t.Fatalf("ignored = %v", ignored)
	}
	if got["extra"] == nil || len(got["paths"]) != 2 {
		t.Fatalf("request body = %v; extra must be [] not null", got)
	}
}

func TestOlderAPIsWithoutPolicyOrSeenRoutes(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	c := &Client{APIURL: srv.URL, APIKey: "wsk_test", HTTP: srv.Client()}
	if _, err := c.CheckPolicy(context.Background(), []string{"a"}, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CheckPolicy on an old API: %v, want ErrNotFound", err)
	}
	if err := c.TouchBuild(context.Background(), "bld_x"); err != nil {
		t.Fatalf("TouchBuild on an old API should be a no-op, got %v", err)
	}
}

func TestTouchBuildUsesTheAgentRoute(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.Method + " " + r.URL.Path
		_, _ = w.Write([]byte(`{"seen":"bld_tfp_1"}`))
	}))
	defer srv.Close()
	c := &Client{APIURL: srv.URL, APIKey: "wsk_test", HTTP: srv.Client()}
	if err := c.TouchBuild(context.Background(), "bld_tfp_1"); err != nil {
		t.Fatal(err)
	}
	if path != "POST /agent/v1/builds/bld_tfp_1/seen" {
		t.Fatalf("called %s", path)
	}
}
