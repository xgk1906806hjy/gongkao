package crawler

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"gongkao/internal/model"
)

// cptaSource crawls 中国人事考试网 homepage for exam notices and current affairs.
// https://www.cpta.com.cn/
type cptaSource struct {
	runner *Runner
	url    string
	client *http.Client
}

func (s *cptaSource) Name() string { return "cpta" }

var (
	reAnchorTitle = regexp.MustCompile(`(?is)<a[^>]+href=["']([^"']+)["'][^>]*title=["']([^"']+)["'][^>]*>`)
	reAnchorText  = regexp.MustCompile(`(?is)<a[^>]+href=["']([^"']+)["'][^>]*>([^<]{6,120})</a>`)
)

type cptaLink struct {
	href  string
	title string
}

func (s *cptaSource) Fetch(ctx context.Context) error {
	base := s.url
	if base == "" {
		base = "http://www.cpta.com.cn/"
	}
	html, err := fetchText(ctx, s.client, base)
	if err != nil {
		return err
	}
	links := extractCPTAlinks(html, base)
	annN, affN := 0, 0
	seen := map[string]bool{}
	for _, lk := range links {
		if seen[lk.href] {
			continue
		}
		seen[lk.href] = true
		path := strings.ToLower(lk.href)
		switch {
		case strings.Contains(path, "/notice/") || strings.Contains(path, "/examnotice/") ||
			strings.Contains(path, "/2026/") || strings.Contains(path, "/2025/") ||
			strings.Contains(path, "/2024/"):
			if err := s.upsertAnnouncement(lk); err != nil {
				_ = s.runner.recordFailure(s.Name(), lk.href, err)
				continue
			}
			annN++
		case strings.Contains(path, "/currentaffairs/"):
			if err := s.upsertAffair(lk); err != nil {
				_ = s.runner.recordFailure(s.Name(), lk.href, err)
				continue
			}
			affN++
		default:
			// keyword fallback
			if isExamRelatedTitle(lk.title) {
				if err := s.upsertAnnouncement(lk); err == nil {
					annN++
				}
			}
		}
	}
	slog.Info("cpta crawl done", "announcements", annN, "affairs", affN, "links", len(links))
	return nil
}

func extractCPTAlinks(html, base string) []cptaLink {
	out := []cptaLink{}
	add := func(href, title string) {
		href = absURL(base, href)
		title = strings.TrimSpace(stripTags(title))
		if href == "" || title == "" {
			return
		}
		if !strings.Contains(href, "cpta.com.cn") {
			return
		}
		out = append(out, cptaLink{href: href, title: title})
	}
	for _, m := range reAnchorTitle.FindAllStringSubmatch(html, -1) {
		add(m[1], m[2])
	}
	for _, m := range reAnchorText.FindAllStringSubmatch(html, -1) {
		add(m[1], m[2])
	}
	return out
}

func (s *cptaSource) upsertAnnouncement(lk cptaLink) error {
	ann := &model.Announcement{
		Type:        classifyAnnouncementType(lk.title),
		Title:       lk.title,
		Region:      guessRegion(lk.title),
		Source:      s.Name(),
		SourceID:    shortHash(lk.href),
		SourceURL:   lk.href,
		Summary:     truncateRunes(lk.title, 160),
		Content:     "来源：中国人事考试网。详情请查看原文链接。",
		PublishedAt: time.Now(),
		Status:      model.StatusPublished,
	}
	return s.runner.ann.UpsertFromCrawl(ann)
}

func (s *cptaSource) upsertAffair(lk cptaLink) error {
	art := &model.AffairsArticle{
		Title:       lk.title,
		Source:      s.Name(),
		SourceURL:   lk.href,
		Summary:     truncateRunes(lk.title, 160),
		Content:     "来源：中国人事考试网时政栏目。详情请查看原文链接。",
		Tags:        "人事考试,时政",
		PublishedAt: time.Now(),
		Status:      model.StatusPublished,
	}
	return s.runner.aff.UpsertFromCrawl(art)
}
