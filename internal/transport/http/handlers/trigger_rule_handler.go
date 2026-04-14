package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	mediaapp "media-pipeline/internal/app/media"
	domaintrigger "media-pipeline/internal/domain/trigger"
	"media-pipeline/internal/observability"
)

// triggerRulePageRenderer is implemented by UploadHandler so TriggerRuleHandler
// can delegate HTML error rendering for the legacy form-based endpoints without
// carrying the full template machinery itself.
type triggerRulePageRenderer interface {
	renderTriggerRuleError(w http.ResponseWriter, r *http.Request, errMsg string, form *TriggerRuleForm)
}

// TriggerPreviewService runs a dry-run trigger match against existing transcripts.
type TriggerPreviewService interface {
	Preview(ctx context.Context, req mediaapp.TriggerPreviewRequest) (mediaapp.TriggerPreviewResult, error)
}

// TriggerRuleHandler handles both the legacy HTML form endpoints
// (/trigger-rules/...) and the JSON API endpoints (/api/trigger-rules/...).
type TriggerRuleHandler struct {
	triggerRulesSvc TriggerRulesService
	previewSvc      TriggerPreviewService
	pageRenderer    triggerRulePageRenderer
	logger          *slog.Logger
}

func NewTriggerRuleHandler(svc TriggerRulesService, pageRenderer triggerRulePageRenderer, logger *slog.Logger) *TriggerRuleHandler {
	return &TriggerRuleHandler{
		triggerRulesSvc: svc,
		pageRenderer:    pageRenderer,
		logger:          logger,
	}
}

// WithPreviewService attaches the trigger preview use case to the handler.
func (h *TriggerRuleHandler) WithPreviewService(svc TriggerPreviewService) *TriggerRuleHandler {
	h.previewSvc = svc
	return h
}

func (h *TriggerRuleHandler) writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

// — Legacy HTML form endpoints —

func (h *TriggerRuleHandler) CreateTriggerRule(w http.ResponseWriter, r *http.Request) {
	logger := observability.LoggerFromContext(r.Context(), h.logger).With(
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
	)

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		logger.Warn("trigger rule request rejected: invalid form", slog.Any("error", err))
		h.pageRenderer.renderTriggerRuleError(w, r,
			"Не удалось прочитать форму правила триггера. Попробуйте ещё раз.",
			buildTriggerRuleFormFromRequest(r))
		return
	}

	rule, form, err := parseTriggerRuleForm(r)
	if err != nil {
		h.pageRenderer.renderTriggerRuleError(w, r, err.Error(), &form)
		return
	}

	if _, err := h.triggerRulesSvc.Create(r.Context(), rule); err != nil {
		logger.Warn("create trigger rule failed", slog.Any("error", err))
		h.pageRenderer.renderTriggerRuleError(w, r,
			"Не удалось сохранить правило триггера: "+err.Error(), &form)
		return
	}

	http.Redirect(w, r, "/?status=trigger_rule_saved", http.StatusSeeOther)
}

func (h *TriggerRuleHandler) ToggleTriggerRule(w http.ResponseWriter, r *http.Request) {
	ruleID, err := triggerRuleIDFromRequest(r)
	if err != nil {
		http.Error(w, "invalid trigger rule id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.pageRenderer.renderTriggerRuleError(w, r,
			"Не удалось прочитать действие для правила триггера.", nil)
		return
	}

	enabled := r.FormValue("enabled") == "true"
	if err := h.triggerRulesSvc.SetEnabled(r.Context(), ruleID, enabled); err != nil {
		h.pageRenderer.renderTriggerRuleError(w, r,
			"Не удалось обновить правило триггера: "+err.Error(), nil)
		return
	}

	http.Redirect(w, r, "/?status=trigger_rule_updated", http.StatusSeeOther)
}

func (h *TriggerRuleHandler) DeleteTriggerRule(w http.ResponseWriter, r *http.Request) {
	ruleID, err := triggerRuleIDFromRequest(r)
	if err != nil {
		http.Error(w, "invalid trigger rule id", http.StatusBadRequest)
		return
	}

	if err := h.triggerRulesSvc.Delete(r.Context(), ruleID); err != nil {
		h.pageRenderer.renderTriggerRuleError(w, r,
			"Не удалось удалить правило триггера: "+err.Error(), nil)
		return
	}

	http.Redirect(w, r, "/?status=trigger_rule_deleted", http.StatusSeeOther)
}

// — JSON API endpoints —

func (h *TriggerRuleHandler) APITriggerRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.triggerRulesSvc.List(r.Context())
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("load api trigger rules failed", slog.Any("error", err))
		http.Error(w, "не удалось загрузить правила", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{"items": buildTriggerRuleViews(rules)})
}

func (h *TriggerRuleHandler) APICreateTriggerRule(w http.ResponseWriter, r *http.Request) {
	var payload triggerRulePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "не удалось прочитать JSON правила", http.StatusBadRequest)
		return
	}

	rule := domaintrigger.NormalizeRule(domaintrigger.Rule{
		Name:      payload.Name,
		Category:  payload.Category,
		Pattern:   payload.Pattern,
		MatchMode: domaintrigger.MatchMode(payload.MatchMode),
		Enabled:   true,
	})
	if err := domaintrigger.ValidateRule(rule); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": err.Error()})
		return
	}

	created, err := h.triggerRulesSvc.Create(r.Context(), rule)
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("create api trigger rule failed", slog.Any("error", err))
		http.Error(w, "не удалось создать правило", http.StatusInternalServerError)
		return
	}

	view := buildTriggerRuleViews([]domaintrigger.Rule{created})
	h.writeJSON(w, http.StatusCreated, map[string]any{"status": "created", "item": view[0]})
}

func (h *TriggerRuleHandler) APIUpdateTriggerRule(w http.ResponseWriter, r *http.Request) {
	ruleID, err := triggerRuleIDFromRequest(r)
	if err != nil {
		http.Error(w, "invalid trigger rule id", http.StatusBadRequest)
		return
	}

	var payload triggerRulePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "не удалось прочитать JSON правила", http.StatusBadRequest)
		return
	}
	if payload.Enabled == nil {
		http.Error(w, "нужно передать enabled", http.StatusBadRequest)
		return
	}

	if err := h.triggerRulesSvc.SetEnabled(r.Context(), ruleID, *payload.Enabled); err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("update api trigger rule failed",
			slog.Int64("rule_id", ruleID), slog.Any("error", err))
		http.Error(w, "не удалось обновить правило", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{"status": "updated", "ruleId": ruleID, "enabled": *payload.Enabled})
}

func (h *TriggerRuleHandler) APIDeleteTriggerRule(w http.ResponseWriter, r *http.Request) {
	ruleID, err := triggerRuleIDFromRequest(r)
	if err != nil {
		http.Error(w, "invalid trigger rule id", http.StatusBadRequest)
		return
	}

	if err := h.triggerRulesSvc.Delete(r.Context(), ruleID); err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("delete api trigger rule failed",
			slog.Int64("rule_id", ruleID), slog.Any("error", err))
		http.Error(w, "не удалось удалить правило", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{"status": "deleted", "ruleId": ruleID})
}

// — Types —

type TriggerRuleForm struct {
	Name      string
	Category  string
	Pattern   string
	MatchMode string
}

type TriggerRuleView struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	Pattern      string `json:"pattern"`
	MatchMode    string `json:"matchMode"`
	Enabled      bool   `json:"enabled"`
	ToggleURL    string `json:"toggleUrl"`
	DeleteURL    string `json:"deleteUrl"`
	ToggleLabel  string `json:"toggleLabel"`
	EnabledLabel string `json:"enabledLabel"`
	EnabledTone  string `json:"enabledTone"`
}

type triggerRulePayload struct {
	Name      string `json:"name"`
	Category  string `json:"category"`
	Pattern   string `json:"pattern"`
	MatchMode string `json:"matchMode"`
	Enabled   *bool  `json:"enabled,omitempty"`
}

// — Helpers —

func buildTriggerRuleViews(items []domaintrigger.Rule) []TriggerRuleView {
	views := make([]TriggerRuleView, 0, len(items))
	for _, item := range items {
		view := TriggerRuleView{
			ID:           item.ID,
			Name:         item.Name,
			Category:     item.Category,
			Pattern:      item.Pattern,
			MatchMode:    string(item.MatchMode),
			Enabled:      item.Enabled,
			ToggleURL:    fmt.Sprintf("/trigger-rules/%d/toggle", item.ID),
			DeleteURL:    fmt.Sprintf("/trigger-rules/%d/delete", item.ID),
			EnabledLabel: "Выключено",
			EnabledTone:  "neutral",
			ToggleLabel:  "Включить",
		}
		if item.Enabled {
			view.EnabledLabel = "Включено"
			view.EnabledTone = "success"
			view.ToggleLabel = "Выключить"
		}
		views = append(views, view)
	}
	return views
}

func parseTriggerRuleForm(r *http.Request) (domaintrigger.Rule, TriggerRuleForm, error) {
	form := *buildTriggerRuleFormFromRequest(r)
	rule := domaintrigger.NormalizeRule(domaintrigger.Rule{
		Name:      form.Name,
		Category:  form.Category,
		Pattern:   form.Pattern,
		MatchMode: domaintrigger.MatchMode(form.MatchMode),
		Enabled:   true,
	})
	form = buildTriggerRuleForm(rule)

	if err := domaintrigger.ValidateRule(rule); err != nil {
		return domaintrigger.Rule{}, form, err
	}

	return rule, form, nil
}

func buildTriggerRuleForm(rule domaintrigger.Rule) TriggerRuleForm {
	return TriggerRuleForm{
		Name:      rule.Name,
		Category:  rule.Category,
		Pattern:   rule.Pattern,
		MatchMode: string(rule.MatchMode),
	}
}

func buildTriggerRuleFormFromRequest(r *http.Request) *TriggerRuleForm {
	return &TriggerRuleForm{
		Name:      strings.TrimSpace(r.FormValue("name")),
		Category:  strings.TrimSpace(r.FormValue("category")),
		Pattern:   strings.TrimSpace(r.FormValue("pattern")),
		MatchMode: strings.TrimSpace(r.FormValue("match_mode")),
	}
}

func matchModeOptions() []string {
	options := domaintrigger.SupportedMatchModes()
	values := make([]string, 0, len(options))
	for _, option := range options {
		values = append(values, string(option))
	}
	return values
}

func triggerRuleIDFromRequest(r *http.Request) (int64, error) {
	raw := strings.TrimSpace(chi.URLParam(r, "ruleID"))
	if raw == "" {
		return 0, fmt.Errorf("trigger rule id is required")
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid trigger rule id %q", raw)
	}

	return value, nil
}

// APIPreviewTriggerRule runs a dry-run trigger match against existing transcripts.
// POST /api/trigger-rules/preview
func (h *TriggerRuleHandler) APIPreviewTriggerRule(w http.ResponseWriter, r *http.Request) {
	if h.previewSvc == nil {
		http.Error(w, "preview not available", http.StatusNotImplemented)
		return
	}

	var payload struct {
		Pattern   string `json:"pattern"`
		MatchMode string `json:"matchMode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	payload.Pattern = strings.TrimSpace(payload.Pattern)
	if payload.Pattern == "" {
		http.Error(w, "pattern is required", http.StatusBadRequest)
		return
	}
	switch domaintrigger.MatchMode(payload.MatchMode) {
	case domaintrigger.MatchModeContains, domaintrigger.MatchModeExact:
	default:
		http.Error(w, "matchMode must be 'contains' or 'exact'", http.StatusBadRequest)
		return
	}

	result, err := h.previewSvc.Preview(r.Context(), mediaapp.TriggerPreviewRequest{
		Pattern:   payload.Pattern,
		MatchMode: payload.MatchMode,
	})
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error(
			"trigger preview failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"totalMatches": result.TotalMatches,
		"mediaMatches": result.MediaMatches,
		"limited":      result.Limited,
	})
}

