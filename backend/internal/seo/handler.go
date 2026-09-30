package seo

import (
	"encoding/xml"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Handler struct {
	db     *pgxpool.Pool
	origin string
}

func New(db *pgxpool.Pool, origin string) *Handler {
	return &Handler{db: db, origin: strings.TrimSuffix(origin, "/")}
}

type entry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}
type sitemap struct {
	XMLName xml.Name `xml:"urlset"`
	XMLNS   string   `xml:"xmlns,attr"`
	URLs    []entry  `xml:"url"`
}

func (h *Handler) Sitemap(w http.ResponseWriter, r *http.Request) {
	site := sitemap{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	for _, path := range []string{"/", "/shop", "/blog", "/about", "/contact", "/faq", "/shipping", "/returns", "/privacy", "/terms"} {
		site.URLs = append(site.URLs, entry{Loc: h.origin + path})
	}
	rows, err := h.db.Query(r.Context(), `SELECT '/products/'||slug,updated_at FROM products UNION ALL SELECT '/blog/'||slug,updated_at FROM articles WHERE status='published' UNION ALL SELECT '/category/'||slug,updated_at FROM categories ORDER BY 1 LIMIT 45000`)
	if err != nil {
		http.Error(w, "unavailable", 503)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var path string
		var updated time.Time
		if err = rows.Scan(&path, &updated); err != nil {
			http.Error(w, "unavailable", 503)
			return
		}
		parts := strings.Split(path, "/")
		site.URLs = append(site.URLs, entry{Loc: h.origin + "/" + parts[1] + "/" + url.PathEscape(parts[2]), LastMod: updated.UTC().Format(time.RFC3339)})
	}
	if rows.Err() != nil {
		http.Error(w, "unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(site)
}
func (h *Handler) Robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("User-agent: *\nAllow: /\nDisallow: /admin\nDisallow: /account\nDisallow: /cart\nDisallow: /wishlist\nDisallow: /checkout\nDisallow: /orders\nDisallow: /login\nDisallow: /register\nDisallow: /forgot-password\nDisallow: /reset-password\nSitemap: " + h.origin + "/sitemap.xml\n"))
}
