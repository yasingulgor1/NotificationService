package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Channel represents the notification delivery channel
type Channel string

const (
	ChannelSMS   Channel = "sms"
	ChannelEmail Channel = "email"
	ChannelPush  Channel = "push"
)

func (c Channel) IsValid() bool {
	switch c {
	case ChannelSMS, ChannelEmail, ChannelPush:
		return true
	}
	return false
}

type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityNormal Priority = "normal"
	PriorityLow    Priority = "low"
)

// Weight returns the priority weight for queue ordering (lower = higher priority)
func (p Priority) Weight() int64 {
	switch p {
	case PriorityHigh:
		return 0
	case PriorityNormal:
		return 1000000
	case PriorityLow:
		return 2000000
	}
	return 1000000 // default to normal
}

func (p Priority) IsValid() bool {
	switch p {
	case PriorityHigh, PriorityNormal, PriorityLow:
		return true
	}
	return false
}

type Status string

const (
	StatusPending    Status = "pending"
	StatusScheduled  Status = "scheduled"
	StatusQueued     Status = "queued"
	StatusProcessing Status = "processing"
	StatusSent       Status = "sent"
	StatusDelivered  Status = "delivered"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

// Notification represents a notification entity
type Notification struct {
	ID             uuid.UUID      `json:"id"`
	BatchID        *uuid.UUID     `json:"batch_id,omitempty"`
	Recipient      string         `json:"recipient"`
	Channel        Channel        `json:"channel"`
	Content        string         `json:"content"`
	Priority       Priority       `json:"priority"`
	Status         Status         `json:"status"`
	ScheduledAt    *time.Time     `json:"scheduled_at,omitempty"`
	SentAt         *time.Time     `json:"sent_at,omitempty"`
	ExternalID     *string        `json:"external_id,omitempty"`
	RetryCount     int            `json:"retry_count"`
	IdempotencyKey *string        `json:"idempotency_key,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	ErrorMessage   *string        `json:"error_message,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

func NewNotification(recipient string, channel Channel, content string) *Notification {
	now := time.Now().UTC()
	return &Notification{
		ID:        uuid.New(),
		Recipient: recipient,
		Channel:   channel,
		Content:   content,
		Priority:  PriorityNormal,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (n *Notification) CanCancel() bool {
	return n.Status == StatusPending || n.Status == StatusScheduled || n.Status == StatusQueued
}

// MarkAsQueued updates the notification status to queued
func (n *Notification) MarkAsQueued() {
	n.Status = StatusQueued
	n.UpdatedAt = time.Now().UTC()
}

// MarkAsProcessing updates the notification status to processing
func (n *Notification) MarkAsProcessing() {
	n.Status = StatusProcessing
	n.UpdatedAt = time.Now().UTC()
}

// MarkAsSent updates the notification status to sent
func (n *Notification) MarkAsSent(externalID string) {
	n.Status = StatusSent
	n.ExternalID = &externalID
	now := time.Now().UTC()
	n.SentAt = &now
	n.UpdatedAt = now
}

// MarkAsFailed updates the notification status to failed
func (n *Notification) MarkAsFailed(errorMsg string) {
	n.Status = StatusFailed
	n.ErrorMessage = &errorMsg
	n.UpdatedAt = time.Now().UTC()
}

func (n *Notification) MarkAsCancelled() {
	n.Status = StatusCancelled
	n.UpdatedAt = time.Now().UTC()
}

func (n *Notification) IncrementRetry() {
	n.RetryCount++
	n.UpdatedAt = time.Now().UTC()
}

type NotificationFilter struct {
	Status    *Status
	Channel   *Channel
	BatchID   *uuid.UUID
	StartDate *time.Time
	EndDate   *time.Time
	Page      int
	PageSize  int
}

type NotificationListResult struct {
	Notifications []*Notification `json:"notifications"`
	Total         int64           `json:"total"`
	Page          int             `json:"page"`
	PageSize      int             `json:"page_size"`
	TotalPages    int             `json:"total_pages"`
}

type NotificationRepository interface {
	Create(ctx context.Context, notification *Notification) error
	CreateBatch(ctx context.Context, notifications []*Notification) error
	GetByID(ctx context.Context, id uuid.UUID) (*Notification, error)
	GetByBatchID(ctx context.Context, batchID uuid.UUID) ([]*Notification, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*Notification, error)
	Update(ctx context.Context, notification *Notification) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter NotificationFilter) (*NotificationListResult, error)
	GetScheduledNotifications(ctx context.Context, before time.Time, limit int) ([]*Notification, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status Status) error
}
