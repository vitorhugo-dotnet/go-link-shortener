package handlers

import (
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/vitorhugo-java/go-link-shortener/internal/config"
	"github.com/vitorhugo-java/go-link-shortener/internal/database"
	"github.com/vitorhugo-java/go-link-shortener/internal/models"
)

var aliasRe = regexp.MustCompile(`^[a-zA-Z0-9\-]+$`)

var validate = validator.New()

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

const formTemplate = `<!DOCTYPE html>
<html lang="pt-BR">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Encurtador de Links</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{background:#0d1117;color:#c9d1d9;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{padding:2.5rem;background:#161b22;border:1px solid #30363d;border-radius:12px;width:90%%;max-width:480px}
h1{font-size:1.6rem;color:#58a6ff;margin-bottom:1.8rem;text-align:center}
label{display:block;font-size:.85rem;color:#8b949e;margin-bottom:.35rem}
input{width:100%%;background:#0d1117;border:1px solid #30363d;border-radius:6px;padding:.65rem .9rem;color:#e6edf3;font-size:.95rem;margin-bottom:1.1rem;outline:none}
input:focus{border-color:#58a6ff}
button{width:100%%;background:#238636;color:#fff;border:none;padding:.7rem;border-radius:6px;font-size:1rem;cursor:pointer;transition:background .2s}
button:hover{background:#2ea043}
.result{background:#0d1117;border:1px solid #238636;border-radius:8px;padding:1rem 1.2rem;margin-bottom:1.2rem}
.result p{font-size:.8rem;color:#8b949e;margin-bottom:.4rem}
.result .link{word-break:break-all;color:#58a6ff;font-size:.95rem;margin-bottom:.8rem}
.copy-btn{width:auto;background:#238636;padding:.45rem 1.2rem;font-size:.9rem}
.copy-btn.copied{background:#388bfd}
.error{background:#3d1a1a;border:1px solid #f85149;border-radius:8px;padding:.8rem 1rem;color:#f85149;font-size:.9rem;margin-bottom:1.2rem}
</style>
</head>
<body>
<div class="card">
  <h1>&#128279; Encurtador de Links</h1>
  %s
  <form method="POST" action="/" toolautosubmit toolname="create_short_link" tooldescription="Create a short URL with a user-provided alias that redirects to an HTTP or HTTPS URL.">
    <label for="url">URL a encurtar</label>
    <input id="url" name="url" type="text" placeholder="https://exemplo.com/link-longo" value="%s" maxlength="2048" required toolparamdescription="Target HTTP or HTTPS URL, required, maximum 2048 characters.">
    <label for="alias">Alias desejado</label>
    <input id="alias" name="alias" type="text" placeholder="meu-link" value="%s" maxlength="100" required toolparamdescription="Alias using only letters, numbers, and hyphens; required, maximum 100 characters.">
    <button type="submit">Encurtar</button>
  </form>
</div>
<script src="/web/webmcp.js" defer></script>
</body>
</html>`

type Handler struct {
	pg              *pgxpool.Pool
	rdb             *redis.Client
	cfg             *config.Config
	lookupLink      func(string) (database.LinkDetails, error)
	lookupAnalytics func(string) (database.LinkAnalytics, error)
}

func New(pg *pgxpool.Pool, rdb *redis.Client, cfg *config.Config) *Handler {
	h := &Handler{pg: pg, rdb: rdb, cfg: cfg}
	h.lookupLink = func(alias string) (database.LinkDetails, error) {
		if targetURL, err := database.CacheGet(h.rdb, alias); err == nil && targetURL != "" {
			return database.LinkDetails{Slug: alias, OriginalURL: targetURL}, nil
		}
		link, err := database.GetLinkDetails(h.pg, alias)
		if err == nil {
			_ = database.CacheSet(h.rdb, link.Slug, link.OriginalURL)
		}
		return link, err
	}
	h.lookupAnalytics = func(alias string) (database.LinkAnalytics, error) {
		return database.GetLinkAnalytics(h.pg, alias)
	}
	return h
}

func (h *Handler) ShowForm(c fiber.Ctx) error {
	var infoBlock string

	if shortURL := c.Query("created"); shortURL != "" {
		escaped := html.EscapeString(shortURL)
		infoBlock = fmt.Sprintf(`<div class="result"><p>Link gerado com sucesso!</p><div class="link" id="sl">%s</div><button class="copy-btn" id="cb" onclick="copyLink()">Copiar</button></div><script>function copyLink(){navigator.clipboard.writeText(document.getElementById('sl').textContent).then(function(){var b=document.getElementById('cb');b.textContent='Copiado!';b.classList.add('copied');setTimeout(function(){b.textContent='Copiar';b.classList.remove('copied');},2000);});}</script>`, escaped)
	} else if errMsg := c.Query("error"); errMsg != "" {
		infoBlock = fmt.Sprintf(`<div class="error">%s</div>`, html.EscapeString(errMsg))
	}

	body := fmt.Sprintf(formTemplate, infoBlock, "", "")
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return c.SendString(body)
}

func (h *Handler) CreateLinkForm(c fiber.Ctx) error {
	rawURL := strings.TrimSpace(c.FormValue("url"))
	alias := strings.TrimSpace(c.FormValue("alias"))

	renderError := func(msg string) error {
		body := fmt.Sprintf(formTemplate,
			fmt.Sprintf(`<div class="error">%s</div>`, html.EscapeString(msg)),
			html.EscapeString(rawURL),
			html.EscapeString(alias),
		)
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Status(fiber.StatusUnprocessableEntity).SendString(body)
	}

	if rawURL == "" || alias == "" {
		return renderError("URL e alias são obrigatórios.")
	}

	if len(alias) > 100 || !aliasRe.MatchString(alias) {
		return renderError("Alias inválido: use apenas letras, números e hífens (máx. 100 caracteres).")
	}

	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return renderError("URL inválida. Informe uma URL com http:// ou https://.")
	}
	if len(rawURL) > 2048 {
		return renderError("URL muito longa (máximo 2048 caracteres).")
	}

	// Check for duplicate alias before saving.
	existing, err := database.GetLinkURL(h.pg, alias)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return renderError("Erro interno ao verificar alias.")
	}
	if existing != "" {
		return renderError("Esse alias já está em uso. Escolha outro.")
	}

	if err := database.SaveLink(h.pg, alias, rawURL); err != nil {
		return renderError("Erro ao salvar o link. Tente novamente.")
	}
	_ = database.CacheSet(h.rdb, alias, rawURL)

	shortURL := fmt.Sprintf("%s://%s/%s", c.Scheme(), h.cfg.AppHost, alias)
	return c.Redirect().To("/?created=" + url.QueryEscape(shortURL))
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

	// Validate the fully assembled URL against the CreateLinkRequest constraints.
	// This covers both structural validity and the 2048-character ceiling that
	// guards against oversized query strings or deeply-nested path segments that
	// only become apparent after fragment re-assembly in phase 2.
	req := models.CreateLinkRequest{Slug: slug, OriginalURL: targetURL}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("invalid request: " + err.Error())
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
