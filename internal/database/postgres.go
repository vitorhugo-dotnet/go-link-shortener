package database

import (
	"context"
	_ "embed"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vitorhugo-java/go-link-shortener/internal/config"
	"github.com/vitorhugo-java/go-link-shortener/internal/models"
)

type LinkDetails struct {
	Slug        string
	OriginalURL string
}

type LinkAnalytics struct {
	TotalClicks   int
	LastClickedAt *time.Time
}

//go:embed migrations/001_init.sql
var migrationSQL string

func NewPostgres(cfg *config.Config) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DBURL)
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func Migrate(pool *pgxpool.Pool) error {
	_, err := pool.Exec(context.Background(), migrationSQL)
	return err
}

func SaveLink(pool *pgxpool.Pool, slug, originalURL string) error {
	_, err := pool.Exec(
		context.Background(),
		`INSERT INTO links (slug, original_url) VALUES ($1, $2)
         ON CONFLICT (slug) DO UPDATE SET original_url = EXCLUDED.original_url`,
		slug, originalURL,
	)
	return err
}

func GetLinkURL(pool *pgxpool.Pool, slug string) (string, error) {
	var originalURL string
	err := pool.QueryRow(
		context.Background(),
		`SELECT original_url FROM links WHERE slug = $1`,
		slug,
	).Scan(&originalURL)
	return originalURL, err
}

func GetLinkDetails(pool *pgxpool.Pool, slug string) (LinkDetails, error) {
	var link LinkDetails
	err := pool.QueryRow(
		context.Background(),
		`SELECT slug, original_url FROM links WHERE slug = $1`,
		slug,
	).Scan(&link.Slug, &link.OriginalURL)
	return link, err
}

func GetLinkAnalytics(pool *pgxpool.Pool, slug string) (LinkAnalytics, error) {
	var data []byte
	err := pool.QueryRow(
		context.Background(),
		`SELECT analytics FROM links WHERE slug = $1`,
		slug,
	).Scan(&data)
	if err != nil {
		return LinkAnalytics{}, err
	}
	return decodeAnalyticsSummary(data)
}

func decodeAnalyticsSummary(data []byte) (LinkAnalytics, error) {
	var events []struct {
		Timestamp time.Time `json:"timestamp"`
	}
	if err := json.Unmarshal(data, &events); err != nil {
		return LinkAnalytics{}, err
	}

	summary := LinkAnalytics{TotalClicks: len(events)}
	for _, event := range events {
		if summary.LastClickedAt == nil || event.Timestamp.After(*summary.LastClickedAt) {
			timestamp := event.Timestamp
			summary.LastClickedAt = &timestamp
		}
	}
	return summary, nil
}

func AppendClickEvent(pool *pgxpool.Pool, slug string, event models.ClickEvent) {
	data, err := json.Marshal([]models.ClickEvent{event})
	if err != nil {
		return
	}
	pool.Exec(
		context.Background(),
		`UPDATE links SET analytics = analytics || $1::jsonb WHERE slug = $2`,
		string(data), slug,
	)
}
