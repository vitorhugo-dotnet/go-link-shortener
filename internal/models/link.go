package models

import "time"

type ClickEvent struct {
	Timestamp time.Time `json:"timestamp"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Referrer  string    `json:"referrer"`
}

type Link struct {
	ID          string       `json:"id"`
	Slug        string       `json:"slug"`
	// max=2048 mirrors the practical URL length limit accepted by major browsers
	// and search engines; PostgreSQL TEXT has no length cap on its own.
	OriginalURL string       `json:"original_url" validate:"required,url,max=2048"`
	Metadata    []byte       `json:"metadata"`
	Analytics   []ClickEvent `json:"analytics"`
	CreatedAt   time.Time    `json:"created_at"`
}

// CreateLinkRequest is the DTO used when creating a new short link.
// The URL length ceiling of 2048 characters aligns with the Link model
// constraint and guards against oversized payloads before any DB round-trip.
type CreateLinkRequest struct {
	Slug        string `json:"slug"         validate:"required"`
	OriginalURL string `json:"original_url" validate:"required,url,max=2048"`
}
