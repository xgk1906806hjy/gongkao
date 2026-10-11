package crawler

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

type timeline struct {
	RegisterStart *time.Time
	RegisterEnd   *time.Time
	ConfirmStart  *time.Time
	ConfirmEnd    *time.Time
	ExamAt        *time.Time
}

var (
	reLooseDate = regexp.MustCompile(`(?:(\d{4})\s*年\s*)?(\d{1,2})\s*月\s*(\d{1,2})\s*日(?:\s*(\d{1,2})\s*[:：]\s*(\d{2}))?`)
	reTrailYMD  = regexp.MustCompile(`(20\d{2})-(\d{2})-(\d{2})\s*$`)
	reTrailMD   = regexp.MustCompile(`(\d{2})-(\d{2})\s*$`)
	rePublished = regexp.MustCompile(`发布日期[:：\s]*(\d{4})\s*[-年/]\s*(\d{1,2})\s*[-月/]\s*(\d{1,2})`)
)

func extractTimeline(text string) timeline {
	text = strings.Join(strings.Fields(text), " ")
	var tl timeline
	if w := keywordWindow(text, []string{"报名时间", "网上报名时间", "报名"}, 160); w != "" {
		ds := parseLooseDates(w)
		if len(ds) > 0 {
			tl.RegisterStart = &ds[0]
		}
		if len(ds) > 1 {
			tl.RegisterEnd = &ds[1]
		}
	}
	if w := keywordWindow(text, []string{"确认时间", "资格审查时间", "资格审查", "现场确认"}, 100); w != "" {
		ds := parseLooseDates(w)
		if len(ds) > 0 {
			tl.ConfirmStart = &ds[0]
		}
		if len(ds) > 1 {
			tl.ConfirmEnd = &ds[1]
		}
	}
	if w := keywordWindow(text, []string{"笔试时间", "考试时间", "公共科目笔试"}, 100); w != "" {
		ds := parseLooseDates(w)
		if len(ds) > 0 {
			tl.ExamAt = &ds[0]
		}
	}
	return tl
}

func keywordWindow(text string, keywords []string, n int) string {
	bestAt := -1
	bestScore := int(^uint(0) >> 1)
	for _, k := range keywords {
		from := 0
		for from < len(text) {
			rel := strings.Index(text[from:], k)
			if rel < 0 {
				break
			}
			at := from + rel
			rest := text[at+len(k):]
			score := 1 << 20
			if loc := reLooseDate.FindStringIndex(rest); loc != nil {
				score = loc[0]
			}
			score -= len(k)
			if score < bestScore {
				bestScore = score
				bestAt = at + len(k)
			}
			from = at + len(k)
		}
	}
	if bestAt < 0 || bestAt > len(text) {
		return ""
	}
	rest := []rune(text[bestAt:])
	if len(rest) > n {
		rest = rest[:n]
	}
	return string(rest)
}

func parseLooseDates(s string) []time.Time {
	year := time.Now().Year()
	var out []time.Time
	for _, m := range reLooseDate.FindAllStringSubmatch(s, -1) {
		if m[1] != "" {
			year = atoi(m[1])
		}
		month := atoi(m[2])
		day := atoi(m[3])
		hour, min := 0, 0
		if m[4] != "" {
			hour = atoi(m[4])
			min = atoi(m[5])
		}
		if month < 1 || month > 12 || day < 1 || day > 31 {
			continue
		}
		out = append(out, time.Date(year, time.Month(month), day, hour, min, 0, 0, time.Local))
	}
	return out
}

func splitTrailingDate(title string, now time.Time) (string, time.Time) {
	title = strings.TrimSpace(title)
	if m := reTrailYMD.FindStringSubmatch(title); m != nil {
		t := time.Date(atoi(m[1]), time.Month(atoi(m[2])), atoi(m[3]), 0, 0, 0, 0, time.Local)
		return strings.TrimSpace(strings.TrimSuffix(title, m[0])), t
	}
	if m := reTrailMD.FindStringSubmatch(title); m != nil {
		t := time.Date(now.Year(), time.Month(atoi(m[1])), atoi(m[2]), 0, 0, 0, 0, time.Local)
		return strings.TrimSpace(strings.TrimSuffix(title, m[0])), t
	}
	return title, time.Time{}
}

var reURLDate = regexp.MustCompile(`t(20\d{2})(\d{2})(\d{2})`)

func parseURLDate(raw string) *time.Time {
	m := reURLDate.FindStringSubmatch(raw)
	if m == nil {
		return nil
	}
	month, day := atoi(m[2]), atoi(m[3])
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return nil
	}
	t := time.Date(atoi(m[1]), time.Month(month), day, 0, 0, 0, 0, time.Local)
	return &t
}

func parsePublishedAt(text string) *time.Time {
	m := rePublished.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	t := time.Date(atoi(m[1]), time.Month(atoi(m[2])), atoi(m[3]), 0, 0, 0, 0, time.Local)
	return &t
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
