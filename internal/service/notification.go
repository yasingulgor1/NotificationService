package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/insider-one/notification-service/internal/domain"
)

const (
	maxBatchSize = 1000
)

// NotificationService handles notification business logic
type NotificationService struct {
	repo            domain.NotificationRepository
	templateRepo    domain.TemplateRepository
	queue           domain.Queue
	logger          *slog.Logger
	statusBroadcast func(notification *domain.Notification)
}

// NewNotificationService creates a new NotificationService
func NewNotificationService(
	repo domain.NotificationRepository,
	templateRepo domain.TemplateRepository,
	queue domain.Queue,
	logger *slog.Logger,
) *NotificationService {
	return &NotificationService{
		repo:         repo,
		templateRepo: templateRepo,
		queue:        queue,
		logger:       logger,
	}
}

// SetStatusBroadcast sets the function to broadcast status updates
func (s *NotificationService) SetStatusBroadcast(fn func(notification *domain.Notification)) {
	s.statusBroadcast = fn
}

// CreateRequest represents a request to create a notification
type CreateRequest struct {
	Recipient      string            `json:"recipient" validate:"required"`
	Channel        domain.Channel    `json:"channel" validate:"required"`
	Content        string            `json:"content"`
	Priority       domain.Priority   `json:"priority"`
	ScheduledAt    *time.Time        `json:"scheduled_at,omitempty"`
	IdempotencyKey *string           `json:"idempotency_key,omitempty"`
	Metadata       map[string]any    `json:"metadata,omitempty"`
	TemplateName   *string           `json:"template_name,omitempty"`
	TemplateVars   map[string]string `json:"template_vars,omitempty"`
}

// BatchCreateRequest represents a request to create multiple notifications
type BatchCreateRequest struct {
	Notifications []CreateRequest `json:"notifications" validate:"required,min=1,max=1000,dive"`
}

// Create creates a single notification
func (s *NotificationService) Create(ctx context.Context, req CreateRequest) (*domain.Notification, error) {
	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := s.repo.GetByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil && existing != nil {
			return existing, nil
		}
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("failed to check idempotency: %w", err)
		}
	}

	// Validate channel
	if !req.Channel.IsValid() {
		return nil, domain.NewValidationError("channel", "invalid channel")
	}

	// Get content from template if specified
	content := req.Content
	if req.TemplateName != nil {
		template, err := s.templateRepo.GetByName(ctx, *req.TemplateName)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, domain.ErrTemplateNotFound
			}
			return nil, fmt.Errorf("failed to get template: %w", err)
		}

		// Validate template variables
		missing := template.Validate(req.TemplateVars)
		if len(missing) > 0 {
			return nil, fmt.Errorf("%w: %v", domain.ErrMissingVariables, missing)
		}

		content = template.Render(req.TemplateVars)
	}

	// Validate content
	if content == "" {
		return nil, domain.NewValidationError("content", "content is required")
	}

	// Validate content length per channel
	if err := validateContentLength(req.Channel, content); err != nil {
		return nil, err
	}

	// Create notification
	notification := domain.NewNotification(req.Recipient, req.Channel, content)

	if req.Priority != "" && req.Priority.IsValid() {
		notification.Priority = req.Priority
	}

	if req.ScheduledAt != nil {
		if req.ScheduledAt.Before(time.Now()) {
			return nil, domain.NewValidationError("scheduled_at", "scheduled time must be in the future")
		}
		notification.ScheduledAt = req.ScheduledAt
		notification.Status = domain.StatusScheduled
	}

	notification.IdempotencyKey = req.IdempotencyKey
	notification.Metadata = req.Metadata

	// Save to database
	if err := s.repo.Create(ctx, notification); err != nil {
		if errors.Is(err, domain.ErrIdempotencyConflict) {
			// Another request with same key was created, return existing
			return s.repo.GetByIdempotencyKey(ctx, *req.IdempotencyKey)
		}
		return nil, fmt.Errorf("failed to create notification: %w", err)
	}

	// Queue if not scheduled
	if notification.Status == domain.StatusPending {
		if err := s.enqueueNotification(ctx, notification); err != nil {
			s.logger.Error("failed to enqueue notification",
				"notification_id", notification.ID,
				"error", err,
			)
			// Don't fail the request, notification is saved
		}
	}

	s.logger.Info("notification created",
		"notification_id", notification.ID,
		"channel", notification.Channel,
		"status", notification.Status,
	)

	return notification, nil
}

// CreateBatch creates multiple notifications
func (s *NotificationService) CreateBatch(ctx context.Context, req BatchCreateRequest) ([]*domain.Notification, error) {
	if len(req.Notifications) > maxBatchSize {
		return nil, domain.ErrBatchSizeExceeded
	}

	batchID := uuid.New()
	notifications := make([]*domain.Notification, 0, len(req.Notifications))
	queueItems := make([]*domain.QueueItem, 0, len(req.Notifications))

	for i, createReq := range req.Notifications {
		// Validate channel
		if !createReq.Channel.IsValid() {
			return nil, fmt.Errorf("notification %d: %w", i, domain.NewValidationError("channel", "invalid channel"))
		}

		// Get content
		content := createReq.Content
		if createReq.TemplateName != nil {
			template, err := s.templateRepo.GetByName(ctx, *createReq.TemplateName)
			if err != nil {
				return nil, fmt.Errorf("notification %d: %w", i, domain.ErrTemplateNotFound)
			}
			content = template.Render(createReq.TemplateVars)
		}

		if content == "" {
			return nil, fmt.Errorf("notification %d: %w", i, domain.NewValidationError("content", "content is required"))
		}

		// Create notification
		notification := domain.NewNotification(createReq.Recipient, createReq.Channel, content)
		notification.BatchID = &batchID

		if createReq.Priority != "" && createReq.Priority.IsValid() {
			notification.Priority = createReq.Priority
		}

		if createReq.ScheduledAt != nil {
			notification.ScheduledAt = createReq.ScheduledAt
			notification.Status = domain.StatusScheduled
		}

		notification.IdempotencyKey = createReq.IdempotencyKey
		notification.Metadata = createReq.Metadata

		notifications = append(notifications, notification)

		// Prepare queue item if not scheduled
		if notification.Status == domain.StatusPending {
			queueItems = append(queueItems, &domain.QueueItem{
				NotificationID: notification.ID,
				Channel:        notification.Channel,
				Priority:       notification.Priority,
				RetryCount:     0,
			})
		}
	}

	// Save batch to database
	if err := s.repo.CreateBatch(ctx, notifications); err != nil {
		return nil, fmt.Errorf("failed to create batch: %w", err)
	}

	// Queue notifications
	if len(queueItems) > 0 {
		if err := s.queue.EnqueueBatch(ctx, queueItems); err != nil {
			s.logger.Error("failed to enqueue batch",
				"batch_id", batchID,
				"error", err,
			)
		} else {
			// Update status to queued
			for _, n := range notifications {
				if n.Status == domain.StatusPending {
					n.Status = domain.StatusQueued
				}
			}
		}
	}

	s.logger.Info("batch created",
		"batch_id", batchID,
		"count", len(notifications),
	)

	return notifications, nil
}

// GetByID retrieves a notification by ID
func (s *NotificationService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	return s.repo.GetByID(ctx, id)
}

// GetByBatchID retrieves all notifications in a batch
func (s *NotificationService) GetByBatchID(ctx context.Context, batchID uuid.UUID) ([]*domain.Notification, error) {
	return s.repo.GetByBatchID(ctx, batchID)
}

// Cancel cancels a pending notification
func (s *NotificationService) Cancel(ctx context.Context, id uuid.UUID) error {
	notification, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if !notification.CanCancel() {
		return domain.ErrCannotCancel
	}

	notification.MarkAsCancelled()

	if err := s.repo.Update(ctx, notification); err != nil {
		return fmt.Errorf("failed to cancel notification: %w", err)
	}

	s.broadcastStatus(notification)

	s.logger.Info("notification cancelled",
		"notification_id", id,
	)

	return nil
}

// List lists notifications with filters
func (s *NotificationService) List(ctx context.Context, filter domain.NotificationFilter) (*domain.NotificationListResult, error) {
	return s.repo.List(ctx, filter)
}

// UpdateStatus updates the status of a notification
func (s *NotificationService) UpdateStatus(ctx context.Context, notification *domain.Notification) error {
	if err := s.repo.Update(ctx, notification); err != nil {
		return err
	}
	s.broadcastStatus(notification)
	return nil
}

// enqueueNotification adds a notification to the processing queue
func (s *NotificationService) enqueueNotification(ctx context.Context, notification *domain.Notification) error {
	item := &domain.QueueItem{
		NotificationID: notification.ID,
		Channel:        notification.Channel,
		Priority:       notification.Priority,
		RetryCount:     notification.RetryCount,
	}

	if err := s.queue.Enqueue(ctx, item); err != nil {
		return err
	}

	notification.MarkAsQueued()
	return s.repo.Update(ctx, notification)
}

// broadcastStatus broadcasts status update via WebSocket
func (s *NotificationService) broadcastStatus(notification *domain.Notification) {
	if s.statusBroadcast != nil {
		s.statusBroadcast(notification)
	}
}

// validateContentLength validates content length based on channel
func validateContentLength(channel domain.Channel, content string) error {
	var maxLen int
	switch channel {
	case domain.ChannelSMS:
		maxLen = 160 * 4 // Allow up to 4 SMS segments
	case domain.ChannelEmail:
		maxLen = 100000 // 100KB
	case domain.ChannelPush:
		maxLen = 4096 // 4KB
	}

	if len(content) > maxLen {
		return domain.NewValidationError("content",
			fmt.Sprintf("content exceeds maximum length of %d characters for %s channel", maxLen, channel))
	}

	return nil
}
