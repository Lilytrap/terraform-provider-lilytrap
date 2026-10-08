package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to the Lilytrap API with a workspace API key. Only hashes of decoy secrets are sent.
type Client struct {
	APIURL  string
	APIKey  string
	HTTP    *http.Client
	Version string
}

// ErrNotFound is returned for a 404 from the API.
var ErrNotFound = errors.New("not found")

type config struct {
	APIURL  *string `json:"apiUrl"`
	TrapURL *string `json:"trapUrl"`
}

// Manifest mirrors BuildManifest in packages/core/src/types.ts.
type Manifest struct {
	Version       int     `json:"version"`
	BuildID       string  `json:"buildId"`
	CreatedAt     string  `json:"createdAt"`
	Target        string  `json:"target"`
	Endpoint      string  `json:"endpoint"`
	Source        *Source `json:"source,omitempty"`
	IngestKeyHash string  `json:"ingestKeyHash,omitempty"`
	Tokens        []Token `json:"tokens"`
}

type Source struct {
	Kind     string   `json:"kind"`
	Name     string   `json:"name,omitempty"`
	Location string   `json:"location,omitempty"`
	Trusted  []string `json:"trusted,omitempty"`
}

type Token struct {
	ID         string   `json:"id"`
	Kit        string   `json:"kit"`
	Kind       string   `json:"kind"`
	SecretHash string   `json:"secretHash"`
	Path       string   `json:"path"`
	Method     string   `json:"method"`
	Tells      []string `json:"tells"`
	Locations  []string `json:"locations"`
	Hop        int64    `json:"hop"`
	Resources  []string `json:"resources,omitempty"`
}

// BuildSummary mirrors BuildSummary in packages/core/src/api.ts (the fields the provider reads).
type BuildSummary struct {
	BuildID    string   `json:"buildId"`
	CreatedAt  string   `json:"createdAt"`
	Target     string   `json:"target"`
	TokenCount int      `json:"tokenCount"`
	Kits       []string `json:"kits"`
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.APIURL, "/")+path, reader)
	if err != nil {
		return err
	}
	if c.APIKey != "" {
		req.Header.Set("authorization", "Bearer "+c.APIKey)
	}
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	req.Header.Set("user-agent", "terraform-provider-lilytrap/"+c.Version)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("%s %s: HTTP %d: %s", method, path, res.StatusCode, strings.TrimSpace(string(data)))
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// TrapURL is the trap surface decoys point at, from the public config endpoint.
func (c *Client) TrapURL(ctx context.Context) (string, error) {
	var cfg config
	if err := c.do(ctx, http.MethodGet, "/agent/v1/config", nil, &cfg); err != nil {
		return "", fmt.Errorf("reading Lilytrap config: %w", err)
	}
	if cfg.TrapURL == nil || *cfg.TrapURL == "" {
		return "", errors.New("the Lilytrap API didn't return a trap URL")
	}
	return strings.TrimRight(*cfg.TrapURL, "/"), nil
}

func (c *Client) Register(ctx context.Context, m Manifest) error {
	return c.do(ctx, http.MethodPost, "/v1/builds", m, nil)
}

func (c *Client) GetBuild(ctx context.Context, id string) (*BuildSummary, error) {
	var out struct {
		Build BuildSummary `json:"build"`
	}
	if err := c.do(ctx, http.MethodGet, "/agent/v1/builds/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out.Build, nil
}

// CheckPolicy returns which of paths the workspace's ignore rules (plus extra) exclude.
// ErrNotFound means the API predates ignore rules.
func (c *Client) CheckPolicy(ctx context.Context, paths, extra []string) ([]string, error) {
	if paths == nil {
		paths = []string{}
	}
	if extra == nil {
		extra = []string{}
	}
	var out struct {
		Ignored []string `json:"ignored"`
	}
	if err := c.do(ctx, http.MethodPost, "/v1/policy/check", map[string][]string{"paths": paths, "extra": extra}, &out); err != nil {
		return nil, err
	}
	return out.Ignored, nil
}

// TouchBuild tells Lilytrap a deployment is still there, so it isn't reported as stale.
// Older APIs don't have the route; that's not an error.
func (c *Client) TouchBuild(ctx context.Context, id string) error {
	err := c.do(ctx, http.MethodPost, "/agent/v1/builds/"+url.PathEscape(id)+"/seen", nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (c *Client) RetireBuild(ctx context.Context, id string) error {
	err := c.do(ctx, http.MethodDelete, "/agent/v1/builds/"+url.PathEscape(id), nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func nowISO() string { return time.Now().UTC().Format("2006-01-02T15:04:05.000Z") }
