package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/insider-one/notification-service/internal/domain"
)

// NotificationRepository implements domain.NotificationRepository using PostgreSQL
type NotificationRepository struct {
	db *DB
}

// NewNotificationRepository creates a new NotificationRepository
func NewNotificationRepository(db *DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

// Create creates a new notification
func (r *NotificationRepository) Create(ctx context.Context, n *domain.Notification) error {
	metadata, err := json.Marshal(n.Metadata)
	if err != nil {
		metadata = []byte("{}")
	}

	query := `
		INSERT INTO notifications (
			id, batch_id, recipient, channel, content, priority, status,
			scheduled_at, sent_at, external_id, retry_count, idempotency_key,
			metadata, error_message, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		)
	`

	_, err = r.db.Pool.Exec(ctx, query,
		n.ID, n.BatchID, n.Recipient, n.Channel, n.Content, n.Priority, n.Status,
		n.ScheduledAt, n.SentAt, n.ExternalID, n.RetryCount, n.IdempotencyKey,
		metadata, n.ErrorMessage, n.CreatedAt, n.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") && strings.Contains(err.Error(), "idempotency_key") {
			return domain.ErrIdempotencyConflict
		}
		return fmt.Errorf("failed to create notification: %w", err)
	}

	return nil
}

// CreateBatch creates multiple notifications in a single transaction
func (r *NotificationRepository) CreateBatch(ctx context.Context, notifications []*domain.Notification) error {
	if len(notifications) == 0 {
		return nil
	}

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO notifications (
			id, batch_id, recipient, channel, content, priority, status,
			scheduled_at, sent_at, external_id, retry_count, idempotency_key,
			metadata, error_message, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		)
	`

	for _, n := range notifications {
		metadata, err := json.Marshal(n.Metadata)
		if err != nil {
			metadata = []byte("{}")
		}

		_, err = tx.Exec(ctx, query,
			n.ID, n.BatchID, n.Recipient, n.Channel, n.Content, n.Priority, n.Status,
			n.ScheduledAt, n.SentAt, n.ExternalID, n.RetryCount, n.IdempotencyKey,
			metadata, n.ErrorMessage, n.CreatedAt, n.UpdatedAt,
		)
		if err != nil {
			if strings.Contains(err.Error(), "duplicate key") && strings.Contains(err.Error(), "idempotency_key") {
				return domain.ErrIdempotencyConflict
			}
			return fmt.Errorf("failed to create notification in batch: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetByID retrieves a notification by ID
func (r *NotificationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	query := `
		SELECT id, batch_id, recipient, channel, content, priority, status,
			scheduled_at, sent_at, external_id, retry_count, idempotency_key,
			metadata, error_message, created_at, updated_at
		FROM notifications
		WHERE id = $1
	`

	return r.scanNotification(ctx, query, id)
}

// GetByBatchID retrieves all notifications in a batch
func (r *NotificationRepository) GetByBatchID(ctx context.Context, batchID uuid.UUID) ([]*domain.Notification, error) {
	query := `
		SELECT id, batch_id, recipient, channel, content, priority, status,
			scheduled_at, sent_at, external_id, retry_count, idempotency_key,
			metadata, error_message, created_at, updated_at
		FROM notifications
		WHERE batch_id = $1
		ORDER BY created_at ASC
	`

	return r.scanNotifications(ctx, query, batchID)
}

// GetByIdempotencyKey retrieves a notification by idempotency key
func (r *NotificationRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Notification, error) {
	query := `
		SELECT id, batch_id, recipient, channel, content, priority, status,
			scheduled_at, sent_at, external_id, retry_count, idempotency_key,
			metadata, error_message, created_at, updated_at
		FROM notifications
		WHERE idempotency_key = $1
	`

	return r.scanNotification(ctx, query, key)
}

// Update updates an existing notification
func (r *NotificationRepository) Update(ctx context.Context, n *domain.Notification) error {
	metadata, err := json.Marshal(n.Metadata)
	if err != nil {
		metadata = []byte("{}")
	}

	query := `
		UPDATE notifications SET
			batch_id = $2, recipient = $3, channel = $4, content = $5,
			priority = $6, status = $7, scheduled_at = $8, sent_at = $9,
			external_id = $10, retry_count = $11, idempotency_key = $12,
			metadata = $13, error_message = $14
		WHERE id = $1
	`

	result, err := r.db.Pool.Exec(ctx, query,
		n.ID, n.BatchID, n.Recipient, n.Channel, n.Content, n.Priority, n.Status,
		n.ScheduledAt, n.SentAt, n.ExternalID, n.RetryCount, n.IdempotencyKey,
		metadata, n.ErrorMessage,
	)
	if err != nil {
		return fmt.Errorf("failed to update notification: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// Delete deletes a notification
func (r *NotificationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM notifications WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete notification: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// List lists notifications with filters and pagination
func (r *NotificationRepository) List(ctx context.Context, filter domain.NotificationFilter) (*domain.NotificationListResult, error) {
	// Build the WHERE clause
	conditions := []string{"1=1"}
	args := []any{}
	argIndex := 1

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, *filter.Status)
		argIndex++
	}

	if filter.Channel != nil {
		conditions = append(conditions, fmt.Sprintf("channel = $%d", argIndex))
		args = append(args, *filter.Channel)
		argIndex++
	}

	if filter.BatchID != nil {
		conditions = append(conditions, fmt.Sprintf("batch_id = $%d", argIndex))
		args = append(args, *filter.BatchID)
		argIndex++
	}

	if filter.StartDate != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, *filter.StartDate)
		argIndex++
	}

	if filter.EndDate != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, *filter.EndDate)
		argIndex++
	}

	whereClause := strings.Join(conditions, " AND ")

	// Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM notifications WHERE %s", whereClause)
	var total int64
	if err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count notifications: %w", err)
	}

	// Apply pagination
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	// Get notifications
	query := fmt.Sprintf(`
		SELECT id, batch_id, recipient, channel, content, priority, status,
			scheduled_at, sent_at, external_id, retry_count, idempotency_key,
			metadata, error_message, created_at, updated_at
		FROM notifications
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, pageSize, offset)
	notifications, err := r.scanNotifications(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	return &domain.NotificationListResult{
		Notifications: notifications,
		Total:         total,
		Page:          page,
		PageSize:      pageSize,
		TotalPages:    totalPages,
	}, nil
}

// GetScheduledNotifications retrieves scheduled notifications ready to be sent
func (r *NotificationRepository) GetScheduledNotifications(ctx context.Context, before time.Time, limit int) ([]*domain.Notification, error) {
	query := `
		SELECT id, batch_id, recipient, channel, content, priority, status,
			scheduled_at, sent_at, external_id, retry_count, idempotency_key,
			metadata, error_message, created_at, updated_at
		FROM notifications
		WHERE status = 'scheduled' AND scheduled_at <= $1
		ORDER BY scheduled_at ASC
		LIMIT $2
	`

	return r.scanNotifications(ctx, query, before, limit)
}

// UpdateStatus updates only the status of a notification
func (r *NotificationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status) error {
	query := `UPDATE notifications SET status = $2 WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("failed to update notification status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// Helper functions

func (r *NotificationRepository) scanNotification(ctx context.Context, query string, args ...any) (*domain.Notification, error) {
	row := r.db.Pool.QueryRow(ctx, query, args...)

	n := &domain.Notification{}
	var metadata []byte

	err := row.Scan(
		&n.ID, &n.BatchID, &n.Recipient, &n.Channel, &n.Content, &n.Priority, &n.Status,
		&n.ScheduledAt, &n.SentAt, &n.ExternalID, &n.RetryCount, &n.IdempotencyKey,
		&metadata, &n.ErrorMessage, &n.CreatedAt, &n.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to scan notification: %w", err)
	}

	if len(metadata) > 0 {
		json.Unmarshal(metadata, &n.Metadata)
	}

	return n, nil
}

func (r *NotificationRepository) scanNotifications(ctx context.Context, query string, args ...any) ([]*domain.Notification, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query notifications: %w", err)
	}
	defer rows.Close()

	notifications := make([]*domain.Notification, 0)
	for rows.Next() {
		n := &domain.Notification{}
		var metadata []byte

		err := rows.Scan(
			&n.ID, &n.BatchID, &n.Recipient, &n.Channel, &n.Content, &n.Priority, &n.Status,
			&n.ScheduledAt, &n.SentAt, &n.ExternalID, &n.RetryCount, &n.IdempotencyKey,
			&metadata, &n.ErrorMessage, &n.CreatedAt, &n.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan notification: %w", err)
		}

		if len(metadata) > 0 {
			json.Unmarshal(metadata, &n.Metadata)
		}

		notifications = append(notifications, n)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating notifications: %w", err)
	}

	return notifications, nil
}
