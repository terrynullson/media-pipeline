package appsettingsapp

import (
	"context"
	"fmt"
	"time"

	"media-pipeline/internal/domain/appsettings"
)

type Repository interface {
	Get(ctx context.Context) (appsettings.Settings, bool, error)
	Save(ctx context.Context, settings appsettings.Settings) (appsettings.Settings, error)
}

type Service struct {
	repo      Repository
	bootstrap appsettings.Settings
}

func NewService(repo Repository, bootstrap appsettings.Settings) *Service {
	return &Service{
		repo:      repo,
		bootstrap: appsettings.NormalizeSettings(bootstrap),
	}
}

func (s *Service) GetCurrent(ctx context.Context) (appsettings.Settings, error) {
	settings, ok, err := s.repo.Get(ctx)
	if err != nil {
		return appsettings.Settings{}, fmt.Errorf("get runtime settings: %w", err)
	}
	if ok {
		return appsettings.NormalizeSettings(settings), nil
	}

	nowUTC := time.Now().UTC()
	settings = s.bootstrap
	settings.CreatedAtUTC = nowUTC
	settings.UpdatedAtUTC = nowUTC

	saved, err := s.repo.Save(ctx, settings)
	if err != nil {
		return appsettings.Settings{}, fmt.Errorf("create runtime settings: %w", err)
	}

	return appsettings.NormalizeSettings(saved), nil
}

func (s *Service) SaveCurrent(ctx context.Context, settings appsettings.Settings) (appsettings.Settings, error) {
	settings = appsettings.NormalizeSettings(settings)
	if err := appsettings.ValidateSettings(settings); err != nil {
		return appsettings.Settings{}, err
	}

	current, ok, err := s.repo.Get(ctx)
	if err != nil {
		return appsettings.Settings{}, fmt.Errorf("get current runtime settings: %w", err)
	}

	nowUTC := time.Now().UTC()
	if ok {
		settings.ID = current.ID
		settings.CreatedAtUTC = current.CreatedAtUTC
	} else if settings.CreatedAtUTC.IsZero() {
		settings.CreatedAtUTC = nowUTC
	}
	settings.UpdatedAtUTC = nowUTC

	saved, err := s.repo.Save(ctx, settings)
	if err != nil {
		return appsettings.Settings{}, fmt.Errorf("save runtime settings: %w", err)
	}

	return appsettings.NormalizeSettings(saved), nil
}
