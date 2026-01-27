package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/insider-one/notification-service/internal/domain"
)

// TemplateRepository implements domain.TemplateRepository using PostgreSQL
type TemplateRepository struct {
	db *DB
}

// NewTemplateRepository creates a new TemplateRepository
func NewTemplateRepository(db *DB) *TemplateRepository {
	return &TemplateRepository{db: db}
}

// Create creates a new template
func (r *TemplateRepository) Create(ctx context.Context, t *domain.Template) error {
	variables, err := json.Marshal(t.Variables)
	if err != nil {
		variables = []byte("[]")
	}

	query := `
		INSERT INTO templates (id, name, channel, content, variables, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = r.db.Pool.Exec(ctx, query,
		t.ID, t.Name, t.Channel, t.Content, variables, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("failed to create template: %w", err)
	}

	return nil
}

// GetByID retrieves a template by ID
func (r *TemplateRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Template, error) {
	query := `
		SELECT id, name, channel, content, variables, created_at, updated_at
		FROM templates
		WHERE id = $1
	`

	return r.scanTemplate(ctx, query, id)
}

// GetByName retrieves a template by name
func (r *TemplateRepository) GetByName(ctx context.Context, name string) (*domain.Template, error) {
	query := `
		SELECT id, name, channel, content, variables, created_at, updated_at
		FROM templates
		WHERE name = $1
	`

	return r.scanTemplate(ctx, query, name)
}

// List retrieves all templates
func (r *TemplateRepository) List(ctx context.Context) ([]*domain.Template, error) {
	query := `
		SELECT id, name, channel, content, variables, created_at, updated_at
		FROM templates
		ORDER BY name ASC
	`

	return r.scanTemplates(ctx, query)
}

// Update updates an existing template
func (r *TemplateRepository) Update(ctx context.Context, t *domain.Template) error {
	variables, err := json.Marshal(t.Variables)
	if err != nil {
		variables = []byte("[]")
	}

	query := `
		UPDATE templates SET
			name = $2, channel = $3, content = $4, variables = $5
		WHERE id = $1
	`

	result, err := r.db.Pool.Exec(ctx, query,
		t.ID, t.Name, t.Channel, t.Content, variables,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("failed to update template: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// Delete deletes a template
func (r *TemplateRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM templates WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// Helper functions

func (r *TemplateRepository) scanTemplate(ctx context.Context, query string, args ...any) (*domain.Template, error) {
	row := r.db.Pool.QueryRow(ctx, query, args...)

	t := &domain.Template{}
	var variables []byte

	err := row.Scan(
		&t.ID, &t.Name, &t.Channel, &t.Content, &variables, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to scan template: %w", err)
	}

	if len(variables) > 0 {
		json.Unmarshal(variables, &t.Variables)
	}

	return t, nil
}

func (r *TemplateRepository) scanTemplates(ctx context.Context, query string, args ...any) ([]*domain.Template, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query templates: %w", err)
	}
	defer rows.Close()

	templates := make([]*domain.Template, 0)
	for rows.Next() {
		t := &domain.Template{}
		var variables []byte

		err := rows.Scan(
			&t.ID, &t.Name, &t.Channel, &t.Content, &variables, &t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan template: %w", err)
		}

		if len(variables) > 0 {
			json.Unmarshal(variables, &t.Variables)
		}

		templates = append(templates, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating templates: %w", err)
	}

	return templates, nil
}
