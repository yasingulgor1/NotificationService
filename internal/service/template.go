package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/insider-one/notification-service/internal/domain"
)

// TemplateService handles template business logic
type TemplateService struct {
	repo   domain.TemplateRepository
	logger *slog.Logger
}

// NewTemplateService creates a new TemplateService
func NewTemplateService(repo domain.TemplateRepository, logger *slog.Logger) *TemplateService {
	return &TemplateService{
		repo:   repo,
		logger: logger,
	}
}

// CreateTemplateRequest represents a request to create a template
type CreateTemplateRequest struct {
	Name    string         `json:"name" validate:"required,min=1,max=100"`
	Channel domain.Channel `json:"channel" validate:"required"`
	Content string         `json:"content" validate:"required"`
}

// UpdateTemplateRequest represents a request to update a template
type UpdateTemplateRequest struct {
	Name    *string         `json:"name,omitempty"`
	Channel *domain.Channel `json:"channel,omitempty"`
	Content *string         `json:"content,omitempty"`
}

// Create creates a new template
func (s *TemplateService) Create(ctx context.Context, req CreateTemplateRequest) (*domain.Template, error) {
	// Validate channel
	if !req.Channel.IsValid() {
		return nil, domain.NewValidationError("channel", "invalid channel")
	}

	// Check if template with same name exists
	existing, err := s.repo.GetByName(ctx, req.Name)
	if err == nil && existing != nil {
		return nil, domain.ErrAlreadyExists
	}
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("failed to check existing template: %w", err)
	}

	// Create template
	template := domain.NewTemplate(req.Name, req.Channel, req.Content)

	if err := s.repo.Create(ctx, template); err != nil {
		return nil, fmt.Errorf("failed to create template: %w", err)
	}

	s.logger.Info("template created",
		"template_id", template.ID,
		"name", template.Name,
	)

	return template, nil
}

// GetByID retrieves a template by ID
func (s *TemplateService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Template, error) {
	return s.repo.GetByID(ctx, id)
}

// GetByName retrieves a template by name
func (s *TemplateService) GetByName(ctx context.Context, name string) (*domain.Template, error) {
	return s.repo.GetByName(ctx, name)
}

// List retrieves all templates
func (s *TemplateService) List(ctx context.Context) ([]*domain.Template, error) {
	return s.repo.List(ctx)
}

// Update updates an existing template
func (s *TemplateService) Update(ctx context.Context, id uuid.UUID, req UpdateTemplateRequest) (*domain.Template, error) {
	template, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		// Check if new name conflicts with existing template
		existing, err := s.repo.GetByName(ctx, *req.Name)
		if err == nil && existing != nil && existing.ID != id {
			return nil, domain.ErrAlreadyExists
		}
		template.Name = *req.Name
	}

	if req.Channel != nil {
		if !req.Channel.IsValid() {
			return nil, domain.NewValidationError("channel", "invalid channel")
		}
		template.Channel = *req.Channel
	}

	if req.Content != nil {
		template.Content = *req.Content
		template.ExtractVariables()
	}

	if err := s.repo.Update(ctx, template); err != nil {
		return nil, fmt.Errorf("failed to update template: %w", err)
	}

	s.logger.Info("template updated",
		"template_id", template.ID,
	)

	return template, nil
}

// Delete deletes a template
func (s *TemplateService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	s.logger.Info("template deleted",
		"template_id", id,
	)

	return nil
}

// Render renders a template with variables
func (s *TemplateService) Render(ctx context.Context, name string, vars map[string]string) (string, error) {
	template, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return "", err
	}

	// Validate variables
	missing := template.Validate(vars)
	if len(missing) > 0 {
		return "", fmt.Errorf("%w: %v", domain.ErrMissingVariables, missing)
	}

	return template.Render(vars), nil
}
