package max

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestClientValidateInitDataSuccess(t *testing.T) {
	t.Parallel()

	client, err := NewClient("bot-token", "")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	initData := buildSignedInitData("bot-token", map[string]string{
		"auth_date":   "1771409719",
		"query_id":    "q-1",
		"start_param": "session-123",
		"user":        `{"id":67890,"first_name":"Max","last_name":"User","username":"maxuser"}`,
	})

	identity, err := client.ValidateInitData(context.Background(), initData)
	if err != nil {
		t.Fatalf("ValidateInitData() error = %v", err)
	}

	if identity.ProviderUserID != "67890" {
		t.Fatalf("provider user id = %q, want 67890", identity.ProviderUserID)
	}

	if identity.FullName != "Max User" {
		t.Fatalf("full name = %q, want Max User", identity.FullName)
	}

	if identity.Phone != nil {
		t.Fatalf("phone = %v, want nil", identity.Phone)
	}

	if identity.StartParam != "session-123" {
		t.Fatalf("start param = %q, want session-123", identity.StartParam)
	}

	if !identity.AuthDate.Equal(time.Unix(1771409719, 0).UTC()) {
		t.Fatalf("auth date = %v", identity.AuthDate)
	}
}

func TestClientValidateInitDataRejectsBadHash(t *testing.T) {
	t.Parallel()

	client, err := NewClient("bot-token", "")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.ValidateInitData(context.Background(), `auth_date=1771409719&user=%7B%22id%22%3A1%7D&hash=deadbeef`)
	if err == nil {
		t.Fatal("ValidateInitData() error = nil, want error")
	}
}

func TestClientGetMeSuccess(t *testing.T) {
	t.Parallel()

	client, err := NewClient("bot-token", "https://platform-api.max.ru")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://platform-api.max.ru/me" {
				t.Fatalf("url = %q, want https://platform-api.max.ru/me", req.URL.String())
			}
			if req.Header.Get("Authorization") != "bot-token" {
				t.Fatalf("authorization = %q, want bot-token", req.Header.Get("Authorization"))
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"user_id":12345,"username":"test_bot","name":"Test Bot"}`)),
			}, nil
		}),
	}

	botInfo, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe() error = %v", err)
	}

	if botInfo.ID != "12345" {
		t.Fatalf("id = %q, want 12345", botInfo.ID)
	}
	if botInfo.Username != "test_bot" {
		t.Fatalf("username = %q, want test_bot", botInfo.Username)
	}
	if botInfo.Name != "Test Bot" {
		t.Fatalf("name = %q, want Test Bot", botInfo.Name)
	}
}

func buildSignedInitData(botToken string, values map[string]string) string {
	query := url.Values{}
	keys := make([]string, 0, len(values))
	for key, value := range values {
		query.Set(key, value)
		keys = append(keys, key)
	}

	launchParams := make([]string, 0, len(keys))
	sort.Strings(keys)
	for _, key := range keys {
		launchParams = append(launchParams, key+"="+values[key])
	}

	secretKey := testSignHMAC([]byte(webAppDataKey), []byte(botToken))
	hash := hex.EncodeToString(testSignHMAC(secretKey, []byte(strings.Join(launchParams, "\n"))))
	query.Set("hash", hash)

	return query.Encode()
}

func testSignHMAC(key []byte, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
