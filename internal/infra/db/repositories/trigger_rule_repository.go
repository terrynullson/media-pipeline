package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	domaintrigger "media-pipeline/internal/domain/trigger"
)

type TriggerRuleRepository struct {
	db *sql.DB
}

func NewTriggerRuleRepository(db *sql.DB) *TriggerRuleRepository {
	return &TriggerRuleRepository{db: db}
}

func (r *TriggerRuleRepository) List(ctx context.Context) ([]domaintrigger.Rule, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, name, category, pattern, match_mode, enabled, created_at, updated_at
		 FROM trigger_rules
		 ORDER BY enabled DESC, category ASC, name ASC, id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query trigger rules: %w", err)
	}
	defer rows.Close()

	items := make([]domaintrigger.Rule, 0)
	for rows.Next() {
		item, err := scanTriggerRule(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trigger rules: %w", err)
	}

	return items, nil
}

func (r *TriggerRuleRepository) ListEnabled(ctx context.Context) ([]domaintrigger.Rule, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, name, category, pattern, match_mode, enabled, created_at, updated_at
		 FROM trigger_rules
		 WHERE enabled = 1
		 ORDER BY category ASC, name ASC, id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query enabled trigger rules: %w", err)
	}
	defer rows.Close()

	items := make([]domaintrigger.Rule, 0)
	for rows.Next() {
		item, err := scanTriggerRule(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enabled trigger rules: %w", err)
	}

	return items, nil
}

func (r *TriggerRuleRepository) Create(ctx context.Context, rule domaintrigger.Rule) (domaintrigger.Rule, error) {
	result, err := r.db.ExecContext(
		ctx,
		`INSERT INTO trigger_rules (name, category, pattern, match_mode, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rule.Name,
		rule.Category,
		rule.Pattern,
		rule.MatchMode,
		triggerBoolToInt(rule.Enabled),
		rule.CreatedAtUTC.Format(time.RFC3339),
		rule.UpdatedAtUTC.Format(time.RFC3339),
	)
	if err != nil {
		return domaintrigger.Rule{}, fmt.Errorf("insert trigger rule: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return domaintrigger.Rule{}, fmt.Errorf("trigger rule last insert id: %w", err)
	}
	rule.ID = id

	return rule, nil
}

func (r *TriggerRuleRepository) SetEnabled(ctx context.Context, id int64, enabled bool, nowUTC time.Time) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE trigger_rules
		 SET enabled = ?, updated_at = ?
		 WHERE id = ?`,
		triggerBoolToInt(enabled),
		nowUTC.Format(time.RFC3339),
		id,
	)
	if err != nil {
		return fmt.Errorf("update trigger rule enabled: %w", err)
	}

	return ensureTriggerRuleRowsAffected(result, id, "update trigger rule enabled")
}

func (r *TriggerRuleRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM trigger_rules WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete trigger rule: %w", err)
	}

	return ensureTriggerRuleRowsAffected(result, id, "delete trigger rule")
}

func scanTriggerRule(scanner interface {
	Scan(dest ...any) error
}) (domaintrigger.Rule, error) {
	var item domaintrigger.Rule
	var enabled int
	var createdAt string
	var updatedAt string

	if err := scanner.Scan(
		&item.ID,
		&item.Name,
		&item.Category,
		&item.Pattern,
		&item.MatchMode,
		&enabled,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domaintrigger.Rule{}, fmt.Errorf("scan trigger rule: %w", err)
	}

	item.Enabled = enabled == 1
	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return domaintrigger.Rule{}, fmt.Errorf("parse trigger rule created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return domaintrigger.Rule{}, fmt.Errorf("parse trigger rule updated_at: %w", err)
	}
	item.CreatedAtUTC = parsedCreatedAt
	item.UpdatedAtUTC = parsedUpdatedAt

	return item, nil
}

func ensureTriggerRuleRowsAffected(result sql.Result, id int64, action string) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s rows affected: %w", action, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("%s: trigger rule %d not found", action, id)
	}

	return nil
}

func triggerBoolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
