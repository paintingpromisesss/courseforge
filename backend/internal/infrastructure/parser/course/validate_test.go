package course

import (
	"strings"
	"testing"

	"github.com/paintingpromisesss/courseforge/internal/domain"
)

type validatable interface{ Validate() error }

func TestValidateInMemory(t *testing.T) {
	tests := []struct {
		name    string
		entity  validatable
		wantSub string // "" means expect no error
	}{
		{
			name:    "valid unit (theory only)",
			entity:  &domain.Unit{Slug: "u", Title: "T", Theory: "theory.md"},
			wantSub: "",
		},
		{
			name:    "empty unit",
			entity:  &domain.Unit{Slug: "u", Title: "T"},
			wantSub: "unit is empty",
		},
		{
			name:    "task without languages",
			entity:  &domain.Task{Slug: "k", Title: "T", Statement: "statement.md"},
			wantSub: "at least one language",
		},
		{
			name: "task language missing template",
			entity: &domain.Task{
				Slug: "k", Title: "T", Statement: "s.md",
				Languages: map[string]domain.Language{"go": {Tests: "x_test.go"}},
			},
			wantSub: "languages.go.template",
		},
		{
			name:    "topic without units",
			entity:  &domain.Topic{Slug: "t", Title: "T"},
			wantSub: "at least one unit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.entity.Validate()
			if tt.wantSub == "" {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantSub, err)
			}
		})
	}
}

func TestValidateInMemoryNoDiskAccess(t *testing.T) {
	u := &domain.Unit{Slug: "u", Title: "T", Theory: "missing-on-disk.md"}
	if err := u.Validate(); err != nil {
		t.Fatalf("in-memory Validate should ignore file existence, got: %v", err)
	}
}
