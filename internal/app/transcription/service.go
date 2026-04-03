package transcriptionapp

import (
	"context"
	"fmt"
	"time"

	"media-pipeline/internal/domain/transcription"
)

type ProfileRepository interface {
	GetDefault(ctx context.Context) (transcription.Profile, bool, error)
	Save(ctx context.Context, profile transcription.Profile) (transcription.Profile, error)
}

type Service struct {
	repo            ProfileRepository
	bootstrapConfig transcription.Profile
}

func NewService(repo ProfileRepository, bootstrapConfig transcription.Profile) *Service {
	return &Service{
		repo:            repo,
		bootstrapConfig: transcription.NormalizeProfile(bootstrapConfig),
	}
}

func (s *Service) GetCurrent(ctx context.Context) (transcription.Profile, error) {
	profile, ok, err := s.repo.GetDefault(ctx)
	if err != nil {
		return transcription.Profile{}, fmt.Errorf("get default transcription profile: %w", err)
	}
	if ok {
		return transcription.NormalizeProfile(profile), nil
	}

	nowUTC := time.Now().UTC()
	profile = s.bootstrapConfig
	profile.IsDefault = true
	profile.CreatedAtUTC = nowUTC
	profile.UpdatedAtUTC = nowUTC

	saved, err := s.repo.Save(ctx, profile)
	if err != nil {
		return transcription.Profile{}, fmt.Errorf("create default transcription profile: %w", err)
	}

	return transcription.NormalizeProfile(saved), nil
}

func (s *Service) SaveCurrent(ctx context.Context, profile transcription.Profile) (transcription.Profile, error) {
	profile = transcription.NormalizeProfile(profile)
	profile.IsDefault = true
	if err := transcription.ValidateProfile(profile); err != nil {
		return transcription.Profile{}, err
	}

	current, ok, err := s.repo.GetDefault(ctx)
	if err != nil {
		return transcription.Profile{}, fmt.Errorf("get current transcription profile: %w", err)
	}

	nowUTC := time.Now().UTC()
	if ok {
		profile.ID = current.ID
		profile.CreatedAtUTC = current.CreatedAtUTC
	} else if profile.CreatedAtUTC.IsZero() {
		profile.CreatedAtUTC = nowUTC
	}
	profile.UpdatedAtUTC = nowUTC

	saved, err := s.repo.Save(ctx, profile)
	if err != nil {
		return transcription.Profile{}, fmt.Errorf("save transcription profile: %w", err)
	}

	return transcription.NormalizeProfile(saved), nil
}
