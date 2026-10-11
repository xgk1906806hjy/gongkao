package crawler

import (
	"context"
	"encoding/xml"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"gongkao/internal/model"
)

type rssFeed struct {
	Channel struct {
		Title string    `xml:"title"`
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Category    string `xml:"category"`
	GUID        string `xml:"guid"`
}

func parseRSS(body string) (*rssFeed, error) {
	var feed rssFeed
	if err := xml.Unmarshal([]byte(body), &feed); err != nil {
		return nil, err
	}
	return &feed, nil
}

func parsePubDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now()
	}
	layouts := []string{
		time.RFC1123Z, time.RFC1123, time.RFC3339,
		"2006-01-02 15:04:05", "2006-01-02",
		"Mon, 02 Jan 2006 15:04:05 MST",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Now()
}

type rssAffairsSource struct {
	runner *Runner
	name   string
	url    string
	tag    string
	client *http.Client
	limit  int
}

func (s *rssAffairsSource) Name() string {
	if s.name != "" {
		return s.name
	}
	return "rss-affairs"
}

func (s *rssAffairsSource) Fetch(ctx context.Context) error {
	body, err := fetchText(ctx, s.client, s.url)
	if err != nil {
		return err
	}
	feed, err := parseRSS(body)
	if err != nil {
		return err
	}
	limit := s.limit
	if limit <= 0 {
		limit = 40
	}
	n := 0
	for i, it := range feed.Channel.Items {
		if i >= limit {
			break
		}
		title := strings.TrimSpace(stripTags(it.Title))
		link := strings.TrimSpace(it.Link)
		if link == "" {
			link = strings.TrimSpace(it.GUID)
		}
		if title == "" || link == "" {
			continue
		}
		summary := stripTags(it.Description)
		tags := s.tag
		if cat := strings.TrimSpace(it.Category); cat != "" {
			if tags != "" {
				tags = tags + "," + cat
			} else {
				tags = cat
			}
		}
		art := &model.AffairsArticle{
			Title:       title,
			Source:      s.Name(),
			SourceURL:   link,
			Summary:     truncateRunes(summary, 300),
			Content:     summary,
			Tags:        tags,
			PublishedAt: parsePubDate(it.PubDate),
			Status:      model.StatusPublished,
		}
		if err := s.runner.aff.UpsertFromCrawl(art); err != nil {
			_ = s.runner.recordFailure(s.Name(), link, err)
			continue
		}
		n++
	}
	slog.Info("rss affairs crawl done", "source", s.Name(), "upserted", n, "url", s.url)
	return nil
}
