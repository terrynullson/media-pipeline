package trigger

import (
	"fmt"
	"strings"
	"time"
)

type MatchMode string

const (
	MatchModeContains MatchMode = "contains"
	MatchModeExact    MatchMode = "exact"
)

type Rule struct {
	ID           int64
	Name         string
	Category     string
	Pattern      string
	MatchMode    MatchMode
	Enabled      bool
	CreatedAtUTC time.Time
	UpdatedAtUTC time.Time
}

type Event struct {
	ID           int64
	MediaID      int64
	TranscriptID *int64
	RuleID       int64
	RuleName     string
	Category     string
	MatchedText  string
	SegmentIndex int
	StartSec     float64
	EndSec       float64
	SegmentText  string
	ContextText  string
	CreatedAtUTC time.Time
}

func NormalizeRule(rule Rule) Rule {
	rule.Name = strings.TrimSpace(rule.Name)
	rule.Category = strings.TrimSpace(rule.Category)
	rule.Pattern = strings.Join(strings.Fields(strings.TrimSpace(rule.Pattern)), " ")
	rule.MatchMode = MatchMode(strings.ToLower(strings.TrimSpace(string(rule.MatchMode))))

	return rule
}

func ValidateRule(rule Rule) error {
	if strings.TrimSpace(rule.Name) == "" {
		return fmt.Errorf("название правила обязательно")
	}
	if strings.TrimSpace(rule.Category) == "" {
		return fmt.Errorf("категория правила обязательна")
	}
	if strings.TrimSpace(rule.Pattern) == "" {
		return fmt.Errorf("шаблон правила обязателен")
	}

	switch rule.MatchMode {
	case MatchModeContains, MatchModeExact:
		return nil
	default:
		return fmt.Errorf("неподдерживаемый режим совпадения %q", rule.MatchMode)
	}
}

func SupportedMatchModes() []MatchMode {
	return []MatchMode{
		MatchModeContains,
		MatchModeExact,
	}
}
