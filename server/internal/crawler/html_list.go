package crawler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"gongkao/internal/model"
)

// htmlListSource reads a public HTML notice index and keeps 公考 announcements.
type htmlListSource struct {
	runner *Runner
	name   string
	url    string
	client *http.Client
	limit  int
}

func (s *htmlListSource) Name() string {
	if s.name != "" {
		return s.name
	}
	return "html-list"
}

func (s *htmlListSource) Fetch(ctx context.Context) error {
	html, err := fetchText(ctx, s.client, s.url)
	if err != nil {
		return err
	}
	links := extractAnnouncementLinks(html, s.url)
	n := 0
	for _, lk := range links {
		if s.limit > 0 && n >= s.limit {
			break
		}
		ann := s.build(ctx, lk)
		if err := s.runner.ann.UpsertFromCrawl(ann); err != nil {
			_ = s.runner.recordFailure(s.Name(), lk.href, err)
			continue
		}
		n++
	}
	slog.Info("html list crawl done", "source", s.Name(), "announcements", n, "links", len(links))
	return nil
}

func extractAnnouncementLinks(html, base string) []cptaLink {
	seen := map[string]bool{}
	var out []cptaLink
	add := func(href, title string) {
		title = strings.TrimSpace(stripTags(title))
		href = absURL(base, href)
		if href == "" || title == "" || seen[href] {
			return
		}
		if !looksLikeDocLink(href) || !isGongkaoAnnouncementTitle(title) {
			return
		}
		seen[href] = true
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

func looksLikeDocLink(href string) bool {
	h := strings.ToLower(strings.TrimSpace(href))
	if h == "" || strings.HasPrefix(h, "javascript:") || strings.HasSuffix(h, "/") {
		return false
	}
	return strings.Contains(h, ".shtml") || strings.Contains(h, ".html") || strings.Contains(h, ".htm")
}

func isGongkaoAnnouncementTitle(title string) bool {
	return isExamRelatedTitle(title) && containsAny(title, "公告", "公示", "简章", "通知", "提示")
}

func (s *htmlListSource) build(ctx context.Context, lk cptaLink) *model.Announcement {
	title, pub := splitTrailingDate(lk.title, time.Now())
	if pub.IsZero() {
		if t := parseURLDate(lk.href); t != nil {
			pub = *t
		}
	}
	summary := title
	content := "详情请查看原文链接。"
	var tl timeline
	if body, err := fetchText(ctx, s.client, lk.href); err == nil {
		text := stripTags(body)
		if t := parsePublishedAt(text); t != nil {
			pub = *t
		}
		tl = extractTimeline(text)
		excerpt := truncateRunes(text, 500)
		if excerpt != "" {
			summary = truncateRunes(excerpt, 180)
			content = excerpt
		}
	} else {
		slog.Warn("announcement detail skipped", "source", s.Name(), "url", lk.href, "err", err)
	}
	if pub.IsZero() {
		pub = time.Now()
	}
	return &model.Announcement{
		Type:          classifyAnnouncementType(title),
		Title:         title,
		Region:        guessRegion(title),
		Source:        s.Name(),
		SourceID:      shortHash(lk.href),
		SourceURL:     lk.href,
		Summary:       summary,
		Content:       content,
		RegisterStart: tl.RegisterStart,
		RegisterEnd:   tl.RegisterEnd,
		ConfirmStart:  tl.ConfirmStart,
		ConfirmEnd:    tl.ConfirmEnd,
		ExamAt:        tl.ExamAt,
		PublishedAt:   pub,
		Status:        model.StatusPublished,
	}
}
