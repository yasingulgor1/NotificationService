package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/insider-one/notification-service/internal/domain"
	"github.com/insider-one/notification-service/internal/service"
)

// NotificationHandler handles notification HTTP requests
type NotificationHandler struct {
	service  *service.NotificationService
	validate *validator.Validate
}

// NewNotificationHandler creates a new NotificationHandler
func NewNotificationHandler(service *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{
		service:  service,
		validate: validator.New(),
	}
}

// RegisterRoutes registers notification routes
func (h *NotificationHandler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.Create)
	r.Post("/batch", h.CreateBatch)
	r.Get("/", h.List)
	r.Get("/{id}", h.GetByID)
	r.Get("/batch/{batchId}", h.GetByBatchID)
	r.Delete("/{id}", h.Cancel)
}

// CreateNotificationRequest represents a request to create a notification
// @Description Request to create a notification
type CreateNotificationRequest struct {
	Recipient      string            `json:"recipient" validate:"required" example:"+905551234567"`
	Channel        domain.Channel    `json:"channel" validate:"required,oneof=sms email push" example:"sms"`
	Content        string            `json:"content" example:"Your verification code is 123456"`
	Priority       domain.Priority   `json:"priority" validate:"omitempty,oneof=high normal low" example:"normal"`
	ScheduledAt    *time.Time        `json:"scheduled_at,omitempty"`
	IdempotencyKey *string           `json:"idempotency_key,omitempty" example:"unique-key-123"`
	Metadata       map[string]any    `json:"metadata,omitempty"`
	TemplateName   *string           `json:"template_name,omitempty" example:"welcome_sms"`
	TemplateVars   map[string]string `json:"template_vars,omitempty"`
}

// Create creates a single notification
// @Summary Create notification
// @Description Create a new notification
// @Tags notifications
// @Accept json
// @Produce json
// @Param notification body CreateNotificationRequest true "Notification request"
// @Success 201 {object} Response{data=domain.Notification}
// @Failure 400 {object} Response
// @Failure 500 {object} Response
// @Router /api/v1/notifications [post]
func (h *NotificationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateNotificationRequest
	if err := DecodeJSON(r, &req); err != nil {
		HandleError(w, err)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		JSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", err.Error())
		return
	}

	notification, err := h.service.Create(r.Context(), service.CreateRequest{
		Recipient:      req.Recipient,
		Channel:        req.Channel,
		Content:        req.Content,
		Priority:       req.Priority,
		ScheduledAt:    req.ScheduledAt,
		IdempotencyKey: req.IdempotencyKey,
		Metadata:       req.Metadata,
		TemplateName:   req.TemplateName,
		TemplateVars:   req.TemplateVars,
	})
	if err != nil {
		HandleError(w, err)
		return
	}

	JSON(w, http.StatusCreated, notification)
}

// BatchCreateRequest represents a request to create multiple notifications
type BatchCreateRequest struct {
	Notifications []CreateNotificationRequest `json:"notifications" validate:"required,min=1,max=1000,dive"`
}

// CreateBatch creates multiple notifications
// @Summary Create batch of notifications
// @Description Create multiple notifications in a single request (max 1000)
// @Tags notifications
// @Accept json
// @Produce json
// @Param notifications body BatchCreateRequest true "Batch notification request"
// @Success 201 {object} Response{data=[]domain.Notification}
// @Failure 400 {object} Response
// @Failure 500 {object} Response
// @Router /api/v1/notifications/batch [post]
func (h *NotificationHandler) CreateBatch(w http.ResponseWriter, r *http.Request) {
	var req BatchCreateRequest
	if err := DecodeJSON(r, &req); err != nil {
		HandleError(w, err)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		JSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", err.Error())
		return
	}

	createRequests := make([]service.CreateRequest, len(req.Notifications))
	for i, n := range req.Notifications {
		createRequests[i] = service.CreateRequest{
			Recipient:      n.Recipient,
			Channel:        n.Channel,
			Content:        n.Content,
			Priority:       n.Priority,
			ScheduledAt:    n.ScheduledAt,
			IdempotencyKey: n.IdempotencyKey,
			Metadata:       n.Metadata,
			TemplateName:   n.TemplateName,
			TemplateVars:   n.TemplateVars,
		}
	}

	notifications, err := h.service.CreateBatch(r.Context(), service.BatchCreateRequest{
		Notifications: createRequests,
	})
	if err != nil {
		HandleError(w, err)
		return
	}

	JSON(w, http.StatusCreated, map[string]any{
		"batch_id":      notifications[0].BatchID,
		"count":         len(notifications),
		"notifications": notifications,
	})
}

// GetByID retrieves a notification by ID
// @Summary Get notification by ID
// @Description Get a notification by its ID
// @Tags notifications
// @Produce json
// @Param id path string true "Notification ID"
// @Success 200 {object} Response{data=domain.Notification}
// @Failure 404 {object} Response
// @Failure 500 {object} Response
// @Router /api/v1/notifications/{id} [get]
func (h *NotificationHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_ID", "Invalid notification ID", nil)
		return
	}

	notification, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		HandleError(w, err)
		return
	}

	JSON(w, http.StatusOK, notification)
}

// GetByBatchID retrieves all notifications in a batch
// @Summary Get notifications by batch ID
// @Description Get all notifications in a batch
// @Tags notifications
// @Produce json
// @Param batchId path string true "Batch ID"
// @Success 200 {object} Response{data=[]domain.Notification}
// @Failure 404 {object} Response
// @Failure 500 {object} Response
// @Router /api/v1/notifications/batch/{batchId} [get]
func (h *NotificationHandler) GetByBatchID(w http.ResponseWriter, r *http.Request) {
	batchIDStr := chi.URLParam(r, "batchId")
	batchID, err := uuid.Parse(batchIDStr)
	if err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_ID", "Invalid batch ID", nil)
		return
	}

	notifications, err := h.service.GetByBatchID(r.Context(), batchID)
	if err != nil {
		HandleError(w, err)
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"batch_id":      batchID,
		"count":         len(notifications),
		"notifications": notifications,
	})
}

// Cancel cancels a pending notification
// @Summary Cancel notification
// @Description Cancel a pending notification
// @Tags notifications
// @Produce json
// @Param id path string true "Notification ID"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Failure 500 {object} Response
// @Router /api/v1/notifications/{id} [delete]
func (h *NotificationHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_ID", "Invalid notification ID", nil)
		return
	}

	if err := h.service.Cancel(r.Context(), id); err != nil {
		HandleError(w, err)
		return
	}

	JSON(w, http.StatusOK, map[string]string{
		"message": "Notification cancelled successfully",
	})
}

// List lists notifications with filters
// @Summary List notifications
// @Description List notifications with optional filters and pagination
// @Tags notifications
// @Produce json
// @Param status query string false "Filter by status"
// @Param channel query string false "Filter by channel"
// @Param batch_id query string false "Filter by batch ID"
// @Param start_date query string false "Filter by start date (RFC3339)"
// @Param end_date query string false "Filter by end date (RFC3339)"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} Response{data=domain.NotificationListResult}
// @Failure 400 {object} Response
// @Failure 500 {object} Response
// @Router /api/v1/notifications [get]
func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := domain.NotificationFilter{
		Page:     1,
		PageSize: 20,
	}

	// Parse status
	if status := r.URL.Query().Get("status"); status != "" {
		s := domain.Status(status)
		filter.Status = &s
	}

	// Parse channel
	if channel := r.URL.Query().Get("channel"); channel != "" {
		c := domain.Channel(channel)
		if !c.IsValid() {
			JSONError(w, http.StatusBadRequest, "INVALID_CHANNEL", "Invalid channel", nil)
			return
		}
		filter.Channel = &c
	}

	// Parse batch_id
	if batchIDStr := r.URL.Query().Get("batch_id"); batchIDStr != "" {
		batchID, err := uuid.Parse(batchIDStr)
		if err != nil {
			JSONError(w, http.StatusBadRequest, "INVALID_BATCH_ID", "Invalid batch ID", nil)
			return
		}
		filter.BatchID = &batchID
	}

	// Parse dates
	if startDateStr := r.URL.Query().Get("start_date"); startDateStr != "" {
		startDate, err := time.Parse(time.RFC3339, startDateStr)
		if err != nil {
			JSONError(w, http.StatusBadRequest, "INVALID_START_DATE", "Invalid start date format (use RFC3339)", nil)
			return
		}
		filter.StartDate = &startDate
	}

	if endDateStr := r.URL.Query().Get("end_date"); endDateStr != "" {
		endDate, err := time.Parse(time.RFC3339, endDateStr)
		if err != nil {
			JSONError(w, http.StatusBadRequest, "INVALID_END_DATE", "Invalid end date format (use RFC3339)", nil)
			return
		}
		filter.EndDate = &endDate
	}

	// Parse pagination
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			JSONError(w, http.StatusBadRequest, "INVALID_PAGE", "Invalid page number", nil)
			return
		}
		filter.Page = page
	}

	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		pageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || pageSize < 1 || pageSize > 100 {
			JSONError(w, http.StatusBadRequest, "INVALID_PAGE_SIZE", "Page size must be between 1 and 100", nil)
			return
		}
		filter.PageSize = pageSize
	}

	result, err := h.service.List(r.Context(), filter)
	if err != nil {
		HandleError(w, err)
		return
	}

	JSON(w, http.StatusOK, result)
}
