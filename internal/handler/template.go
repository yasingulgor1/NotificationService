package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/insider-one/notification-service/internal/domain"
	"github.com/insider-one/notification-service/internal/service"
)

// TemplateHandler handles template HTTP requests
type TemplateHandler struct {
	service  *service.TemplateService
	validate *validator.Validate
}

// NewTemplateHandler creates a new TemplateHandler
func NewTemplateHandler(service *service.TemplateService) *TemplateHandler {
	return &TemplateHandler{
		service:  service,
		validate: validator.New(),
	}
}

// RegisterRoutes registers template routes
func (h *TemplateHandler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/{id}", h.GetByID)
	r.Get("/name/{name}", h.GetByName)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
	r.Post("/{name}/render", h.Render)
}

// CreateTemplateRequest represents a request to create a template
type CreateTemplateRequest struct {
	Name    string         `json:"name" validate:"required,min=1,max=100" example:"welcome_sms"`
	Channel domain.Channel `json:"channel" validate:"required,oneof=sms email push" example:"sms"`
	Content string         `json:"content" validate:"required" example:"Hello {{name}}, welcome to our service!"`
}

// Create creates a new template
// @Summary Create template
// @Description Create a new message template
// @Tags templates
// @Accept json
// @Produce json
// @Param template body CreateTemplateRequest true "Template request"
// @Success 201 {object} Response{data=domain.Template}
// @Failure 400 {object} Response
// @Failure 409 {object} Response
// @Failure 500 {object} Response
// @Router /api/v1/templates [post]
func (h *TemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateTemplateRequest
	if err := DecodeJSON(r, &req); err != nil {
		HandleError(w, err)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		JSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", err.Error())
		return
	}

	template, err := h.service.Create(r.Context(), service.CreateTemplateRequest{
		Name:    req.Name,
		Channel: req.Channel,
		Content: req.Content,
	})
	if err != nil {
		HandleError(w, err)
		return
	}

	JSON(w, http.StatusCreated, template)
}

// List retrieves all templates
// @Summary List templates
// @Description Get all message templates
// @Tags templates
// @Produce json
// @Success 200 {object} Response{data=[]domain.Template}
// @Failure 500 {object} Response
// @Router /api/v1/templates [get]
func (h *TemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	templates, err := h.service.List(r.Context())
	if err != nil {
		HandleError(w, err)
		return
	}

	JSON(w, http.StatusOK, templates)
}

// GetByID retrieves a template by ID
// @Summary Get template by ID
// @Description Get a template by its ID
// @Tags templates
// @Produce json
// @Param id path string true "Template ID"
// @Success 200 {object} Response{data=domain.Template}
// @Failure 404 {object} Response
// @Failure 500 {object} Response
// @Router /api/v1/templates/{id} [get]
func (h *TemplateHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_ID", "Invalid template ID", nil)
		return
	}

	template, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		HandleError(w, err)
		return
	}

	JSON(w, http.StatusOK, template)
}

// GetByName retrieves a template by name
// @Summary Get template by name
// @Description Get a template by its name
// @Tags templates
// @Produce json
// @Param name path string true "Template name"
// @Success 200 {object} Response{data=domain.Template}
// @Failure 404 {object} Response
// @Failure 500 {object} Response
// @Router /api/v1/templates/name/{name} [get]
func (h *TemplateHandler) GetByName(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	template, err := h.service.GetByName(r.Context(), name)
	if err != nil {
		HandleError(w, err)
		return
	}

	JSON(w, http.StatusOK, template)
}

// UpdateTemplateRequest represents a request to update a template
type UpdateTemplateRequest struct {
	Name    *string         `json:"name,omitempty"`
	Channel *domain.Channel `json:"channel,omitempty"`
	Content *string         `json:"content,omitempty"`
}

// Update updates a template
// @Summary Update template
// @Description Update an existing template
// @Tags templates
// @Accept json
// @Produce json
// @Param id path string true "Template ID"
// @Param template body UpdateTemplateRequest true "Update request"
// @Success 200 {object} Response{data=domain.Template}
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Failure 500 {object} Response
// @Router /api/v1/templates/{id} [put]
func (h *TemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_ID", "Invalid template ID", nil)
		return
	}

	var req UpdateTemplateRequest
	if err := DecodeJSON(r, &req); err != nil {
		HandleError(w, err)
		return
	}

	template, err := h.service.Update(r.Context(), id, service.UpdateTemplateRequest{
		Name:    req.Name,
		Channel: req.Channel,
		Content: req.Content,
	})
	if err != nil {
		HandleError(w, err)
		return
	}

	JSON(w, http.StatusOK, template)
}

// Delete deletes a template
// @Summary Delete template
// @Description Delete a template
// @Tags templates
// @Produce json
// @Param id path string true "Template ID"
// @Success 200 {object} Response
// @Failure 404 {object} Response
// @Failure 500 {object} Response
// @Router /api/v1/templates/{id} [delete]
func (h *TemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_ID", "Invalid template ID", nil)
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		HandleError(w, err)
		return
	}

	JSON(w, http.StatusOK, map[string]string{
		"message": "Template deleted successfully",
	})
}

// RenderRequest represents a request to render a template
type RenderRequest struct {
	Variables map[string]string `json:"variables"`
}

// Render renders a template with variables
// @Summary Render template
// @Description Render a template with provided variables
// @Tags templates
// @Accept json
// @Produce json
// @Param name path string true "Template name"
// @Param request body RenderRequest true "Variables"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Failure 500 {object} Response
// @Router /api/v1/templates/{name}/render [post]
func (h *TemplateHandler) Render(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var req RenderRequest
	if err := DecodeJSON(r, &req); err != nil {
		HandleError(w, err)
		return
	}

	content, err := h.service.Render(r.Context(), name, req.Variables)
	if err != nil {
		HandleError(w, err)
		return
	}

	JSON(w, http.StatusOK, map[string]string{
		"content": content,
	})
}
