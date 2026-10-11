package crawler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"gongkao/internal/annxlsx"
	"gongkao/internal/config"
	"gongkao/internal/model"
)

type listPage struct {
	name string
	url  string
}

type memoryAnnouncements struct {
	rows []model.Announcement
	seen map[string]struct{}
}

func (m *memoryAnnouncements) UpsertFromCrawl(a *model.Announcement) error {
	if a == nil || strings.TrimSpace(a.Title) == "" {
		return nil
	}
	if a.Source == "seed" || strings.HasPrefix(a.Source, "seed") {
		return nil
	}
	key := a.Source + "\x00" + a.SourceID
	if a.SourceID == "" {
		key = a.SourceURL
	}
	if m.seen == nil {
		m.seen = map[string]struct{}{}
	}
	if _, ok := m.seen[key]; ok {
		return nil
	}
	m.seen[key] = struct{}{}
	cp := *a
	m.rows = append(m.rows, cp)
	return nil
}

// WriteAnnouncementWorkbook crawls public 公考 pages and writes the Excel workbook.
// An empty result does not replace an existing file.
func WriteAnnouncementWorkbook(ctx context.Context, cfg *config.Config) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	rows, err := CollectAnnouncements(ctx, cfg)
	if err != nil && len(rows) == 0 {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("no announcements collected")
	}
	path := "data/announcements.xlsx"
	if cfg != nil && cfg.AnnouncementXLSX != "" {
		path = cfg.AnnouncementXLSX
	}
	if err := annxlsx.Write(path, rows); err != nil {
		return 0, err
	}
	slog.Info("announcement workbook written", "path", path, "rows", len(rows))
	return len(rows), nil
}

// CollectAnnouncements pulls live announcement rows and does not write the database.
func CollectAnnouncements(ctx context.Context, cfg *config.Config) ([]model.Announcement, error) {
	mem := &memoryAnnouncements{}
	client := defaultHTTPClient()
	fast := &http.Client{Timeout: 8 * time.Second}
	detail := &http.Client{Timeout: 12 * time.Second}
	r := &Runner{ann: mem, client: client}
	sources := []Source{
		&cptaSource{runner: r, url: "http://www.cpta.com.cn/", client: fast},
		&govZhengceSource{
			runner: r,
			url:    "https://www.gov.cn/zhengce/zuixin/ZUIXINZHENGCE.json",
			client: client,
			limit:  40,
		},
	}
	for _, page := range announcementListPages(cfg) {
		sources = append(sources, &htmlListSource{
			runner: r,
			name:   page.name,
			url:    page.url,
			client: detail,
			limit:  20,
		})
	}
	var first error
	for _, src := range sources {
		if err := src.Fetch(ctx); err != nil {
			slog.Error("announcement source failed", "source", src.Name(), "err", err)
			if first == nil {
				first = err
			}
		}
	}
	return mem.rows, first
}

func announcementListPages(cfg *config.Config) []listPage {
	raw := ""
	if cfg != nil {
		raw = cfg.CrawlerAnnouncementListURLs
	}
	if strings.TrimSpace(raw) == "" {
		return []listPage{{
			name: "hubei-rsks",
			url:  "https://rst.hubei.gov.cn/hbrsksw/sy.shtml",
		}}
	}
	var out []listPage
	for _, part := range splitCSV(raw) {
		name, pageURL := "list", part
		if i := strings.Index(part, "|"); i > 0 {
			name = strings.TrimSpace(part[:i])
			pageURL = strings.TrimSpace(part[i+1:])
		}
		if pageURL == "" {
			continue
		}
		out = append(out, listPage{name: name, url: pageURL})
	}
	return out
}
