package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChannel_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		channel Channel
		want    bool
	}{
		{"valid sms", ChannelSMS, true},
		{"valid email", ChannelEmail, true},
		{"valid push", ChannelPush, true},
		{"invalid channel", Channel("invalid"), false},
		{"empty channel", Channel(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.channel.IsValid())
		})
	}
}

func TestPriority_Weight(t *testing.T) {
	tests := []struct {
		name     string
		priority Priority
		want     int64
	}{
		{"high priority", PriorityHigh, 0},
		{"normal priority", PriorityNormal, 1000000},
		{"low priority", PriorityLow, 2000000},
		{"invalid priority defaults to normal", Priority("invalid"), 1000000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.priority.Weight())
		})
	}
}

func TestPriority_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		priority Priority
		want     bool
	}{
		{"valid high", PriorityHigh, true},
		{"valid normal", PriorityNormal, true},
		{"valid low", PriorityLow, true},
		{"invalid priority", Priority("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.priority.IsValid())
		})
	}
}

func TestNewNotification(t *testing.T) {
	recipient := "+905551234567"
	channel := ChannelSMS
	content := "Test message"

	n := NewNotification(recipient, channel, content)

	assert.NotNil(t, n)
	assert.NotEmpty(t, n.ID)
	assert.Equal(t, recipient, n.Recipient)
	assert.Equal(t, channel, n.Channel)
	assert.Equal(t, content, n.Content)
	assert.Equal(t, PriorityNormal, n.Priority)
	assert.Equal(t, StatusPending, n.Status)
	assert.NotZero(t, n.CreatedAt)
	assert.NotZero(t, n.UpdatedAt)
}

func TestNotification_CanCancel(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"pending can cancel", StatusPending, true},
		{"scheduled can cancel", StatusScheduled, true},
		{"queued can cancel", StatusQueued, true},
		{"processing cannot cancel", StatusProcessing, false},
		{"sent cannot cancel", StatusSent, false},
		{"delivered cannot cancel", StatusDelivered, false},
		{"failed cannot cancel", StatusFailed, false},
		{"cancelled cannot cancel", StatusCancelled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNotification("test", ChannelSMS, "test")
			n.Status = tt.status
			assert.Equal(t, tt.want, n.CanCancel())
		})
	}
}

func TestNotification_StatusTransitions(t *testing.T) {
	n := NewNotification("+905551234567", ChannelSMS, "Test")
	originalUpdatedAt := n.UpdatedAt

	// Small delay to ensure time difference
	time.Sleep(time.Millisecond)

	// Test MarkAsQueued
	n.MarkAsQueued()
	assert.Equal(t, StatusQueued, n.Status)
	assert.True(t, n.UpdatedAt.After(originalUpdatedAt))

	// Test MarkAsProcessing
	n.MarkAsProcessing()
	assert.Equal(t, StatusProcessing, n.Status)

	// Test MarkAsSent
	externalID := "ext-123"
	n.MarkAsSent(externalID)
	assert.Equal(t, StatusSent, n.Status)
	assert.Equal(t, &externalID, n.ExternalID)
	assert.NotNil(t, n.SentAt)

	// Test MarkAsFailed
	n2 := NewNotification("+905551234567", ChannelSMS, "Test")
	errorMsg := "Provider error"
	n2.MarkAsFailed(errorMsg)
	assert.Equal(t, StatusFailed, n2.Status)
	assert.Equal(t, &errorMsg, n2.ErrorMessage)

	// Test MarkAsCancelled
	n3 := NewNotification("+905551234567", ChannelSMS, "Test")
	n3.MarkAsCancelled()
	assert.Equal(t, StatusCancelled, n3.Status)
}

func TestNotification_IncrementRetry(t *testing.T) {
	n := NewNotification("+905551234567", ChannelSMS, "Test")
	assert.Equal(t, 0, n.RetryCount)

	n.IncrementRetry()
	assert.Equal(t, 1, n.RetryCount)

	n.IncrementRetry()
	assert.Equal(t, 2, n.RetryCount)
}
