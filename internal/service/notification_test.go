package service

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/insider-one/notification-service/internal/domain"
)

// MockNotificationRepository is a mock implementation of domain.NotificationRepository
type MockNotificationRepository struct {
	mock.Mock
}

func (m *MockNotificationRepository) Create(ctx context.Context, n *domain.Notification) error {
	args := m.Called(ctx, n)
	return args.Error(0)
}

func (m *MockNotificationRepository) CreateBatch(ctx context.Context, notifications []*domain.Notification) error {
	args := m.Called(ctx, notifications)
	return args.Error(0)
}

func (m *MockNotificationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Notification), args.Error(1)
}

func (m *MockNotificationRepository) GetByBatchID(ctx context.Context, batchID uuid.UUID) ([]*domain.Notification, error) {
	args := m.Called(ctx, batchID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Notification), args.Error(1)
}

func (m *MockNotificationRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Notification, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Notification), args.Error(1)
}

func (m *MockNotificationRepository) Update(ctx context.Context, n *domain.Notification) error {
	args := m.Called(ctx, n)
	return args.Error(0)
}

func (m *MockNotificationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockNotificationRepository) List(ctx context.Context, filter domain.NotificationFilter) (*domain.NotificationListResult, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.NotificationListResult), args.Error(1)
}

func (m *MockNotificationRepository) GetScheduledNotifications(ctx context.Context, before time.Time, limit int) ([]*domain.Notification, error) {
	args := m.Called(ctx, before, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Notification), args.Error(1)
}

func (m *MockNotificationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

// MockTemplateRepository is a mock implementation of domain.TemplateRepository
type MockTemplateRepository struct {
	mock.Mock
}

func (m *MockTemplateRepository) Create(ctx context.Context, t *domain.Template) error {
	args := m.Called(ctx, t)
	return args.Error(0)
}

func (m *MockTemplateRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Template, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Template), args.Error(1)
}

func (m *MockTemplateRepository) GetByName(ctx context.Context, name string) (*domain.Template, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Template), args.Error(1)
}

func (m *MockTemplateRepository) List(ctx context.Context) ([]*domain.Template, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Template), args.Error(1)
}

func (m *MockTemplateRepository) Update(ctx context.Context, t *domain.Template) error {
	args := m.Called(ctx, t)
	return args.Error(0)
}

func (m *MockTemplateRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockQueue is a mock implementation of domain.Queue
type MockQueue struct {
	mock.Mock
}

func (m *MockQueue) Enqueue(ctx context.Context, item *domain.QueueItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockQueue) EnqueueBatch(ctx context.Context, items []*domain.QueueItem) error {
	args := m.Called(ctx, items)
	return args.Error(0)
}

func (m *MockQueue) Dequeue(ctx context.Context, channel domain.Channel) (*domain.QueueItem, error) {
	args := m.Called(ctx, channel)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.QueueItem), args.Error(1)
}

func (m *MockQueue) GetQueueDepth(ctx context.Context, channel domain.Channel) (int64, error) {
	args := m.Called(ctx, channel)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQueue) GetAllQueueDepths(ctx context.Context) (map[domain.Channel]int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[domain.Channel]int64), args.Error(1)
}

func TestNotificationService_Create(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	mockRepo := new(MockNotificationRepository)
	mockTemplateRepo := new(MockTemplateRepository)
	mockQueue := new(MockQueue)

	service := NewNotificationService(mockRepo, mockTemplateRepo, mockQueue, logger)

	t.Run("create notification successfully", func(t *testing.T) {
		mockRepo.On("Create", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil).Once()
		mockQueue.On("Enqueue", ctx, mock.AnythingOfType("*domain.QueueItem")).Return(nil).Once()
		mockRepo.On("Update", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil).Once()

		req := CreateRequest{
			Recipient: "+905551234567",
			Channel:   domain.ChannelSMS,
			Content:   "Test message",
			Priority:  domain.PriorityHigh,
		}

		notification, err := service.Create(ctx, req)

		assert.NoError(t, err)
		assert.NotNil(t, notification)
		assert.Equal(t, req.Recipient, notification.Recipient)
		assert.Equal(t, req.Channel, notification.Channel)
		assert.Equal(t, req.Content, notification.Content)
		assert.Equal(t, req.Priority, notification.Priority)
	})

	t.Run("create notification with idempotency key returns existing", func(t *testing.T) {
		existingNotification := domain.NewNotification("+905551234567", domain.ChannelSMS, "Existing")
		idempotencyKey := "unique-key"

		mockRepo.On("GetByIdempotencyKey", ctx, idempotencyKey).Return(existingNotification, nil).Once()

		req := CreateRequest{
			Recipient:      "+905551234567",
			Channel:        domain.ChannelSMS,
			Content:        "New message",
			IdempotencyKey: &idempotencyKey,
		}

		notification, err := service.Create(ctx, req)

		assert.NoError(t, err)
		assert.Equal(t, existingNotification, notification)
	})

	t.Run("create notification with invalid channel", func(t *testing.T) {
		req := CreateRequest{
			Recipient: "+905551234567",
			Channel:   domain.Channel("invalid"),
			Content:   "Test message",
		}

		notification, err := service.Create(ctx, req)

		assert.Error(t, err)
		assert.Nil(t, notification)
	})

	t.Run("create notification without content", func(t *testing.T) {
		req := CreateRequest{
			Recipient: "+905551234567",
			Channel:   domain.ChannelSMS,
			Content:   "",
		}

		notification, err := service.Create(ctx, req)

		assert.Error(t, err)
		assert.Nil(t, notification)
	})
}

func TestNotificationService_Cancel(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	mockRepo := new(MockNotificationRepository)
	mockTemplateRepo := new(MockTemplateRepository)
	mockQueue := new(MockQueue)

	service := NewNotificationService(mockRepo, mockTemplateRepo, mockQueue, logger)

	t.Run("cancel pending notification", func(t *testing.T) {
		id := uuid.New()
		notification := domain.NewNotification("+905551234567", domain.ChannelSMS, "Test")
		notification.ID = id
		notification.Status = domain.StatusPending

		mockRepo.On("GetByID", ctx, id).Return(notification, nil).Once()
		mockRepo.On("Update", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil).Once()

		err := service.Cancel(ctx, id)

		assert.NoError(t, err)
	})

	t.Run("cannot cancel sent notification", func(t *testing.T) {
		id := uuid.New()
		notification := domain.NewNotification("+905551234567", domain.ChannelSMS, "Test")
		notification.ID = id
		notification.Status = domain.StatusSent

		mockRepo.On("GetByID", ctx, id).Return(notification, nil).Once()

		err := service.Cancel(ctx, id)

		assert.Error(t, err)
		assert.Equal(t, domain.ErrCannotCancel, err)
	})

	t.Run("cancel non-existent notification", func(t *testing.T) {
		id := uuid.New()

		mockRepo.On("GetByID", ctx, id).Return(nil, domain.ErrNotFound).Once()

		err := service.Cancel(ctx, id)

		assert.Error(t, err)
		assert.Equal(t, domain.ErrNotFound, err)
	})
}
