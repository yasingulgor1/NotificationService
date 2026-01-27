package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/insider-one/notification-service/internal/config"
	"github.com/insider-one/notification-service/internal/domain"
)

// WebhookProvider implements domain.NotificationProvider using webhook.site
type WebhookProvider struct {
	client  *http.Client
	baseURL string
}

// NewWebhookProvider creates a new WebhookProvider
func NewWebhookProvider(cfg config.WebhookConfig) *WebhookProvider {
	return &WebhookProvider{
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		baseURL: cfg.URL,
	}
}

// Send sends a notification to the webhook provider
func (p *WebhookProvider) Send(ctx context.Context, req *domain.ProviderRequest) (*domain.ProviderResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, domain.NewProviderError(0, fmt.Sprintf("request failed: %v", err), true)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		retryable := resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests
		return nil, domain.NewProviderError(resp.StatusCode, string(respBody), retryable)
	}

	// Parse responses
	var providerResp domain.ProviderResponse
	if err := json.Unmarshal(respBody, &providerResp); err != nil {
		// If error occurs generate our own response
		providerResp = domain.ProviderResponse{
			MessageID: fmt.Sprintf("msg-%d", time.Now().UnixNano()),
			Status:    "accepted",
			Timestamp: time.Now().UTC(),
		}
	}

	return &providerResp, nil
}
