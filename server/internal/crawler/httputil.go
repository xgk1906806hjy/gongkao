package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 25 * time.Second}
}

func fetchBytes(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	if client == nil {
		client = defaultHTTPClient()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; gongkao-crawler/1.1)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml,application/json;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, truncateRunes(string(body), 200))
	}
	return body, nil
}

func fetchText(ctx context.Context, client *http.Client, url string) (string, error) {
	body, err := fetchBytes(ctx, client, url)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func fetchTextRetry(ctx context.Context, client *http.Client, url string, tries int) (string, error) {
	if tries < 1 {
		tries = 1
	}
	var err error
	for i := 0; i < tries; i++ {
		var text string
		text, err = fetchText(ctx, client, url)
		if err == nil {
			return text, nil
		}
	}
	return "", err
}

func absURL(base, href string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "javascript:") || href == "#" {
		return ""
	}
	bu, err := url.Parse(base)
	if err != nil {
		return ""
	}
	hu, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return strings.TrimRight(bu.ResolveReference(hu).String(), "?")
}

var reScriptStyle = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>|<style\b[^>]*>.*?</style>|<!--.*?-->`)

func stripTags(s string) string {
	s = reScriptStyle.ReplaceAllString(s, " ")
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	out = strings.NewReplacer(
		"&nbsp;", " ",
		"&#160;", " ",
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&#39;", "'",
		"\u00a0", " ",
	).Replace(out)
	return strings.Join(strings.Fields(out), " ")
}

// announcementExcerpt keeps the article body. The title is repeated in the
// document title, the navigation, and sometimes a trailing file id. The body
// is the copy that is followed by sentence punctuation.
func announcementExcerpt(text, title string) string {
	text = strings.TrimSpace(text)
	title = strings.TrimSpace(title)
	if title == "" || !strings.Contains(text, title) {
		return truncateRunes(text, 500)
	}
	var idxs []int
	from := 0
	for from < len(text) {
		rel := strings.Index(text[from:], title)
		if rel < 0 {
			break
		}
		idxs = append(idxs, from+rel)
		from += rel + len(title)
	}
	best := idxs[0]
	bestScore := -1
	for n, i := range idxs {
		end := len(text)
		if n+1 < len(idxs) {
			end = idxs[n+1]
		}
		seg := text[i:end]
		if rs := []rune(seg); len(rs) > 800 {
			seg = string(rs[:800])
		}
		score := strings.Count(seg, "。")*5 + strings.Count(seg, "，")
		if score > bestScore {
			bestScore = score
			best = i
		}
	}
	return truncateRunes(strings.TrimSpace(text[best:]), 500)
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func containsAny(s string, kws ...string) bool {
	for _, k := range kws {
		if k != "" && strings.Contains(s, k) {
			return true
		}
	}
	return false
}
