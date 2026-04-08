package handler

import (
	"context"
	"encoding/xml"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// TopicEntry holds the minimal data needed to build a sitemap URL for a topic.
type TopicEntry struct {
	ID        string
	CreatedAt time.Time
}

// SitemapRepository is the data access interface for sitemap generation.
type SitemapRepository interface {
	GetAllTopicIDs(ctx context.Context) ([]TopicEntry, error)
}

// SitemapHandler generates dynamic sitemap.xml.
type SitemapHandler struct {
	repo        SitemapRepository
	frontendURL string
}

// NewSitemapHandler constructs a SitemapHandler.
func NewSitemapHandler(repo SitemapRepository, frontendURL string) *SitemapHandler {
	return &SitemapHandler{repo: repo, frontendURL: frontendURL}
}

// RegisterRoutes registers GET /sitemap.xml on the given router group.
func (h *SitemapHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/sitemap.xml", h.GetSitemap)
}

// xmlURL represents a single <url> entry in the sitemap.
type xmlURL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod"`
	ChangeFreq string `xml:"changefreq"`
	Priority   string `xml:"priority"`
}

// xmlURLSet is the root <urlset> element.
type xmlURLSet struct {
	XMLName xml.Name `xml:"urlset"`
	Xmlns   string   `xml:"xmlns,attr"`
	URLs    []xmlURL `xml:"url"`
}

// staticPage describes a static frontend page for the sitemap.
type staticPage struct {
	path       string
	changeFreq string
	priority   string
}

var staticPages = []staticPage{
	{"/", "daily", "1.0"},
	{"/latest", "daily", "0.9"},
	{"/allnews", "daily", "0.8"},
	{"/privacy-policy", "monthly", "0.3"},
	{"/terms-of-service", "monthly", "0.3"},
	{"/cookie-policy", "monthly", "0.3"},
	{"/about", "monthly", "0.5"},
}

// GetSitemap handles GET /api/v1/sitemap.xml
func (h *SitemapHandler) GetSitemap(c *gin.Context) {
	ctx := c.Request.Context()

	topics, err := h.repo.GetAllTopicIDs(ctx)
	if err != nil {
		slog.Error("sitemap: failed to query topic IDs", "error", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	today := time.Now().UTC().Format("2006-01-02")
	urls := make([]xmlURL, 0, len(staticPages)+len(topics))

	for _, p := range staticPages {
		urls = append(urls, xmlURL{
			Loc:        h.frontendURL + p.path,
			LastMod:    today,
			ChangeFreq: p.changeFreq,
			Priority:   p.priority,
		})
	}

	for _, t := range topics {
		urls = append(urls, xmlURL{
			Loc:        h.frontendURL + "/topic/" + t.ID,
			LastMod:    t.CreatedAt.UTC().Format("2006-01-02"),
			ChangeFreq: "weekly",
			Priority:   "0.7",
		})
	}

	urlSet := xmlURLSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}

	out, err := xml.MarshalIndent(urlSet, "", "  ")
	if err != nil {
		slog.Error("sitemap: failed to marshal XML", "error", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Data(http.StatusOK, "application/xml; charset=utf-8", append([]byte(xml.Header), out...))
}
