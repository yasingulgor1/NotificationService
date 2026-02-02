package worker

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/insider-one/notification-service/internal/config"
	"github.com/insider-one/notification-service/internal/domain"
)

// Processor handles notification processing
type Processor struct {
	notificationRepo domain.NotificationRepository
	queue            domain.Queue
	rateLimiter      domain.RateLimiter
	provider         domain.NotificationProvider
	logger           *slog.Logger
	config           config.RetryConfig
	workerConfig     config.WorkerConfig
	statusBroadcast  func(notification *domain.Notification)

	mu         sync.Mutex
	running    bool
	wg         sync.WaitGroup
	cancelFunc context.CancelFunc
}

// NewProcessor creates a new Processor
func NewProcessor(
	notificationRepo domain.NotificationRepository,
	queue domain.Queue,
	rateLimiter domain.RateLimiter,
	provider domain.NotificationProvider,
	logger *slog.Logger,
	retryConfig config.RetryConfig,
	workerConfig config.WorkerConfig,
) *Processor {
	return &Processor{
		notificationRepo: notificationRepo,
		queue:            queue,
		rateLimiter:      rateLimiter,
		provider:         provider,
		logger:           logger,
		config:           retryConfig,
		workerConfig:     workerConfig,
	}
}

// SetStatusBroadcast sets the function to broadcast status updates
func (p *Processor) SetStatusBroadcast(fn func(notification *domain.Notification)) {
	p.statusBroadcast = fn
}

// Start starts the worker pool
func (p *Processor) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return nil
	}
	p.running = true
	p.mu.Unlock()

	ctx, p.cancelFunc = context.WithCancel(ctx)

	// Start workers for each channel
	channels := []struct {
		channel domain.Channel
		count   int
	}{
		{domain.ChannelSMS, p.workerConfig.SMSCount},
		{domain.ChannelEmail, p.workerConfig.EmailCount},
		{domain.ChannelPush, p.workerConfig.PushCount},
	}

	for _, ch := range channels {
		for i := 0; i < ch.count; i++ {
			p.wg.Add(1)
			go p.worker(ctx, ch.channel, i)
		}
	}

	p.logger.Info("processor started",
		"sms_workers", p.workerConfig.SMSCount,
		"email_workers", p.workerConfig.EmailCount,
		"push_workers", p.workerConfig.PushCount,
	)

	return nil
}

// Stop stops the worker pool
func (p *Processor) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	p.mu.Unlock()

	if p.cancelFunc != nil {
		p.cancelFunc()
	}

	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("processor stopped gracefully")
	case <-time.After(30 * time.Second):
		p.logger.Warn("processor stop timed out")
	}
}

// worker is the main worker loop for a channel
func (p *Processor) worker(ctx context.Context, channel domain.Channel, workerID int) {
	defer p.wg.Done()

	logger := p.logger.With(
		"channel", channel,
		"worker_id", workerID,
	)

	logger.Info("worker started")

	for {
		select {
		case <-ctx.Done():
			logger.Info("worker stopped")
			return
		default:
			if err := p.processNext(ctx, channel, logger); err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				logger.Error("failed to process notification", "error", err)
			}
		}
	}
}

// processNext processes the next notification from the queue
func (p *Processor) processNext(ctx context.Context, channel domain.Channel, logger *slog.Logger) error {
	// Wait for rate limit
	if err := p.rateLimiter.Wait(ctx, channel); err != nil {
		return err
	}

	// Dequeue next item
	item, err := p.queue.Dequeue(ctx, channel)
	if err != nil {
		return err
	}

	if item == nil {
		// Queue is empty, wait a bit before checking again
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return nil
		}
	}

	// Get notification from database
	notification, err := p.notificationRepo.GetByID(ctx, item.NotificationID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			logger.Warn("notification not found", "notification_id", item.NotificationID)
			return nil
		}
		return err
	}

	// Skip if already processed or cancelled
	if notification.Status == domain.StatusSent ||
		notification.Status == domain.StatusDelivered ||
		notification.Status == domain.StatusCancelled {
		return nil
	}

	// Process notification
	return p.processNotification(ctx, notification, logger)
}

// processNotification sends a notification to the provider
func (p *Processor) processNotification(ctx context.Context, notification *domain.Notification, logger *slog.Logger) error {
	logger = logger.With("notification_id", notification.ID)

	// Update status to processing
	notification.MarkAsProcessing()
	if err := p.notificationRepo.Update(ctx, notification); err != nil {
		return err
	}
	p.broadcastStatus(notification)

	// Send to provider
	req := &domain.ProviderRequest{
		To:      notification.Recipient,
		Channel: string(notification.Channel),
		Content: notification.Content,
	}

	resp, err := p.provider.Send(ctx, req)
	if err != nil {
		return p.handleSendError(ctx, notification, err, logger)
	}

	// Mark as sent
	notification.MarkAsSent(resp.MessageID)
	if err := p.notificationRepo.Update(ctx, notification); err != nil {
		return err
	}
	p.broadcastStatus(notification)

	logger.Info("notification sent",
		"external_id", resp.MessageID,
	)

	return nil
}

// handleSendError handles send errors and retries
func (p *Processor) handleSendError(ctx context.Context, notification *domain.Notification, err error, logger *slog.Logger) error {
	var providerErr domain.ProviderError
	if errors.As(err, &providerErr) {
		if !providerErr.Retryable {
			// Non-retryable error, mark as failed
			notification.MarkAsFailed(providerErr.Message)
			if updateErr := p.notificationRepo.Update(ctx, notification); updateErr != nil {
				return updateErr
			}
			p.broadcastStatus(notification)
			logger.Error("notification failed permanently",
				"error", providerErr.Message,
			)
			return nil
		}
	}

	// Check retry count
	notification.IncrementRetry()
	if notification.RetryCount >= p.config.MaxCount {
		notification.MarkAsFailed("max retries exceeded")
		if updateErr := p.notificationRepo.Update(ctx, notification); updateErr != nil {
			return updateErr
		}
		p.broadcastStatus(notification)
		logger.Error("notification failed after max retries",
			"retry_count", notification.RetryCount,
		)
		return nil
	}

	// Calculate backoff delay
	delay := p.calculateBackoff(notification.RetryCount)

	// Update notification and re-queue with delay
	notification.Status = domain.StatusQueued
	if updateErr := p.notificationRepo.Update(ctx, notification); updateErr != nil {
		return updateErr
	}
	p.broadcastStatus(notification)

	logger.Warn("notification will be retried",
		"retry_count", notification.RetryCount,
		"delay", delay,
		"error", err,
	)

	// Wait for backoff delay before re-queueing
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
	}

	// Re-queue
	item := &domain.QueueItem{
		NotificationID: notification.ID,
		Channel:        notification.Channel,
		Priority:       notification.Priority,
		RetryCount:     notification.RetryCount,
	}

	return p.queue.Enqueue(ctx, item)
}

// calculateBackoff calculates exponential backoff delay
func (p *Processor) calculateBackoff(retryCount int) time.Duration {
	// Exponential backoff: baseDelay * 2^retryCount
	multiplier := math.Pow(2, float64(retryCount-1))
	delay := time.Duration(float64(p.config.BaseDelay) * multiplier)

	// Cap at 5 minutes
	maxDelay := 5 * time.Minute
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// broadcastStatus broadcasts status update via WebSocket
func (p *Processor) broadcastStatus(notification *domain.Notification) {
	if p.statusBroadcast != nil {
		p.statusBroadcast(notification)
	}
}
