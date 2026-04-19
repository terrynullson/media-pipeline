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

const triggerRuleColumns = `id, name, category, pattern, match_mode, enabled, created_at, updated_at`

func (r *TriggerRuleRepository) List(ctx context.Context) ([]domaintrigger.Rule, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT `+triggerRuleColumns+`
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
		`SELECT `+triggerRuleColumns+`
		 FROM trigger_rules
		 WHERE enabled = TRUE
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
	var id int64
	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO trigger_rules (name, category, pattern, match_mode, enabled, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		rule.Name,
		rule.Category,
		rule.Pattern,
		rule.MatchMode,
		rule.Enabled,
		rule.CreatedAtUTC.UTC(),
		rule.UpdatedAtUTC.UTC(),
	).Scan(&id)
	if err != nil {
		return domaintrigger.Rule{}, fmt.Errorf("insert trigger rule: %w", err)
	}
	rule.ID = id

	return rule, nil
}

func (r *TriggerRuleRepository) SetEnabled(ctx context.Context, id int64, enabled bool, nowUTC time.Time) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE trigger_rules
		 SET enabled = $1, updated_at = $2
		 WHERE id = $3`,
		enabled,
		nowUTC.UTC(),
		id,
	)
	if err != nil {
		return fmt.Errorf("update trigger rule enabled: %w", err)
	}

	return ensureTriggerRuleRowsAffected(result, id, "update trigger rule enabled")
}

func (r *TriggerRuleRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM trigger_rules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete trigger rule: %w", err)
	}

	return ensureTriggerRuleRowsAffected(result, id, "delete trigger rule")
}

func scanTriggerRule(scanner rowScanner) (domaintrigger.Rule, error) {
	var item domaintrigger.Rule
	var createdAt, updatedAt time.Time

	if err := scanner.Scan(
		&item.ID,
		&item.Name,
		&item.Category,
		&item.Pattern,
		&item.MatchMode,
		&item.Enabled,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domaintrigger.Rule{}, fmt.Errorf("scan trigger rule: %w", err)
	}

	item.CreatedAtUTC = createdAt.UTC()
	item.UpdatedAtUTC = updatedAt.UTC()

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
