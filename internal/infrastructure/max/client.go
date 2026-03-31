package max

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	errorsstatus "eventAI/internal/errorsStatus"
	"eventAI/internal/service"
)

type Client struct {
	httpClient  *http.Client
	validateURL string
	apiKey      string
}

func NewClient(validateURL string, timeout time.Duration, apiKey string) (*Client, error) {
	if strings.TrimSpace(validateURL) == "" {
		return nil, fmt.Errorf("max validate url is required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &Client{
		httpClient:  &http.Client{Timeout: timeout},
		validateURL: validateURL,
		apiKey:      strings.TrimSpace(apiKey),
	}, nil
}

type validateRequest struct {
	Token string `json:"token"`
}

type validateResponse struct {
	Data      *validateUser `json:"data"`
	ID        string        `json:"id"`
	UserID    string        `json:"user_id"`
	MaxUserID string        `json:"max_user_id"`
	FullName  string        `json:"full_name"`
	Name      string        `json:"name"`
	Phone     string        `json:"phone"`
}

type validateUser struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	MaxUserID string `json:"max_user_id"`
	FullName  string `json:"full_name"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
}

func (c *Client) ValidateToken(ctx context.Context, token string) (service.MAXIdentity, error) {
	body, err := json.Marshal(validateRequest{Token: token})
	if err != nil {
		return service.MAXIdentity{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.validateURL, bytes.NewReader(body))
	if err != nil {
		return service.MAXIdentity{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if isTimeoutOrNetwork(err) {
			return service.MAXIdentity{}, errorsstatus.ErrServiceUnavailable
		}
		return service.MAXIdentity{}, errorsstatus.ErrServiceUnavailable
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return service.MAXIdentity{}, errorsstatus.ErrUnauthorized
	case resp.StatusCode >= http.StatusInternalServerError:
		return service.MAXIdentity{}, errorsstatus.ErrServiceUnavailable
	case resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices:
		return service.MAXIdentity{}, errorsstatus.ErrUnauthorized
	}

	var payload validateResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return service.MAXIdentity{}, errorsstatus.ErrServiceUnavailable
	}

	providerUserID := firstNonEmpty(
		payload.MaxUserID,
		payload.UserID,
		payload.ID,
		func() string {
			if payload.Data == nil {
				return ""
			}
			return firstNonEmpty(payload.Data.MaxUserID, payload.Data.UserID, payload.Data.ID)
		}(),
	)
	if providerUserID == "" {
		return service.MAXIdentity{}, errorsstatus.ErrUnauthorized
	}

	fullName := firstNonEmpty(
		payload.FullName,
		payload.Name,
		func() string {
			if payload.Data == nil {
				return ""
			}
			return firstNonEmpty(payload.Data.FullName, payload.Data.Name)
		}(),
	)

	phoneValue := firstNonEmpty(
		payload.Phone,
		func() string {
			if payload.Data == nil {
				return ""
			}
			return payload.Data.Phone
		}(),
	)

	var phone *string
	if phoneValue != "" {
		phone = &phoneValue
	}

	return service.MAXIdentity{
		ProviderUserID: providerUserID,
		FullName:       fullName,
		Phone:          phone,
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func isTimeoutOrNetwork(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr)
}
