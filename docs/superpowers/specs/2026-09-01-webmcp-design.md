# WebMCP integration design

## Scope

Expose the URL shortener's existing creation flow and two read-only capabilities
to WebMCP-enabled browsers. The normal Fiber server-rendered UI remains the
primary interface and works unchanged in browsers without WebMCP.

The MVP excludes deletion and does not introduce a frontend framework, remote
MCP server, authentication, or raw analytics event exposure.

## Architecture

The home form remains a `POST /` form handled by `CreateLinkForm`. WebMCP
declarative attributes expose it as `create_short_link`; `toolautosubmit`
submits the same form, so server-side validation and persistence remain the
only source of truth.

Two new read-only Fiber handlers provide JSON for the browser-side imperative
tools:

- `GET /api/links/:alias` returns the alias, generated short URL, and target
  URL, or a JSON 404.
- `GET /api/links/:alias/analytics` returns the alias, total click count, and
  most recent click timestamp, or a JSON 404.

The lookup handler follows the current Redis-then-PostgreSQL behavior. The
analytics handler queries PostgreSQL for aggregate data only; IP addresses,
User-Agent values, referrers, and individual events are never returned.

The existing template embeds a small script. Once the page loads, it checks
for `document.modelContext`; when available, it registers
`resolve_short_link` and `get_link_analytics`. Each tool has a lowercase,
action-oriented stable name, JSON Schema requiring `alias`, a descriptive
purpose, and `annotations.readOnlyHint: true`. The tool fetches its JSON
endpoint and returns a concise result; failed requests produce readable
errors. In unsupported browsers the script is inert.

## Data flow

1. An agent invokes `create_short_link`; Chrome fills and submits the visible
   form, and `CreateLinkForm` validates then saves the link as it already does.
2. An agent invokes `resolve_short_link`; the registered script fetches the
   lookup API and returns its normalized response.
3. A redirect continues to append an internal JSONB click event.
4. An agent invokes `get_link_analytics`; the script fetches an aggregate-only
   response computed from that JSONB data.

## Files and tests

Expected production changes are `main.go`, `internal/handlers/link.go`, a new
`internal/handlers/api.go`, `internal/database/postgres.go`, and `README.md`.
Tests will cover the WebMCP form metadata, valid and missing API lookups,
aggregate analytics including no clicks, and unchanged creation validation.

## Error handling and compatibility

JSON APIs return an explicit not-found result for unknown aliases. Imperative
tools surface HTTP failures with the endpoint error message. Existing global
and form rate limiters remain enabled. WebMCP registration is progressive
enhancement only and cannot introduce a JavaScript error where the API is not
implemented.
