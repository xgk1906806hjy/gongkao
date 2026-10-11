package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	return bu.ResolveReference(hu).String()
}

func stripTags(s string) string {
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
	out = strings.ReplaceAll(out, "\u00a0", " ")
	return strings.Join(strings.Fields(out), " ")
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
