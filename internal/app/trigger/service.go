package triggerapp

import (
	"context"
	"fmt"
	"time"

	domaintrigger "media-pipeline/internal/domain/trigger"
)

type RuleRepository interface {
	List(ctx context.Context) ([]domaintrigger.Rule, error)
	Create(ctx context.Context, rule domaintrigger.Rule) (domaintrigger.Rule, error)
	SetEnabled(ctx context.Context, id int64, enabled bool, nowUTC time.Time) error
	Delete(ctx context.Context, id int64) error
}

type Service struct {
	repo RuleRepository
}

func NewService(repo RuleRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context) ([]domaintrigger.Rule, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list trigger rules: %w", err)
	}

	return items, nil
}

func (s *Service) Create(ctx context.Context, rule domaintrigger.Rule) (domaintrigger.Rule, error) {
	rule = domaintrigger.NormalizeRule(rule)
	rule.Enabled = true
	if err := domaintrigger.ValidateRule(rule); err != nil {
		return domaintrigger.Rule{}, err
	}

	nowUTC := time.Now().UTC()
	rule.CreatedAtUTC = nowUTC
	rule.UpdatedAtUTC = nowUTC

	saved, err := s.repo.Create(ctx, rule)
	if err != nil {
		return domaintrigger.Rule{}, fmt.Errorf("create trigger rule: %w", err)
	}

	return saved, nil
}

func (s *Service) SetEnabled(ctx context.Context, id int64, enabled bool) error {
	if id <= 0 {
		return fmt.Errorf("trigger rule id is required")
	}

	if err := s.repo.SetEnabled(ctx, id, enabled, time.Now().UTC()); err != nil {
		return fmt.Errorf("set trigger rule enabled: %w", err)
	}

	return nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("trigger rule id is required")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete trigger rule: %w", err)
	}

	return nil
}
