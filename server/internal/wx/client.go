package wx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	appID     string
	secret    string
	mock      bool
	http      *http.Client
	rdb       *redis.Client
	mu        sync.Mutex
	token     string
	tokenExp  time.Time
}

func NewClient(appID, secret string, mock bool, rdb *redis.Client) *Client {
	return &Client{
		appID:  appID,
		secret: secret,
		mock:   mock,
		http:   &http.Client{Timeout: 10 * time.Second},
		rdb:    rdb,
	}
}

type SessionResult struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

func (c *Client) Code2Session(ctx context.Context, code string) (*SessionResult, error) {
	if c.mock {
		oid := "mock_" + code
		if code == "" {
			oid = "mock_dev_user"
		}
		return &SessionResult{OpenID: oid, SessionKey: "mock_session_key"}, nil
	}
	u := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		url.QueryEscape(c.appID), url.QueryEscape(c.secret), url.QueryEscape(code),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out SessionResult
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if out.ErrCode != 0 {
		return nil, fmt.Errorf("wx code2session: %d %s", out.ErrCode, out.ErrMsg)
	}
	return &out, nil
}

type tokenResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

func (c *Client) AccessToken(ctx context.Context) (string, error) {
	if c.mock {
		return "mock_access_token", nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExp) {
		return c.token, nil
	}
	if c.rdb != nil {
		if v, err := c.rdb.Get(ctx, "wx:access_token").Result(); err == nil && v != "" {
			c.token = v
			c.tokenExp = time.Now().Add(5 * time.Minute)
			return v, nil
		}
	}
	u := fmt.Sprintf(
		"https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		url.QueryEscape(c.appID), url.QueryEscape(c.secret),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out tokenResp
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.ErrCode != 0 || out.AccessToken == "" {
		return "", fmt.Errorf("wx token: %d %s", out.ErrCode, out.ErrMsg)
	}
	ttl := time.Duration(out.ExpiresIn-120) * time.Second
	if ttl < time.Minute {
		ttl = time.Minute
	}
	c.token = out.AccessToken
	c.tokenExp = time.Now().Add(ttl)
	if c.rdb != nil {
		_ = c.rdb.Set(ctx, "wx:access_token", out.AccessToken, ttl).Err()
	}
	return out.AccessToken, nil
}

type SubscribeReq struct {
	ToUser     string                 `json:"touser"`
	TemplateID string                 `json:"template_id"`
	Page       string                 `json:"page,omitempty"`
	Data       map[string]interface{} `json:"data"`
}

func (c *Client) SendSubscribe(ctx context.Context, req SubscribeReq) error {
	if c.mock {
		slog.Info("wx mock subscribe message", "touser", req.ToUser, "template", req.TemplateID, "data", req.Data)
		return nil
	}
	token, err := c.AccessToken(ctx)
	if err != nil {
		return err
	}
	u := "https://api.weixin.qq.com/cgi-bin/message/subscribe/send?access_token=" + url.QueryEscape(token)
	payload, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	_ = json.Unmarshal(body, &out)
	if out.ErrCode != 0 {
		return fmt.Errorf("subscribe send: %d %s", out.ErrCode, out.ErrMsg)
	}
	return nil
}