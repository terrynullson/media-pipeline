package repositories

import (
	"context"
	"testing"
	"time"

	domaintrigger "media-pipeline/internal/domain/trigger"
)

func TestTriggerRuleRepository_CreateListSetEnabledDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	repo := NewTriggerRuleRepository(sqlDB)
	nowUTC := time.Date(2026, 4, 3, 15, 0, 0, 0, time.UTC)

	rule, err := repo.Create(ctx, domaintrigger.Rule{
		Name:         "Test Rule",
		Category:     "support",
		Pattern:      "speak to a manager",
		MatchMode:    domaintrigger.MatchModeContains,
		Enabled:      true,
		CreatedAtUTC: nowUTC,
		UpdatedAtUTC: nowUTC,
	})
	if err != nil {
		t.Fatalf("Create(trigger rule) error = %v", err)
	}

	items, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) < 4 {
		t.Fatalf("items = %d, want at least seeded rules plus created rule", len(items))
	}

	if err := repo.SetEnabled(ctx, rule.ID, false, nowUTC.Add(time.Minute)); err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}

	enabled, err := repo.ListEnabled(ctx)
	if err != nil {
		t.Fatalf("ListEnabled() error = %v", err)
	}
	for _, item := range enabled {
		if item.ID == rule.ID {
			t.Fatalf("rule %d still present in enabled list", rule.ID)
		}
	}

	if err := repo.Delete(ctx, rule.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	items, err = repo.List(ctx)
	if err != nil {
		t.Fatalf("List(after delete) error = %v", err)
	}
	for _, item := range items {
		if item.ID == rule.ID {
			t.Fatalf("rule %d still exists after delete", rule.ID)
		}
	}
}
