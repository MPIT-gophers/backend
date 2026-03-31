package max

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	errorsstatus "eventAI/internal/errorsStatus"
	"eventAI/internal/service"
)

const webAppDataKey = "WebAppData"

type Client struct {
	botToken string
}

func NewClient(botToken string) (*Client, error) {
	botToken = strings.TrimSpace(botToken)
	if botToken == "" {
		return nil, fmt.Errorf("max bot token is required")
	}

	return &Client{botToken: botToken}, nil
}

type maxUser struct {
	ID        json.Number `json:"id"`
	FirstName string      `json:"first_name"`
	LastName  string      `json:"last_name"`
	Username  string      `json:"username"`
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

func signHMAC(key []byte, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
