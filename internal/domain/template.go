package domain

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Template represents a message template
type Template struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Channel   Channel   `json:"channel"`
	Content   string    `json:"content"`
	Variables []string  `json:"variables"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// variablePattern matches template variables like {{variable_name}}
var variablePattern = regexp.MustCompile(`\{\{(\w+)\}\}`)

// NewTemplate creates a new template
func NewTemplate(name string, channel Channel, content string) *Template {
	now := time.Now().UTC()
	t := &Template{
		ID:        uuid.New(),
		Name:      name,
		Channel:   channel,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}
	t.ExtractVariables()
	return t
}

// ExtractVariables extracts variable names from the template content
func (t *Template) ExtractVariables() {
	matches := variablePattern.FindAllStringSubmatch(t.Content, -1)
	seen := make(map[string]bool)
	variables := make([]string, 0)

	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			variables = append(variables, match[1])
			seen[match[1]] = true
		}
	}
	t.Variables = variables
}

// Render renders the template with the given variables
func (t *Template) Render(vars map[string]string) string {
	result := t.Content
	for key, value := range vars {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// Validate checks if all required variables are provided
func (t *Template) Validate(vars map[string]string) []string {
	missing := make([]string, 0)
	for _, v := range t.Variables {
		if _, ok := vars[v]; !ok {
			missing = append(missing, v)
		}
	}
	return missing
}

// TemplateRepository defines the interface for template persistence
type TemplateRepository interface {
	Create(ctx context.Context, template *Template) error
	GetByID(ctx context.Context, id uuid.UUID) (*Template, error)
	GetByName(ctx context.Context, name string) (*Template, error)
	List(ctx context.Context) ([]*Template, error)
	Update(ctx context.Context, template *Template) error
	Delete(ctx context.Context, id uuid.UUID) error
}
