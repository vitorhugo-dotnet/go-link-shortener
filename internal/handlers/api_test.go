package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
	"github.com/vitorhugo-java/go-link-shortener/internal/config"
	"github.com/vitorhugo-java/go-link-shortener/internal/database"
)

func TestGetLinkReturnsAgentReadableLinkDetails(t *testing.T) {
	h := &Handler{
		cfg: &config.Config{AppHost: "short.example"},
		lookupLink: func(alias string) (database.LinkDetails, error) {
			if alias != "webmcp" {
				t.Fatalf("lookup alias = %q, want webmcp", alias)
			}
			return database.LinkDetails{Slug: "webmcp", OriginalURL: "https://developer.chrome.com/docs/ai/webmcp"}, nil
		},
	}
	app := fiber.New()
	app.Get("/api/links/:alias", h.GetLink)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/links/webmcp", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got struct {
		Alias     string `json:"alias"`
		ShortURL  string `json:"shortUrl"`
		TargetURL string `json:"targetUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Alias != "webmcp" || got.ShortURL != "http://short.example/webmcp" || got.TargetURL != "https://developer.chrome.com/docs/ai/webmcp" {
		t.Fatalf("response = %#v", got)
	}
}

func TestGetLinkAnalyticsReturnsAggregateOnly(t *testing.T) {
	lastClicked := time.Date(2026, time.September, 1, 14, 20, 0, 0, time.UTC)
	h := &Handler{
		lookupAnalytics: func(alias string) (database.LinkAnalytics, error) {
			return database.LinkAnalytics{TotalClicks: 42, LastClickedAt: &lastClicked}, nil
		},
	}
	app := fiber.New()
	app.Get("/api/links/:alias/analytics", h.GetLinkAnalytics)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/links/webmcp/analytics", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["alias"] != "webmcp" || got["totalClicks"] != float64(42) || got["lastClickedAt"] != "2026-09-01T14:20:00Z" {
		t.Fatalf("response = %#v", got)
	}
	if _, ok := got["ip"]; ok {
		t.Fatalf("response exposes IP: %#v", got)
	}
}

func TestGetLinkReturnsNotFoundForUnknownAlias(t *testing.T) {
	h := &Handler{
		lookupLink: func(string) (database.LinkDetails, error) { return database.LinkDetails{}, pgx.ErrNoRows },
	}
	app := fiber.New()
	app.Get("/api/links/:alias", h.GetLink)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/links/missing", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}
