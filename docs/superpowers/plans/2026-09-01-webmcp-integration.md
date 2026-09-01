# WebMCP Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose URL creation, resolution, and aggregate traffic as WebMCP tools while preserving the Fiber UI.

**Architecture:** Annotate the existing form for declarative WebMCP. Add two JSON read endpoints backed by Redis/PostgreSQL, then conditionally register two imperative browser tools.

**Tech Stack:** Go, Fiber v3, pgx/v5, Redis, PostgreSQL JSONB, JavaScript.

**Spec:** `docs/superpowers/specs/2026-09-01-webmcp-design.md`

## Global Constraints

- Do not add dependencies, frontend frameworks, a build step, deletion, authentication, or a remote MCP server.
- Keep `CreateLinkForm` as the only creation validation and persistence path.
- Use `create_short_link`, `resolve_short_link`, and `get_link_analytics`; imperative tools set `annotations.readOnlyHint: true`.
- Never return IP, User-Agent, referrer, or individual analytics events.
- Feature-detect `document.modelContext`; normal browser behavior must work without it.

---

## File Structure

- `main.go`: API and script routes before `/:slug`.
- `internal/database/postgres.go`: link read model and aggregate analytics query.
- `internal/handlers/api.go`: JSON read handlers.
- `internal/handlers/link.go`: form metadata and script tag.
- `web/webmcp.js`: tool registration and fetch errors.
- `internal/database/postgres_test.go`, `internal/handlers/api_test.go`, `internal/handlers/link_test.go`: focused coverage.
- `README.md`: Chrome verification instructions.

### Task 1: Database lookup and aggregate models

**Files:** Modify `internal/database/postgres.go`; test `internal/database/postgres_test.go`.

**Interfaces:** Produce `LinkDetails { Slug string; OriginalURL string }`, `LinkAnalytics { TotalClicks int; LastClickedAt *time.Time }`, `GetLinkDetails(*pgxpool.Pool, string)`, and `GetLinkAnalytics(*pgxpool.Pool, string)`.

- [ ] Write a failing decoder test:

```go
func TestDecodeAnalyticsSummary(t *testing.T) {
  got, err := decodeAnalyticsSummary([]byte(`[{"timestamp":"2026-09-01T14:20:00Z"},{"timestamp":"2026-09-02T10:00:00Z"}]`))
  if err != nil || got.TotalClicks != 2 || got.LastClickedAt.Format(time.RFC3339) != "2026-09-02T10:00:00Z" { t.Fatal(got, err) }
}
```

- [ ] Run `go test ./internal/database -run TestDecodeAnalyticsSummary -count=1`; expect failure because the decoder is missing.
- [ ] Implement `GetLinkDetails` using `SELECT slug, original_url FROM links WHERE slug = $1`; query the matching JSONB `analytics`, decode timestamps, return zero/nil for `[]`, and preserve `pgx.ErrNoRows`.
- [ ] Run `go test ./internal/database -run 'TestDecodeAnalyticsSummary|TestDecodeAnalyticsSummaryEmpty' -count=1`; expect pass.
- [ ] Commit database changes with `feat: add link lookup and analytics summaries`.

### Task 2: Read-only JSON APIs

**Files:** Create `internal/handlers/api.go`; modify `main.go`; test `internal/handlers/api_test.go`.

**Interfaces:** Consume the task-one database functions and Redis helpers. Produce `Handler.GetLink(fiber.Ctx)` and `Handler.GetLinkAnalytics(fiber.Ctx)`.

- [ ] Write failing endpoint tests for a JSON lookup response containing `alias`, `shortUrl`, `targetUrl`; analytics containing `totalClicks`, `lastClickedAt`; zero-click behavior; and `GET /api/links/missing` returning 404.
- [ ] Run `go test ./internal/handlers -run 'TestGetLink|TestGetLinkAnalytics' -count=1`; expect failure because handlers and routes are missing.
- [ ] Add `/api/links/:alias` and `/api/links/:alias/analytics` before slug routes. Lookup checks Redis, falls back to PostgreSQL, refills Redis, and produces JSON. Both endpoints return `{"error":"link not found"}` with 404 for `pgx.ErrNoRows` and a JSON 500 for database failures.
- [ ] Run `go test ./internal/handlers -run 'TestGetLink|TestGetLinkAnalytics' -count=1`; expect pass.
- [ ] Commit API changes with `feat: expose link lookup and analytics APIs`.

### Task 3: Declarative and imperative tools

**Files:** Create `web/webmcp.js`; modify `main.go` and `internal/handlers/link.go`; test `internal/handlers/link_test.go`.

**Interfaces:** Consume both API URLs. Produce browser registrations of `resolve_short_link` and `get_link_analytics`.

- [ ] Write a failing home-page test asserting `toolname="create_short_link"`, `tooldescription`, `toolautosubmit`, and `/web/webmcp.js`; test the script text for `document.modelContext`, both tool names, `inputSchema`, both API paths, and `readOnlyHint: true`.
- [ ] Run `go test ./internal/handlers -run 'TestShowFormIncludesWebMCPMetadata|TestWebMCPScriptRegistersReadOnlyTools' -count=1`; expect failure.
- [ ] Annotate the current form with `toolautosubmit`, the exact creation tool name and description, and input `toolparamdescription` limits. Serve `/web/webmcp.js` before slug routes and link it with `defer`.
- [ ] In the script, wait for document readiness, return if `document.modelContext.registerTool` is unavailable, and register each read tool with `{type:'object',properties:{alias:{type:'string'}},required:['alias']}`, `annotations:{readOnlyHint:true}`, URL-encoded `fetch`, readable HTTP error extraction, and `JSON.stringify` results.
- [ ] Re-run the task-three test command; expect pass.
- [ ] Commit with `feat: register WebMCP link tools`.

### Task 4: Documentation and verification

**Files:** Modify `README.md`.

- [ ] Document enabling `chrome://flags/#enable-webmcp-testing`, inspecting the three tools with Model Context Tool Inspector, and the create/resolve/traffic demo flow. State origin isolation and progressive enhancement requirements.
- [ ] Run `go test ./internal/database ./internal/handlers -count=1` and `go vet ./...`; expect no failures.
- [ ] Run `git diff --check`; expect no whitespace errors.
- [ ] Commit with `docs: explain WebMCP testing`.

## Plan Self-Review

- Tasks 1–2 cover cache-aware lookup, aggregate-only analytics, and missing aliases.
- Task 3 covers all three tool contracts and unsupported browsers.
- Task 4 covers testing guidance and final verification.
- Names and signatures are consistent across tasks; no dependencies or unrelated refactors are required.
