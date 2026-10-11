package crawler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"gongkao/internal/model"
)

// htmlListSource reads a public HTML notice index and keeps 公考 announcements.
type htmlListSource struct {
	runner   *Runner
	name     string
	url      string
	region   string
	client   *http.Client
	limit    int
	maxPages int
}

func (s *htmlListSource) Name() string {
	if s.name != "" {
		return s.name
	}
	return "html-list"
}

func (s *htmlListSource) Fetch(ctx context.Context) error {
	maxPages := s.maxPages
	if maxPages <= 0 {
		maxPages = 1
	}
	pageURL := s.url
	seenPage := map[string]bool{}
	var links []cptaLink
	for i := 0; i < maxPages; i++ {
		if pageURL == "" || seenPage[pageURL] {
			break
		}
		seenPage[pageURL] = true
		html, err := fetchTextRetry(ctx, s.client, pageURL, 2)
		if err != nil {
			if i == 0 {
				return err
			}
			slog.Warn("announcement list page skipped", "source", s.Name(), "url", pageURL, "err", err)
			break
		}
		links = append(links, extractAnnouncementLinks(html, pageURL)...)
		pageURL = nextListPage(html, pageURL)
	}
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

var (
	reAnchor    = regexp.MustCompile(`(?is)<a\b([^>]*)>(.*?)</a>`)
	reHrefAttr  = regexp.MustCompile(`(?is)\bhref\s*=\s*["']([^"']+)["']`)
	reTitleAttr = regexp.MustCompile(`(?is)\btitle\s*=\s*["']([^"']+)["']`)
	reListPage  = regexp.MustCompile(`createPageHTML\(\s*'[^']+'\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*'([^']+)'\s*,\s*'([^']+)'`)
)

func extractAnnouncementLinks(html, base string) []cptaLink {
	seen := map[string]bool{}
	var out []cptaLink
	for _, m := range reAnchor.FindAllStringSubmatch(html, -1) {
		hm := reHrefAttr.FindStringSubmatch(m[1])
		if hm == nil {
			continue
		}
		title := ""
		if tm := reTitleAttr.FindStringSubmatch(m[1]); tm != nil {
			title = tm[1]
		}
		if strings.TrimSpace(stripTags(title)) == "" {
			title = stripTags(m[2])
		}
		title = strings.TrimSpace(stripTags(title))
		href := absURL(base, hm[1])
		if href == "" || title == "" || seen[href] {
			continue
		}
		if !looksLikeDocLink(href) || !isGongkaoAnnouncementTitle(title) {
			continue
		}
		seen[href] = true
		out = append(out, cptaLink{href: href, title: title})
	}
	return out
}

func nextListPage(html, pageURL string) string {
	m := reListPage.FindStringSubmatch(html)
	if m == nil {
		return ""
	}
	total, curr := atoi(m[1]), atoi(m[2])
	if curr < 1 || curr >= total {
		return ""
	}
	nextName := fmt.Sprintf("%s_%d.%s", m[3], curr+1, m[4])
	return absURL(pageURL, nextName)
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
		title, pub = splitLeadingDate(title)
	}
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
		Region:        announcementRegion(title, s.region),
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
