package domain

import (
	"context"
	"time"
)

// ProviderRequest represents a request to the external notification provider
type ProviderRequest struct {
	To      string `json:"to"`
	Channel string `json:"channel"`
	Content string `json:"content"`
}

// ProviderResponse represents a response from the external notification provider
type ProviderResponse struct {
	MessageID string    `json:"messageId"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// NotificationProvider defines the interface for sending notifications
type NotificationProvider interface {
	// Send sends a notification to the external provider
	Send(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error)
}
