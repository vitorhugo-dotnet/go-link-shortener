package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestOriginIsolationAddsRequiredWebMCPHeaders(t *testing.T) {
	app := fiber.New()
	app.Use(OriginIsolation(""))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer response.Body.Close()

	if got := response.Header.Get("Cross-Origin-Opener-Policy"); got != "same-origin" {
		t.Fatalf("Cross-Origin-Opener-Policy = %q, want same-origin", got)
	}
	if got := response.Header.Get("Cross-Origin-Embedder-Policy"); got != "require-corp" {
		t.Fatalf("Cross-Origin-Embedder-Policy = %q, want require-corp", got)
	}
}

func TestOriginIsolationAddsOriginTrialHeaderWhenTokenIsConfigured(t *testing.T) {
	app := fiber.New()
	app.Use(OriginIsolation("origin-trial-token"))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer response.Body.Close()

	if got := response.Header.Get("Origin-Trial"); got != "origin-trial-token" {
		t.Fatalf("Origin-Trial = %q, want origin-trial-token", got)
	}
}
