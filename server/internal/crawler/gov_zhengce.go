package crawler

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"gongkao/internal/model"
)

// govZhengceSource crawls https://www.gov.cn/zhengce/zuixin/ZUIXINZHENGCE.json
// and keeps items that look like recruitment / exam notices.
type govZhengceSource struct {
	runner *Runner
	url    string
	client *http.Client
	limit  int
}

func (s *govZhengceSource) Name() string { return "gov-zhengce" }

type govZhengceItem struct {
	Title         string `json:"TITLE"`
	URL           string `json:"URL"`
	DocRelPubTime string `json:"DOCRELPUBTIME"`
	SubTitle      string `json:"SUB_TITLE"`
}

func (s *govZhengceSource) Fetch(ctx context.Context) error {
	body, err := fetchBytes(ctx, s.client, s.url)
	if err != nil {
		return err
	}
	var items []govZhengceItem
	if err := json.Unmarshal(body, &items); err != nil {
		return err
	}
	limit := s.limit
	if limit <= 0 {
		limit = 30
	}
	n := 0
	for _, it := range items {
		title := strings.TrimSpace(it.Title)
		link := strings.TrimSpace(it.URL)
		if title == "" || link == "" {
			continue
		}
		if !isExamRelatedTitle(title) {
			continue
		}
		pub := time.Now()
		if t, err := time.ParseInLocation("2006-01-02", it.DocRelPubTime, time.Local); err == nil {
			pub = t
		}
		sum := it.SubTitle
		if sum == "" {
			sum = truncateRunes(title, 120)
		}
		ann := &model.Announcement{
			Type:        classifyAnnouncementType(title),
			Title:       title,
			Region:      guessRegion(title),
			Source:      s.Name(),
			SourceID:    shortHash(link),
			SourceURL:   link,
			Summary:     sum,
			Content:     sum,
			PublishedAt: pub,
			Status:      model.StatusPublished,
		}
		if err := s.runner.ann.UpsertFromCrawl(ann); err != nil {
			_ = s.runner.recordFailure(s.Name(), link, err)
			continue
		}
		n++
		if n >= limit {
			break
		}
	}
	slog.Info("gov zhengce crawl done", "upserted", n, "scanned", len(items))
	return nil
}

func isExamRelatedTitle(title string) bool {
	return containsAny(title,
		"招考", "招聘", "录用", "遴选", "公开招聘", "考试录用",
		"公务员考", "事业单位公开", "事业单位招聘", "事业单位招",
		"选调", "人才引进", "考录",
	)
}

func classifyAnnouncementType(title string) string {
	if containsAny(title, "事业", "事业单位", "公开招聘") && !containsAny(title, "公务员") {
		return model.AnnouncementInstitution
	}
	return model.AnnouncementCivil
}

func announcementRegion(title, fallback string) string {
	region := guessRegion(title)
	if fallback != "" && region == "全国" {
		return fallback
	}
	return region
}

func guessRegion(title string) string {
	provinces := []string{
		"北京", "天津", "上海", "重庆", "河北", "山西", "辽宁", "吉林", "黑龙江",
		"江苏", "浙江", "安徽", "福建", "江西", "山东", "河南", "湖北", "湖南",
		"广东", "海南", "四川", "贵州", "云南", "陕西", "甘肃", "青海", "台湾",
		"内蒙古", "广西", "西藏", "宁夏", "新疆", "香港", "澳门",
	}
	for _, p := range provinces {
		if strings.Contains(title, p) {
			return p
		}
	}
	if containsAny(title, "中央", "国家", "国务院", "全国") {
		return "全国"
	}
	return "全国"
}

func shortHash(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:8])
}
