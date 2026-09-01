package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/vitorhugo-java/go-link-shortener/internal/config"
)

func TestShowFormExposesCreateShortLinkAsDeclarativeTool(t *testing.T) {
	app := fiber.New()
	app.Get("/", (&Handler{cfg: &config.Config{}}).ShowForm)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	for _, want := range []string{"toolname=\"create_short_link\"", "toolautosubmit", "tooldescription=\"Create a short URL", "toolparamdescription=\"Target HTTP or HTTPS URL", "maxlength=\"2048\"", "toolparamdescription=\"Alias using only letters, numbers, and hyphens"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("home page does not expose %q", want)
		}
	}
}
