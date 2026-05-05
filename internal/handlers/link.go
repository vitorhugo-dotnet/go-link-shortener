package handlers

import (
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/vitorhugo-java/go-link-shortener/internal/config"
	"github.com/vitorhugo-java/go-link-shortener/internal/database"
	"github.com/vitorhugo-java/go-link-shortener/internal/models"
)

// stagingTemplate is returned on the first visit to the creation URL.
// JavaScript reads window.location.hash (the URL fragment, which browsers never
// send to the server) and re-navigates with the fragment encoded as the "_f"
// query parameter, alongside "_c=1" to signal the confirmed/second request.
const stagingTemplate = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>Creating short link…</title></head>
<body>
<script>
(function(){
  var hash = window.location.hash;
  var qs   = window.location.search;
  var sep  = qs ? '&' : '?';
  var extra = '_c=1' + (hash ? '&_f=' + encodeURIComponent(hash.slice(1)) : '');
  window.location.replace(window.location.pathname + qs + sep + extra);
})();
</script>
<noscript>JavaScript is required to create short links. If your target URL contains a <code>#</code> fragment, please percent-encode it as <code>%23</code> before using this service.</noscript>
</body>
</html>`

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Link Shortened</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{background:#0d1117;color:#c9d1d9;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{text-align:center;padding:2.5rem;background:#161b22;border:1px solid #30363d;border-radius:12px;max-width:480px;width:90%%}
h1{font-size:1.6rem;color:#58a6ff;margin-bottom:1.2rem}
.link-box{background:#0d1117;border:1px solid #30363d;border-radius:8px;padding:.9rem 1.2rem;font-size:1rem;word-break:break-all;margin-bottom:1.2rem;color:#e6edf3}
button{background:#238636;color:#fff;border:none;padding:.65rem 1.6rem;border-radius:6px;font-size:.95rem;cursor:pointer;transition:background .2s}
button:hover{background:#2ea043}
.copied{background:#388bfd!important}
</style>
</head>
<body>
<div class="card">
<h1>&#128279; Link Shortened</h1>
<p class="link-box" id="sl">%s</p>
<button id="cb" onclick="copyLink()">Copy Link</button>
</div>
<script>
function copyLink(){
navigator.clipboard.writeText(document.getElementById('sl').textContent).then(function(){
var b=document.getElementById('cb');
b.textContent='Copied!';
b.classList.add('copied');
setTimeout(function(){b.textContent='Copy Link';b.classList.remove('copied');},2000);
});
}
</script>
</body>
</html>`

type Handler struct {
	pg  *pgxpool.Pool
	rdb *redis.Client
	cfg *config.Config
}

func New(pg *pgxpool.Pool, rdb *redis.Client, cfg *config.Config) *Handler {
	return &Handler{pg: pg, rdb: rdb, cfg: cfg}
}

func (h *Handler) CreateLink(c fiber.Ctx) error {
	slug := c.Params("slug")
	pathTail := c.Params("*")

	if slug == "" || pathTail == "" {
		return c.Status(fiber.StatusBadRequest).SendString("slug and target URL are required")
	}

	// Phase 1 – first visit: return a staging page so the browser's JavaScript
	// can capture window.location.hash (URL fragments are never sent to the
	// server by browsers) and re-submit as the "_f" query parameter.
	if c.Query("_c") != "1" {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.SendString(stagingTemplate)
	}

	// Phase 2 – confirmed request: build the full target URL.
	// Strip the internal "_c" and "_f" parameters before storing.
	rawQS := string(c.Request().URI().QueryString())
	qvals, err := url.ParseQuery(rawQS)
	if err != nil {
		qvals = url.Values{}
	}
	fragment := qvals.Get("_f")
	qvals.Del("_c")
	qvals.Del("_f")
	qs := qvals.Encode()

	targetURL := pathTail
	if qs != "" {
		targetURL = pathTail + "?" + qs
	}

	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}

	// Append the fragment captured by JavaScript (if any).
	if fragment != "" {
		targetURL += "#" + fragment
	}

	// url.Parse (instead of url.ParseRequestURI) correctly handles URLs that
	// contain a fragment component such as https://example.com/#section.
	parsed, err := url.Parse(targetURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return c.Status(fiber.StatusBadRequest).SendString("invalid target URL")
	}

	if err := database.SaveLink(h.pg, slug, targetURL); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("failed to save link")
	}

	_ = database.CacheSet(h.rdb, slug, targetURL)

	shortLink := fmt.Sprintf("%s://%s/%s", c.Scheme(), h.cfg.AppHost, html.EscapeString(slug))
	body := fmt.Sprintf(htmlTemplate, shortLink)
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return c.SendString(body)
}

func (h *Handler) RedirectLink(c fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return c.Status(fiber.StatusBadRequest).SendString("slug is required")
	}

	var targetURL string
	cacheHit := false

	cached, err := database.CacheGet(h.rdb, slug)
	if err == nil && cached != "" {
		targetURL = cached
		cacheHit = true
	}

	if !cacheHit {
		url, err := database.GetLinkURL(h.pg, slug)
		if err != nil {
			return c.Status(fiber.StatusNotFound).SendString("link not found")
		}
		targetURL = url
		_ = database.CacheSet(h.rdb, slug, targetURL)
	}

	event := models.ClickEvent{
		Timestamp: time.Now().UTC(),
		IP:        c.IP(),
		UserAgent: c.Get(fiber.HeaderUserAgent),
		Referrer:  c.Get(fiber.HeaderReferer),
	}
	go database.AppendClickEvent(h.pg, slug, event)

	return c.Redirect().To(targetURL)
}
