package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/insider-one/notification-service/internal/domain"
)

// SchedulerService handles scheduled notification processing
type SchedulerService struct {
	notificationRepo domain.NotificationRepository
	queue            domain.Queue
	logger           *slog.Logger
	interval         time.Duration
	batchSize        int

	mu       sync.Mutex
	running  bool
	stopChan chan struct{}
}

// NewSchedulerService creates a new SchedulerService
func NewSchedulerService(
	notificationRepo domain.NotificationRepository,
	queue domain.Queue,
	logger *slog.Logger,
	interval time.Duration,
) *SchedulerService {
	return &SchedulerService{
		notificationRepo: notificationRepo,
		queue:            queue,
		logger:           logger,
		interval:         interval,
		batchSize:        100,
	}
}

// Start starts the scheduler
func (s *SchedulerService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.stopChan = make(chan struct{})
	s.mu.Unlock()

	s.logger.Info("scheduler started", "interval", s.interval)

	go s.run(ctx)
	return nil
}

// Stop stops the scheduler
func (s *SchedulerService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	close(s.stopChan)
	s.running = false
	s.logger.Info("scheduler stopped")
}

// run is the main scheduler loop
func (s *SchedulerService) run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Process immediately on start
	s.processScheduledNotifications(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.processScheduledNotifications(ctx)
		}
	}
}

// processScheduledNotifications processes notifications that are due
func (s *SchedulerService) processScheduledNotifications(ctx context.Context) {
	now := time.Now().UTC()

	notifications, err := s.notificationRepo.GetScheduledNotifications(ctx, now, s.batchSize)
	if err != nil {
		s.logger.Error("failed to get scheduled notifications", "error", err)
		return
	}

	if len(notifications) == 0 {
		return
	}

	s.logger.Info("processing scheduled notifications", "count", len(notifications))

	queueItems := make([]*domain.QueueItem, 0, len(notifications))
	for _, n := range notifications {
		queueItems = append(queueItems, &domain.QueueItem{
			NotificationID: n.ID,
			Channel:        n.Channel,
			Priority:       n.Priority,
			RetryCount:     n.RetryCount,
		})
	}

	// Enqueue notifications
	if err := s.queue.EnqueueBatch(ctx, queueItems); err != nil {
		s.logger.Error("failed to enqueue scheduled notifications", "error", err)
		return
	}

	// Update status to queued
	for _, n := range notifications {
		n.MarkAsQueued()
		if err := s.notificationRepo.Update(ctx, n); err != nil {
			s.logger.Error("failed to update notification status",
				"notification_id", n.ID,
				"error", err,
			)
		}
	}

	s.logger.Info("scheduled notifications queued", "count", len(notifications))
}
