# go-link-shortener

A high-performance URL shortener written in Go using Fiber v3, PostgreSQL, and Redis.

## Features

- **Create short links** — `GET /:slug/<target-url>` stores a mapping and returns a dark-mode HTML page with a one-click "Copy Link" button.
- **Redirect** — `GET /:slug` looks up the slug (Redis cache → PostgreSQL fallback) and performs an HTTP redirect.
- **Analytics** — Every redirect appends a click event (timestamp, IP, User-Agent, Referrer) to the `analytics` JSONB column asynchronously.
- **Rate limiting** — 100 requests per minute per IP via Fiber's built-in limiter middleware.
- **JSONB** — Metadata and analytics stored in PostgreSQL JSONB columns for flexible querying.

## Quick Start (Docker Compose)

```sh
cp .env.example .env   # adjust values as needed
docker compose up --build
```

The service listens on port **3000** by default.

## Usage

### Shorten a link

```
GET http://localhost:3000/my-slug/google.com?q=golang
```

Returns a dark-mode HTML page displaying `http://localhost:3000/my-slug` with a **Copy Link** button.

### Follow a short link

```
GET http://localhost:3000/my-slug
```

Redirects to the stored target URL.

## WebMCP

The home page exposes the existing creation form as `create_short_link` for
WebMCP-enabled browsers. It also registers read-only `resolve_short_link` and
`get_link_analytics` tools. The read tools return the destination or aggregate
click count and latest click timestamp; they never expose visitor IP addresses,
User-Agent strings, referrers, or individual click events.

WebMCP is a progressive enhancement: browsers without it continue to use the
normal form and redirects. It requires an origin-isolated document.

### Test locally

1. In Chrome, enable `chrome://flags/#enable-webmcp-testing` and relaunch.
2. Start the application and open its home page in Chrome.
3. Use the Model Context Tool Inspector extension to confirm these tools are
   discoverable: `create_short_link`, `resolve_short_link`, and
   `get_link_analytics`.
4. Ask the agent to create `/webmcp` for
   `https://developer.chrome.com/docs/ai/webmcp`; it should use the visible
   form tool rather than clicking through the page.
5. Ask where `/webmcp` redirects, open the short URL a few times, then ask for
   traffic. The agent should use the two read-only tools and report aggregate
   metrics.

## Environment Variables

| Variable    | Description                                      | Default           |
|-------------|--------------------------------------------------|-------------------|
| `DB_URL`    | PostgreSQL connection string (pgx DSN format)    | —                 |
| `REDIS_URL` | Redis URL (`redis://host:port`)                  | —                 |
| `PORT`      | HTTP listen port                                 | `3000`            |
| `APP_HOST`  | Hostname used in the generated short link        | `localhost:<PORT>`|
| `WEBMCP_ORIGIN_TRIAL_TOKEN` | Optional WebMCP Origin Trial token, emitted as the `Origin-Trial` response header | — |

## Project Structure

```
.
├── main.go                          # Entry point, routes, middleware
├── internal/
│   ├── config/config.go             # Environment config
│   ├── database/
│   │   ├── postgres.go              # pgx/v5 pool, migrations, queries
│   │   ├── redis.go                 # go-redis/v9 client, cache helpers
│   │   └── migrations/001_init.sql  # Schema (embedded at compile time)
│   ├── handlers/link.go             # HTTP handlers
│   └── models/link.go              # Data models
├── migrations/001_init.sql          # SQL schema reference copy
├── Dockerfile                       # Multi-stage build (Go → Alpine)
└── docker-compose.yml               # App + PostgreSQL 17 + Redis 7
```
