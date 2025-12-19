package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// handleSitemap generate sitemap.xml including all published articles
func (s *Server) handleSitemap(c *gin.Context) {
	// Get all published articles
	articles, err := s.database.GetPublishedArticles(1000, 0) // Get up to 1000 articles
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate sitemap")
		return
	}

	// Base URL from environment or default
	baseURL := "https://aitrading247.com"
	if url := c.GetHeader("X-Forwarded-Host"); url != "" {
		scheme := "https"
		if c.GetHeader("X-Forwarded-Proto") == "http" {
			scheme = "http"
		}
		baseURL = fmt.Sprintf("%s://%s", scheme, url)
	}

	// Generate sitemap XML
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>` + baseURL + `</loc>
    <lastmod>` + time.Now().Format("2006-01-02") + `</lastmod>
    <changefreq>daily</changefreq>
    <priority>1.0</priority>
  </url>
  <url>
    <loc>` + baseURL + `/blog</loc>
    <lastmod>` + time.Now().Format("2006-01-02") + `</lastmod>
    <changefreq>daily</changefreq>
    <priority>0.8</priority>
  </url>
  <url>
    <loc>` + baseURL + `/traders</loc>
    <lastmod>` + time.Now().Format("2006-01-02") + `</lastmod>
    <changefreq>daily</changefreq>
    <priority>0.8</priority>
  </url>
  <url>
    <loc>` + baseURL + `/competition</loc>
    <lastmod>` + time.Now().Format("2006-01-02") + `</lastmod>
    <changefreq>daily</changefreq>
    <priority>0.7</priority>
  </url>
  <url>
    <loc>` + baseURL + `/features</loc>
    <lastmod>` + time.Now().Format("2006-01-02") + `</lastmod>
    <changefreq>monthly</changefreq>
    <priority>0.6</priority>
  </url>
  <url>
    <loc>` + baseURL + `/about</loc>
    <lastmod>` + time.Now().Format("2006-01-02") + `</lastmod>
    <changefreq>monthly</changefreq>
    <priority>0.6</priority>
  </url>
  <url>
    <loc>` + baseURL + `/faq</loc>
    <lastmod>` + time.Now().Format("2006-01-02") + `</lastmod>
    <changefreq>monthly</changefreq>
    <priority>0.6</priority>
  </url>`

	// Add article URLs
	for _, article := range articles {
		if article.PublishedAt != nil {
			// Use updated_at if available, otherwise use published_at
			var lastmod string
			if !article.UpdatedAt.IsZero() {
				lastmod = article.UpdatedAt.Format("2006-01-02")
			} else {
				lastmod = article.PublishedAt.Format("2006-01-02")
			}
			xml += fmt.Sprintf(`
  <url>
    <loc>%s/blog/%s</loc>
    <lastmod>%s</lastmod>
    <changefreq>weekly</changefreq>
    <priority>0.7</priority>
  </url>`, baseURL, article.Slug, lastmod)
		}
	}

	xml += `
</urlset>`

	c.Header("Content-Type", "application/xml")
	c.String(http.StatusOK, xml)
}
