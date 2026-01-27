package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTemplate(t *testing.T) {
	name := "welcome_sms"
	channel := ChannelSMS
	content := "Hello {{name}}, welcome to {{company}}!"

	tmpl := NewTemplate(name, channel, content)

	assert.NotNil(t, tmpl)
	assert.NotEmpty(t, tmpl.ID)
	assert.Equal(t, name, tmpl.Name)
	assert.Equal(t, channel, tmpl.Channel)
	assert.Equal(t, content, tmpl.Content)
	assert.Contains(t, tmpl.Variables, "name")
	assert.Contains(t, tmpl.Variables, "company")
	assert.Len(t, tmpl.Variables, 2)
}

func TestTemplate_ExtractVariables(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantVars []string
	}{
		{
			name:     "single variable",
			content:  "Hello {{name}}!",
			wantVars: []string{"name"},
		},
		{
			name:     "multiple variables",
			content:  "Hello {{name}}, your code is {{code}}",
			wantVars: []string{"name", "code"},
		},
		{
			name:     "duplicate variables",
			content:  "{{name}} said hello to {{name}}",
			wantVars: []string{"name"},
		},
		{
			name:     "no variables",
			content:  "Hello World!",
			wantVars: []string{},
		},
		{
			name:     "underscore in variable name",
			content:  "Hello {{first_name}} {{last_name}}",
			wantVars: []string{"first_name", "last_name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := &Template{Content: tt.content}
			tmpl.ExtractVariables()

			assert.Len(t, tmpl.Variables, len(tt.wantVars))
			for _, v := range tt.wantVars {
				assert.Contains(t, tmpl.Variables, v)
			}
		})
	}
}

func TestTemplate_Render(t *testing.T) {
	tests := []struct {
		name    string
		content string
		vars    map[string]string
		want    string
	}{
		{
			name:    "render single variable",
			content: "Hello {{name}}!",
			vars:    map[string]string{"name": "John"},
			want:    "Hello John!",
		},
		{
			name:    "render multiple variables",
			content: "Hello {{name}}, your code is {{code}}",
			vars:    map[string]string{"name": "John", "code": "123456"},
			want:    "Hello John, your code is 123456",
		},
		{
			name:    "render with missing variable",
			content: "Hello {{name}}, {{greeting}}",
			vars:    map[string]string{"name": "John"},
			want:    "Hello John, {{greeting}}",
		},
		{
			name:    "render duplicate variables",
			content: "{{name}} said hello to {{name}}",
			vars:    map[string]string{"name": "John"},
			want:    "John said hello to John",
		},
		{
			name:    "render with no variables",
			content: "Hello World!",
			vars:    map[string]string{},
			want:    "Hello World!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := &Template{Content: tt.content}
			tmpl.ExtractVariables()
			result := tmpl.Render(tt.vars)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestTemplate_Validate(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		vars        map[string]string
		wantMissing []string
	}{
		{
			name:        "all variables provided",
			content:     "Hello {{name}}, your code is {{code}}",
			vars:        map[string]string{"name": "John", "code": "123456"},
			wantMissing: []string{},
		},
		{
			name:        "missing one variable",
			content:     "Hello {{name}}, your code is {{code}}",
			vars:        map[string]string{"name": "John"},
			wantMissing: []string{"code"},
		},
		{
			name:        "missing all variables",
			content:     "Hello {{name}}, your code is {{code}}",
			vars:        map[string]string{},
			wantMissing: []string{"name", "code"},
		},
		{
			name:        "no variables in template",
			content:     "Hello World!",
			vars:        map[string]string{},
			wantMissing: []string{},
		},
		{
			name:        "extra variables ignored",
			content:     "Hello {{name}}!",
			vars:        map[string]string{"name": "John", "extra": "ignored"},
			wantMissing: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := &Template{Content: tt.content}
			tmpl.ExtractVariables()
			missing := tmpl.Validate(tt.vars)

			assert.Len(t, missing, len(tt.wantMissing))
			for _, v := range tt.wantMissing {
				assert.Contains(t, missing, v)
			}
		})
	}
}
