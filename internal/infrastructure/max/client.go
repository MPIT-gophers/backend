package max

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	errorsstatus "eventAI/internal/errorsStatus"
	"eventAI/internal/service"
)

const (
	webAppDataKey     = "WebAppData"
	defaultAPIBaseURL = "https://platform-api.max.ru"
)

type Client struct {
	botToken   string
	apiBaseURL string
	httpClient *http.Client
}

type BotInfo struct {
	ID       string
	Username string
	Name     string
}

type maxUser struct {
	ID        json.Number `json:"id"`
	FirstName string      `json:"first_name"`
	LastName  string      `json:"last_name"`
	Username  string      `json:"username"`
}

type maxBotMe struct {
	UserID    json.Number `json:"user_id"`
	Username  string      `json:"username"`
	FirstName string      `json:"first_name"`
	LastName  string      `json:"last_name"`
	Name      string      `json:"name"`
}

func NewClient(botToken string, apiBaseURL string) (*Client, error) {
	botToken = strings.TrimSpace(botToken)
	if botToken == "" {
		return nil, fmt.Errorf("max bot token is required")
	}

	apiBaseURL = strings.TrimRight(strings.TrimSpace(apiBaseURL), "/")
	if apiBaseURL == "" {
		apiBaseURL = defaultAPIBaseURL
	}

	return &Client{
		botToken:   botToken,
		apiBaseURL: apiBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (c *Client) GetMe(ctx context.Context) (BotInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBaseURL+"/me", nil)
	if err != nil {
		return BotInfo{}, err
	}
	req.Header.Set("Authorization", c.botToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return BotInfo{}, fmt.Errorf("request MAX /me: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return BotInfo{}, fmt.Errorf("MAX /me returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var me maxBotMe
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		return BotInfo{}, fmt.Errorf("decode MAX /me response: %w", err)
	}

	name := strings.TrimSpace(strings.Join([]string{
		strings.TrimSpace(me.FirstName),
		strings.TrimSpace(me.LastName),
	}, " "))
	if name == "" {
		name = strings.TrimSpace(me.Name)
	}

	return BotInfo{
		ID:       strings.TrimSpace(me.UserID.String()),
		Username: normalizeUsername(me.Username),
		Name:     name,
	}, nil
}

func (c *Client) ValidateInitData(_ context.Context, initData string) (service.MAXIdentity, error) {
	initData = strings.TrimSpace(initData)
	if initData == "" {
		return service.MAXIdentity{}, errorsstatus.ErrInvalidInput
	}

	values, err := url.ParseQuery(initData)
	if err != nil {
		return service.MAXIdentity{}, errorsstatus.ErrUnauthorized
	}

	hashValues := values["hash"]
	if len(hashValues) != 1 {
		return service.MAXIdentity{}, errorsstatus.ErrUnauthorized
	}
	originalHash := strings.TrimSpace(hashValues[0])
	if originalHash == "" {
		return service.MAXIdentity{}, errorsstatus.ErrUnauthorized
	}

	keys := make([]string, 0, len(values))
	for key, entries := range values {
		if len(entries) != 1 {
			return service.MAXIdentity{}, errorsstatus.ErrUnauthorized
		}
		if key == "hash" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	launchParams := make([]string, 0, len(keys))
	for _, key := range keys {
		launchParams = append(launchParams, key+"="+values.Get(key))
	}

	secretKey := signHMAC([]byte(webAppDataKey), []byte(c.botToken))
	actualHash := hex.EncodeToString(signHMAC(secretKey, []byte(strings.Join(launchParams, "\n"))))
	if !hmac.Equal([]byte(actualHash), []byte(originalHash)) {
		return service.MAXIdentity{}, errorsstatus.ErrUnauthorized
	}

	userRaw := strings.TrimSpace(values.Get("user"))
	if userRaw == "" {
		return service.MAXIdentity{}, errorsstatus.ErrUnauthorized
	}

	var user maxUser
	if err := json.Unmarshal([]byte(userRaw), &user); err != nil {
		return service.MAXIdentity{}, errorsstatus.ErrUnauthorized
	}

	providerUserID := strings.TrimSpace(user.ID.String())
	if providerUserID == "" {
		return service.MAXIdentity{}, errorsstatus.ErrUnauthorized
	}

	fullName := strings.TrimSpace(strings.Join([]string{
		strings.TrimSpace(user.FirstName),
		strings.TrimSpace(user.LastName),
	}, " "))
	if fullName == "" {
		fullName = strings.TrimSpace(user.Username)
	}

	authDateRaw := strings.TrimSpace(values.Get("auth_date"))
	if authDateRaw == "" {
		return service.MAXIdentity{}, errorsstatus.ErrUnauthorized
	}

	authDateUnix, err := strconv.ParseInt(authDateRaw, 10, 64)
	if err != nil || authDateUnix <= 0 {
		return service.MAXIdentity{}, errorsstatus.ErrUnauthorized
	}

	return service.MAXIdentity{
		ProviderUserID: providerUserID,
		FullName:       fullName,
		Phone:          nil,
		StartParam:     strings.TrimSpace(values.Get("start_param")),
		AuthDate:       time.Unix(authDateUnix, 0).UTC(),
	}, nil
}

func normalizeUsername(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "@")
	return value
}

func signHMAC(key []byte, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
