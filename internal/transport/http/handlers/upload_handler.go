package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"media-pipeline/internal/app/command"
	mediaapp "media-pipeline/internal/app/media"
	"media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/transcription"
	domaintrigger "media-pipeline/internal/domain/trigger"
	"media-pipeline/internal/observability"
)

type UploadHandler struct {
	uploadUC         *command.UploadMediaUseCase
	transcriptionSvc TranscriptionSettingsService
	triggerRulesSvc  TriggerRulesService
	transcriptViewUC TranscriptViewService
	deleteMediaUC    MediaDeletionService
	tmpl             *template.Template
	maxUploadSizeB   int64
	maxRequestBodyB  int64
	maxFormMemoryB   int64
	listItemsMaxSize int
	maxSettingsBodyB int64
	logger           *slog.Logger
}

type TranscriptionSettingsService interface {
	GetCurrent(ctx context.Context) (transcription.Profile, error)
	SaveCurrent(ctx context.Context, profile transcription.Profile) (transcription.Profile, error)
}

type TranscriptViewService interface {
	Load(ctx context.Context, mediaID int64) (mediaapp.TranscriptViewResult, error)
}

type TriggerRulesService interface {
	List(ctx context.Context) ([]domaintrigger.Rule, error)
	Create(ctx context.Context, rule domaintrigger.Rule) (domaintrigger.Rule, error)
	SetEnabled(ctx context.Context, id int64, enabled bool) error
	Delete(ctx context.Context, id int64) error
}

type MediaDeletionService interface {
	Delete(ctx context.Context, mediaID int64) (mediaapp.DeleteMediaResult, error)
}

type MediaListItem struct {
	ID                int64
	OriginalName      string
	Extension         string
	SizeHuman         string
	Status            media.Status
	StatusLabel       string
	StatusTone        string
	StageLabel        string
	StageValue        int
	StageTotal        int
	StagePercent      int
	IsActive          bool
	CreatedAtUTC      string
	CanOpenTranscript bool
	HasTranscript     bool
	TriggerCount      int
	TranscriptURL     string
	DeleteURL         string
}

type IndexViewData struct {
	Items               []MediaListItem
	UploadError         string
	UploadSuccess       string
	SettingsError       string
	SettingsSuccess     string
	TriggerRuleError    string
	TriggerRuleSuccess  string
	MaxUploadMB         string
	MaxUploadHuman      string
	SettingsForm        TranscriptionSettingsForm
	TriggerRuleForm     TriggerRuleForm
	TriggerRules        []TriggerRuleView
	BackendOptions      []string
	ModelOptions        []string
	DeviceOptions       []string
	CurrentComputeTypes []string
	CPUComputeTypes     []string
	CUDAComputeTypes    []string
	MatchModeOptions    []string
}

type TranscriptionSettingsForm struct {
	Backend     string
	ModelName   string
	Device      string
	ComputeType string
	Language    string
	BeamSize    int
	VADEnabled  bool
}

type TriggerRuleForm struct {
	Name      string
	Category  string
	Pattern   string
	MatchMode string
}

type TriggerRuleView struct {
	ID           int64
	Name         string
	Category     string
	Pattern      string
	MatchMode    string
	Enabled      bool
	ToggleURL    string
	DeleteURL    string
	ToggleLabel  string
	EnabledLabel string
	EnabledTone  string
}

func NewUploadHandler(
	uploadUC *command.UploadMediaUseCase,
	transcriptionSvc TranscriptionSettingsService,
	triggerRulesSvc TriggerRulesService,
	transcriptViewUC TranscriptViewService,
	deleteMediaUC MediaDeletionService,
	templatesDir string,
	maxUploadSizeB int64,
	logger *slog.Logger,
) (*UploadHandler, error) {
	tmpl, err := template.New("index.html").Funcs(template.FuncMap{
		"humanSize":         HumanSize,
		"formatTimestamp":   FormatTimestamp,
		"formatDateTimeUTC": FormatDateTimeUTC,
	}).ParseGlob(filepath.Join(templatesDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("parse html templates: %w", err)
	}

	return &UploadHandler{
		uploadUC:         uploadUC,
		transcriptionSvc: transcriptionSvc,
		triggerRulesSvc:  triggerRulesSvc,
		transcriptViewUC: transcriptViewUC,
		deleteMediaUC:    deleteMediaUC,
		tmpl:             tmpl,
		maxUploadSizeB:   maxUploadSizeB,
		maxRequestBodyB:  maxUploadSizeB + (1 << 20),
		maxFormMemoryB:   32 << 20,
		listItemsMaxSize: 100,
		maxSettingsBodyB: 1 << 20,
		logger:           logger,
	}, nil
}

func (h *UploadHandler) Index(w http.ResponseWriter, r *http.Request) {
	successMessage := ""
	triggerRuleSuccess := ""
	switch r.URL.Query().Get("status") {
	case "uploaded":
		successMessage = "Upload completed. Media record and pending job were created."
	case "deleted":
		successMessage = "Media item was deleted. Related records were removed and file cleanup was attempted."
	case "trigger_rule_saved":
		triggerRuleSuccess = "Trigger rule was created."
	case "trigger_rule_updated":
		triggerRuleSuccess = "Trigger rule status was updated."
	case "trigger_rule_deleted":
		triggerRuleSuccess = "Trigger rule was deleted."
	}
	settingsSuccess := ""
	if r.URL.Query().Get("status") == "settings_saved" {
		settingsSuccess = "Transcription settings were saved."
	}

	h.renderIndex(w, r, "", successMessage, "", settingsSuccess, "", triggerRuleSuccess, nil, nil)
}

func (h *UploadHandler) Upload(w http.ResponseWriter, r *http.Request) {
	logger := observability.LoggerFromContext(r.Context(), h.logger).With(
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
	)

	r.Body = http.MaxBytesReader(w, r.Body, h.maxRequestBodyB)
	if err := r.ParseMultipartForm(h.maxFormMemoryB); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			logger.Warn("upload request rejected: request body too large", slog.Any("error", err))
			h.renderUploadFailure(w, r, h.uploadLimitMessage("File is too large."))
			return
		}

		logger.Warn("upload request rejected: invalid multipart form", slog.Any("error", err))
		h.renderUploadFailure(w, r, "Could not read the upload form. Please choose the file again.")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	file, fileHeader, err := r.FormFile("media")
	if err != nil {
		logger.Warn("upload request rejected: media field missing", slog.Any("error", err))
		h.renderUploadFailure(w, r, "Please choose a file in the media field.")
		return
	}
	defer file.Close()

	if strings.TrimSpace(fileHeader.Filename) == "" {
		h.renderUploadFailure(w, r, "File name is required.")
		return
	}
	if fileHeader.Size == 0 {
		h.renderUploadFailure(w, r, "Empty upload is not allowed.")
		return
	}

	logger.Info("upload accepted",
		slog.String("filename", fileHeader.Filename),
		slog.Int64("declared_size_bytes", fileHeader.Size),
	)

	result, err := h.uploadUC.Upload(r.Context(), command.UploadMediaInput{
		OriginalName: fileHeader.Filename,
		MIMEType:     normalizeMIMEType(fileHeader.Header.Get("Content-Type")),
		SizeBytes:    fileHeader.Size,
		Content:      file,
	})
	if err != nil {
		logger.Warn("upload failed", slog.Any("error", err), slog.String("filename", fileHeader.Filename))
		h.renderUploadFailure(w, r, h.userFacingUploadError(err))
		return
	}

	logger.Info("upload succeeded", slog.String("filename", fileHeader.Filename))
	if wantsJSON(r) {
		h.writeJSON(w, http.StatusCreated, map[string]any{
			"status":  "uploaded",
			"mediaId": result.MediaID,
			"message": "Upload completed. Media record and pending job were created.",
		})
		return
	}
	http.Redirect(w, r, "/?status=uploaded", http.StatusSeeOther)
}

func (h *UploadHandler) MediaStatuses(w http.ResponseWriter, r *http.Request) {
	items, err := h.buildMediaListItems(r.Context())
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("load media statuses failed", slog.Any("error", err))
		http.Error(w, "failed to load media statuses", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (h *UploadHandler) renderUploadFailure(w http.ResponseWriter, r *http.Request, message string) {
	if wantsJSON(r) {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{
			"status":  "error",
			"message": message,
		})
		return
	}

	h.renderIndex(w, r, message, "", "", "", "", "", nil, nil)
}

func (h *UploadHandler) SaveTranscriptionSettings(w http.ResponseWriter, r *http.Request) {
	logger := observability.LoggerFromContext(r.Context(), h.logger).With(
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
	)

	r.Body = http.MaxBytesReader(w, r.Body, h.maxSettingsBodyB)
	if err := r.ParseForm(); err != nil {
		logger.Warn("settings request rejected: invalid form", slog.Any("error", err))
		h.renderIndex(w, r, "", "", "Could not read the settings form. Please try again.", "", "", "", buildSettingsFormFromRequest(r), nil)
		return
	}

	profile, form, err := parseTranscriptionProfileForm(r)
	if err != nil {
		h.renderIndex(w, r, "", "", err.Error(), "", "", "", &form, nil)
		return
	}

	if _, err := h.transcriptionSvc.SaveCurrent(r.Context(), profile); err != nil {
		logger.Warn("save transcription settings failed", slog.Any("error", err))
		h.renderIndex(w, r, "", "", "Could not save settings: "+err.Error(), "", "", "", &form, nil)
		return
	}

	logger.Info("transcription settings saved",
		slog.String("backend", string(profile.Backend)),
		slog.String("model_name", profile.ModelName),
		slog.String("device", profile.Device),
		slog.String("compute_type", profile.ComputeType),
	)
	http.Redirect(w, r, "/?status=settings_saved", http.StatusSeeOther)
}

func (h *UploadHandler) CreateTriggerRule(w http.ResponseWriter, r *http.Request) {
	logger := observability.LoggerFromContext(r.Context(), h.logger).With(
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
	)

	r.Body = http.MaxBytesReader(w, r.Body, h.maxSettingsBodyB)
	if err := r.ParseForm(); err != nil {
		logger.Warn("trigger rule request rejected: invalid form", slog.Any("error", err))
		h.renderIndex(w, r, "", "", "", "", "Could not read the trigger rule form. Please try again.", "", nil, buildTriggerRuleFormFromRequest(r))
		return
	}

	rule, form, err := parseTriggerRuleForm(r)
	if err != nil {
		h.renderIndex(w, r, "", "", "", "", err.Error(), "", nil, &form)
		return
	}

	if _, err := h.triggerRulesSvc.Create(r.Context(), rule); err != nil {
		logger.Warn("create trigger rule failed", slog.Any("error", err))
		h.renderIndex(w, r, "", "", "", "", "Could not save trigger rule: "+err.Error(), "", nil, &form)
		return
	}

	http.Redirect(w, r, "/?status=trigger_rule_saved", http.StatusSeeOther)
}

func (h *UploadHandler) ToggleTriggerRule(w http.ResponseWriter, r *http.Request) {
	ruleID, err := triggerRuleIDFromRequest(r)
	if err != nil {
		http.Error(w, "invalid trigger rule id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderIndex(w, r, "", "", "", "", "Could not read the trigger rule action.", "", nil, nil)
		return
	}

	enabled := r.FormValue("enabled") == "true"
	if err := h.triggerRulesSvc.SetEnabled(r.Context(), ruleID, enabled); err != nil {
		h.renderIndex(w, r, "", "", "", "", "Could not update trigger rule: "+err.Error(), "", nil, nil)
		return
	}

	http.Redirect(w, r, "/?status=trigger_rule_updated", http.StatusSeeOther)
}

func (h *UploadHandler) DeleteTriggerRule(w http.ResponseWriter, r *http.Request) {
	ruleID, err := triggerRuleIDFromRequest(r)
	if err != nil {
		http.Error(w, "invalid trigger rule id", http.StatusBadRequest)
		return
	}

	if err := h.triggerRulesSvc.Delete(r.Context(), ruleID); err != nil {
		h.renderIndex(w, r, "", "", "", "", "Could not delete trigger rule: "+err.Error(), "", nil, nil)
		return
	}

	http.Redirect(w, r, "/?status=trigger_rule_deleted", http.StatusSeeOther)
}

func (h *UploadHandler) renderIndex(
	w http.ResponseWriter,
	r *http.Request,
	uploadError string,
	uploadSuccess string,
	settingsError string,
	settingsSuccess string,
	triggerRuleError string,
	triggerRuleSuccess string,
	settingsForm *TranscriptionSettingsForm,
	triggerRuleForm *TriggerRuleForm,
) {
	viewItems, err := h.buildMediaListItems(r.Context())
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("load media list failed", slog.Any("error", err))
		http.Error(w, "failed to load media list", http.StatusInternalServerError)
		return
	}
	currentProfile, err := h.transcriptionSvc.GetCurrent(r.Context())
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("load transcription settings failed", slog.Any("error", err))
		http.Error(w, "failed to load transcription settings", http.StatusInternalServerError)
		return
	}
	triggerRules, err := h.triggerRulesSvc.List(r.Context())
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("load trigger rules failed", slog.Any("error", err))
		http.Error(w, "failed to load trigger rules", http.StatusInternalServerError)
		return
	}
	currentForm := buildSettingsForm(currentProfile)
	if settingsForm != nil {
		currentForm = *settingsForm
	}
	currentTriggerRuleForm := TriggerRuleForm{MatchMode: string(domaintrigger.MatchModeContains)}
	if triggerRuleForm != nil {
		currentTriggerRuleForm = *triggerRuleForm
	}
	computeTypes := transcription.SupportedComputeTypes(strings.ToLower(strings.TrimSpace(currentForm.Device)))
	if len(computeTypes) == 0 {
		computeTypes = transcription.SupportedComputeTypes("cpu")
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := IndexViewData{
		Items:               viewItems,
		UploadError:         uploadError,
		UploadSuccess:       uploadSuccess,
		SettingsError:       settingsError,
		SettingsSuccess:     settingsSuccess,
		TriggerRuleError:    triggerRuleError,
		TriggerRuleSuccess:  triggerRuleSuccess,
		MaxUploadMB:         strconv.FormatInt(h.maxUploadSizeB/(1024*1024), 10),
		MaxUploadHuman:      HumanSize(h.maxUploadSizeB),
		SettingsForm:        currentForm,
		TriggerRuleForm:     currentTriggerRuleForm,
		TriggerRules:        buildTriggerRuleViews(triggerRules),
		BackendOptions:      backendOptions(),
		ModelOptions:        transcription.SupportedModels(),
		DeviceOptions:       transcription.SupportedDevices(),
		CurrentComputeTypes: computeTypes,
		CPUComputeTypes:     transcription.SupportedComputeTypes("cpu"),
		CUDAComputeTypes:    transcription.SupportedComputeTypes("cuda"),
		MatchModeOptions:    matchModeOptions(),
	}
	if execErr := h.tmpl.ExecuteTemplate(w, "index.html", data); execErr != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("render index template failed", slog.Any("error", execErr))
		http.Error(w, "failed to render page", http.StatusInternalServerError)
	}
}

func (h *UploadHandler) buildMediaListItems(ctx context.Context) ([]MediaListItem, error) {
	items, err := h.uploadUC.ListRecent(ctx, h.listItemsMaxSize)
	if err != nil {
		return nil, err
	}

	viewItems := make([]MediaListItem, 0, len(items))
	for _, item := range items {
		statusLabel, statusTone, stageLabel, stageValue, active := describeMediaStatus(item.Status)
		viewItems = append(viewItems, MediaListItem{
			ID:                item.ID,
			OriginalName:      item.OriginalName,
			Extension:         item.Extension,
			SizeHuman:         HumanSize(item.SizeBytes),
			Status:            item.Status,
			StatusLabel:       statusLabel,
			StatusTone:        statusTone,
			StageLabel:        stageLabel,
			StageValue:        stageValue,
			StageTotal:        5,
			StagePercent:      stagePercent(stageValue, 5),
			IsActive:          active,
			CreatedAtUTC:      item.CreatedAtUTC.UTC().Format("2006-01-02 15:04:05"),
			HasTranscript:     strings.TrimSpace(item.TranscriptText) != "",
			CanOpenTranscript: canOpenTranscript(item),
			TranscriptURL:     fmt.Sprintf("/media/%d/transcript", item.ID),
			DeleteURL:         fmt.Sprintf("/media/%d/delete", item.ID),
		})
	}

	return viewItems, nil
}

func canOpenTranscript(item media.Media) bool {
	if strings.TrimSpace(item.TranscriptText) != "" {
		return true
	}

	switch item.Status {
	case media.StatusTranscribing, media.StatusTranscribed, media.StatusFailed:
		return true
	default:
		return false
	}
}

func stagePercent(value int, total int) int {
	if value <= 0 || total <= 0 {
		return 0
	}

	return (value * 100) / total
}

func describeMediaStatus(status media.Status) (statusLabel string, statusTone string, stageLabel string, stageValue int, active bool) {
	switch status {
	case media.StatusUploaded:
		return "Uploaded", "uploaded", "Waiting for audio extraction", 1, false
	case media.StatusProcessing:
		return "Extracting audio", "running", "Extracting audio", 2, true
	case media.StatusAudioExtracted:
		return "Audio ready", "ready", "Audio extracted, waiting for transcription", 3, false
	case media.StatusTranscribing:
		return "Transcribing", "running", "Transcribing audio", 4, true
	case media.StatusTranscribed:
		return "Transcribed", "success", "Transcription completed", 5, false
	case media.StatusFailed:
		return "Failed", "error", "Processing failed", 0, false
	default:
		return string(status), "neutral", "Unknown status", 0, false
	}
}

func wantsJSON(r *http.Request) bool {
	accept := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept")))
	requestedWith := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Requested-With")))

	return strings.Contains(accept, "application/json") || requestedWith == "xmlhttprequest"
}

func (h *UploadHandler) writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *UploadHandler) uploadLimitMessage(prefix string) string {
	return fmt.Sprintf("%s Maximum supported size is %s.", strings.TrimSpace(prefix), HumanSize(h.maxUploadSizeB))
}

func (h *UploadHandler) userFacingUploadError(err error) string {
	if err == nil {
		return ""
	}

	msg := err.Error()
	switch {
	case strings.Contains(msg, "unsupported file format"):
		return "Unsupported file extension. Only the listed media formats are allowed."
	case strings.Contains(msg, "content type is not supported"):
		return "Uploaded content does not look like audio or video."
	case strings.Contains(msg, "empty file"):
		return "Empty upload is not allowed."
	case strings.Contains(msg, "exceeds max size"):
		return h.uploadLimitMessage("File exceeds the upload limit.")
	case strings.Contains(msg, "upload canceled"):
		return "Upload was canceled before completion."
	default:
		if errors.Is(err, http.ErrMissingFile) {
			return "Please choose a file to upload."
		}
		return "Upload failed: " + msg
	}
}

func normalizeMIMEType(value string) string {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return value
	}

	return mediaType
}

func parseTranscriptionProfileForm(r *http.Request) (transcription.Profile, TranscriptionSettingsForm, error) {
	form := buildSettingsFormFromRequest(r)
	beamSize, err := strconv.Atoi(strings.TrimSpace(r.FormValue("beam_size")))
	if err != nil {
		return transcription.Profile{}, *form, fmt.Errorf("Beam size must be a whole number.")
	}
	form.BeamSize = beamSize

	profile := transcription.Profile{
		Backend:     transcription.Backend(strings.TrimSpace(r.FormValue("backend"))),
		ModelName:   strings.TrimSpace(r.FormValue("model_name")),
		Device:      strings.TrimSpace(r.FormValue("device")),
		ComputeType: strings.TrimSpace(r.FormValue("compute_type")),
		Language:    strings.TrimSpace(r.FormValue("language")),
		BeamSize:    beamSize,
		VADEnabled:  r.FormValue("vad_enabled") == "on",
		IsDefault:   true,
	}
	profile = transcription.NormalizeProfile(profile)
	normalizedForm := buildSettingsForm(profile)

	if err := transcription.ValidateProfile(profile); err != nil {
		return transcription.Profile{}, normalizedForm, err
	}

	return profile, normalizedForm, nil
}

func buildSettingsForm(profile transcription.Profile) TranscriptionSettingsForm {
	return TranscriptionSettingsForm{
		Backend:     string(profile.Backend),
		ModelName:   profile.ModelName,
		Device:      profile.Device,
		ComputeType: profile.ComputeType,
		Language:    profile.Language,
		BeamSize:    profile.BeamSize,
		VADEnabled:  profile.VADEnabled,
	}
}

func buildSettingsFormFromRequest(r *http.Request) *TranscriptionSettingsForm {
	beamSize, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("beam_size")))
	return &TranscriptionSettingsForm{
		Backend:     strings.TrimSpace(r.FormValue("backend")),
		ModelName:   strings.TrimSpace(r.FormValue("model_name")),
		Device:      strings.TrimSpace(r.FormValue("device")),
		ComputeType: strings.TrimSpace(r.FormValue("compute_type")),
		Language:    strings.TrimSpace(r.FormValue("language")),
		BeamSize:    beamSize,
		VADEnabled:  r.FormValue("vad_enabled") == "on",
	}
}

func backendOptions() []string {
	items := transcription.SupportedBackends()
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, string(item))
	}
	return values
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
			EnabledLabel: "Disabled",
			EnabledTone:  "neutral",
			ToggleLabel:  "Enable",
		}
		if item.Enabled {
			view.EnabledLabel = "Enabled"
			view.EnabledTone = "success"
			view.ToggleLabel = "Disable"
		}
		views = append(views, view)
	}

	return views
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
