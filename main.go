package main

import (
	_ "embed"
	"log"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/vitorhugo-java/go-link-shortener/internal/config"
	"github.com/vitorhugo-java/go-link-shortener/internal/database"
	"github.com/vitorhugo-java/go-link-shortener/internal/handlers"
)

//go:embed web/webmcp.js
var webMCPScript string

func main() {
	cfg := config.Load()

	pg, err := database.NewPostgres(cfg)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer pg.Close()

	rdb := database.NewRedis(cfg)
	defer rdb.Close()

	if err := database.Migrate(pg); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	h := handlers.New(pg, rdb, cfg)

	app := fiber.New(fiber.Config{
		AppName: "go-link-shortener",
	})

	app.Use(limiter.New(limiter.Config{
		Max:        100,
		Expiration: time.Minute,
	}))

	formLimiter := limiter.New(limiter.Config{
		Max:        2,
		Expiration: 10 * time.Second,
	})

	app.Get("/", h.ShowForm)
	app.Post("/", formLimiter, h.CreateLinkForm)
	app.Get("/api/links/:alias/analytics", h.GetLinkAnalytics)
	app.Get("/api/links/:alias", h.GetLink)
	app.Get("/web/webmcp.js", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, "application/javascript; charset=utf-8")
		return c.SendString(webMCPScript)
	})
	app.Get("/:slug", h.RedirectLink)
	app.Get("/:slug/*", h.CreateLink)

	log.Fatal(app.Listen(":" + cfg.Port))
}
