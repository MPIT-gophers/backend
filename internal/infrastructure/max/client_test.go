package max

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"testing"
)

func TestClientValidateInitDataSuccess(t *testing.T) {
	t.Parallel()

	client, err := NewClient("bot-token")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	initData := buildSignedInitData("bot-token", map[string]string{
		"auth_date": "1771409719",
		"query_id":  "q-1",
		"user":      `{"id":67890,"first_name":"Max","last_name":"User","username":"maxuser"}`,
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
}

func TestClientValidateInitDataRejectsBadHash(t *testing.T) {
	t.Parallel()

	client, err := NewClient("bot-token")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.ValidateInitData(context.Background(), `auth_date=1771409719&user=%7B%22id%22%3A1%7D&hash=deadbeef`)
	if err == nil {
		t.Fatal("ValidateInitData() error = nil, want error")
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
