package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

// LoadDotEnv reads KEY=VALUE lines into process env if not already set.
func LoadDotEnv(paths ...string) {
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			i := strings.IndexByte(line, '=')
			if i <= 0 {
				continue
			}
			k := strings.TrimSpace(line[:i])
			v := strings.TrimSpace(line[i+1:])
			if os.Getenv(k) == "" {
				_ = os.Setenv(k, v)
			}
		}
		_ = f.Close()
	}
}

type Config struct {
	AppEnv                   string
	HTTPAddr                 string
	DatabaseURL              string
	RedisURL                 string
	JWTSecret                string
	JWTAccessTTL             time.Duration
	JWTRefreshTTL            time.Duration
	WXAppID                  string
	WXSecret                 string
	WXMock                   bool
	WXSubscribeTemplateTodo  string
	WXSubscribeTemplateAff   string
	CrawlerEnabled           bool
	CrawlerAffairsRSSURL     string
	AffairsDailyPushEnabled  bool
	SessionKeyAESKey         string
	AdminBootstrapUser       string
	AdminBootstrapPass       string
}

func Load() *Config {
	return &Config{
		AppEnv:                   getenv("APP_ENV", "dev"),
		HTTPAddr:                 getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:              getenv("DATABASE_URL", "postgres://gongkao:gongkao@127.0.0.1:5432/gongkao?sslmode=disable"),
		RedisURL:                 getenv("REDIS_URL", "redis://127.0.0.1:6379/0"),
		JWTSecret:                getenv("JWT_SECRET", "gongkao-dev-secret-change-me"),
		JWTAccessTTL:             durationEnv("JWT_ACCESS_TTL", 2*time.Hour),
		JWTRefreshTTL:            durationEnv("JWT_REFRESH_TTL", 720*time.Hour),
		WXAppID:                  os.Getenv("WX_APPID"),
		WXSecret:                 os.Getenv("WX_SECRET"),
		WXMock:                   boolEnv("WX_MOCK", true),
		WXSubscribeTemplateTodo:  os.Getenv("WX_SUBSCRIBE_TEMPLATE_TODO"),
		WXSubscribeTemplateAff:   os.Getenv("WX_SUBSCRIBE_TEMPLATE_AFFAIRS"),
		CrawlerEnabled:           boolEnv("CRAWLER_ENABLED", true),
		CrawlerAffairsRSSURL:     os.Getenv("CRAWLER_AFFAIRS_RSS_URL"),
		AffairsDailyPushEnabled:  boolEnv("AFFAIRS_DAILY_PUSH_ENABLED", false),
		SessionKeyAESKey:         getenv("SESSION_KEY_AES_KEY", "0123456789abcdef0123456789abcdef"),
		AdminBootstrapUser:       getenv("ADMIN_BOOTSTRAP_USER", "admin"),
		AdminBootstrapPass:       getenv("ADMIN_BOOTSTRAP_PASS", "admin123"),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func boolEnv(k string, def bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func durationEnv(k string, def time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}