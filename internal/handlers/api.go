package handlers

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
)

type linkResponse struct {
	Alias     string `json:"alias"`
	ShortURL  string `json:"shortUrl"`
	TargetURL string `json:"targetUrl"`
}

type analyticsResponse struct {
	Alias         string `json:"alias"`
	TotalClicks   int    `json:"totalClicks"`
	LastClickedAt any    `json:"lastClickedAt"`
}

func (h *Handler) GetLink(c fiber.Ctx) error {
	link, err := h.lookupLink(c.Params("alias"))
	if err != nil {
		return apiError(c, err)
	}
	return c.JSON(linkResponse{
		Alias:     link.Slug,
		ShortURL:  fmt.Sprintf("%s://%s/%s", c.Scheme(), h.cfg.AppHost, link.Slug),
		TargetURL: link.OriginalURL,
	})
}

func (h *Handler) GetLinkAnalytics(c fiber.Ctx) error {
	analytics, err := h.lookupAnalytics(c.Params("alias"))
	if err != nil {
		return apiError(c, err)
	}
	return c.JSON(analyticsResponse{
		Alias:         c.Params("alias"),
		TotalClicks:   analytics.TotalClicks,
		LastClickedAt: analytics.LastClickedAt,
	})
}

func apiError(c fiber.Ctx, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "link not found"})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
}
