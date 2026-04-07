package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"media-pipeline/internal/app/command"
	mediaapp "media-pipeline/internal/app/media"
	"media-pipeline/internal/domain/job"
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
	summaryRequestUC SummaryRequestService
	deleteMediaUC    MediaDeletionService
	jobReader        MediaJobReader
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

type SummaryRequestService interface {
	Request(ctx context.Context, mediaID int64) (mediaapp.RequestSummaryResult, error)
}

type MediaDeletionService interface {
	Delete(ctx context.Context, mediaID int64) (mediaapp.DeleteMediaResult, error)
}

type MediaJobReader interface {
	ListByMediaID(ctx context.Context, mediaID int64) ([]job.Job, error)
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
	CurrentStage      string
	CurrentTimingText string
	FailedStage       string
	ErrorSummary      string
	ErrorLocation     string
	Steps             []PipelineStepView
	CreatedAtUTC      string
	CanOpenTranscript bool
	HasTranscript     bool
	TriggerCount      int
	TranscriptURL     string
	DeleteURL         string
}

type IndexViewData struct {
	Spotlight           *MediaListItem
	Items               []MediaListItem
	ItemsTotal          int
	ItemsActive         int
	UploadError         string
	UploadSuccess       string
	SettingsError       string
	SettingsSuccess     string
	SettingsPanelOpen   bool
	SettingsWarnings    []string
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
	Backend     string `json:"backend"`
	ModelName   string `json:"modelName"`
	Device      string `json:"device"`
	ComputeType string `json:"computeType"`
	Language    string `json:"language"`
	BeamSize    int    `json:"beamSize"`
	VADEnabled  bool   `json:"vadEnabled"`
	UITheme     string `json:"uiTheme"`
}

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

func NewUploadHandler(
	uploadUC *command.UploadMediaUseCase,
	transcriptionSvc TranscriptionSettingsService,
	triggerRulesSvc TriggerRulesService,
	transcriptViewUC TranscriptViewService,
	summaryRequestUC SummaryRequestService,
	deleteMediaUC MediaDeletionService,
	jobReader MediaJobReader,
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
		summaryRequestUC: summaryRequestUC,
		deleteMediaUC:    deleteMediaUC,
		jobReader:        jobReader,
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
		successMessage = "Файл загружен. Запись создана, задача на извлечение аудио поставлена в очередь."
	case "deleted":
		successMessage = "Файл удалён. Связанные записи очищены, временные файлы тоже были удалены."
	case "trigger_rule_saved":
		triggerRuleSuccess = "Правило триггера создано."
	case "trigger_rule_updated":
		triggerRuleSuccess = "Статус правила триггера обновлён."
	case "trigger_rule_deleted":
		triggerRuleSuccess = "Правило триггера удалено."
	}
	settingsSuccess := ""
	if r.URL.Query().Get("status") == "settings_saved" {
		settingsSuccess = "Настройки распознавания сохранены."
	}

	h.renderIndex(w, r, "", successMessage, "", settingsSuccess, "", triggerRuleSuccess, nil, nil)
}

func (h *UploadHandler) Workspace(w http.ResponseWriter, r *http.Request) {
	profile, err := h.transcriptionSvc.GetCurrent(r.Context())
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("load workspace preference failed", slog.Any("error", err))
		http.Redirect(w, r, "/app-v1", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, preferredAppURL(profile.UITheme), http.StatusSeeOther)
}

func (h *UploadHandler) Upload(w http.ResponseWriter, r *http.Request) {
	logger := observability.LoggerFromContext(r.Context(), h.logger).With(
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
	)
	startedAtUTC := time.Now().UTC()

	r.Body = http.MaxBytesReader(w, r.Body, h.maxRequestBodyB)
	if err := r.ParseMultipartForm(h.maxFormMemoryB); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			logger.Warn("upload request rejected: request body too large", slog.Any("error", err))
			h.renderUploadFailure(w, r, h.uploadLimitMessage("Файл слишком большой."))
			return
		}

		logger.Warn("upload request rejected: invalid multipart form", slog.Any("error", err))
		h.renderUploadFailure(w, r, "Не удалось прочитать форму загрузки. Выберите файл ещё раз.")
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
		h.renderUploadFailure(w, r, "Выберите файл для загрузки.")
		return
	}
	defer file.Close()

	if strings.TrimSpace(fileHeader.Filename) == "" {
		h.renderUploadFailure(w, r, "Имя файла обязательно.")
		return
	}
	if fileHeader.Size == 0 {
		h.renderUploadFailure(w, r, "Пустой файл загружать нельзя.")
		return
	}

	logger.Info("upload accepted",
		slog.String("filename", fileHeader.Filename),
		slog.Int64("declared_size_bytes", fileHeader.Size),
	)

	result, err := h.uploadUC.Upload(r.Context(), command.UploadMediaInput{
		OriginalName:        fileHeader.Filename,
		MIMEType:            normalizeMIMEType(fileHeader.Header.Get("Content-Type")),
		SizeBytes:           fileHeader.Size,
		Content:             file,
		StartedAtUTC:        startedAtUTC,
		RuntimeSnapshotJSON: buildRuntimeSnapshotJSON(r),
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
			"message": "Файл загружен. Запись создана, задача поставлена в очередь.",
		})
		return
	}
	http.Redirect(w, r, "/?status=uploaded", http.StatusSeeOther)
}

func (h *UploadHandler) MediaStatuses(w http.ResponseWriter, r *http.Request) {
	items, err := h.buildMediaListItems(r.Context())
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("load media statuses failed", slog.Any("error", err))
		http.Error(w, "не удалось загрузить статусы файлов", http.StatusInternalServerError)
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
		h.renderIndex(w, r, "", "", "Не удалось прочитать форму настроек. Попробуйте ещё раз.", "", "", "", buildSettingsFormFromRequest(r), nil)
		return
	}

	profile, form, err := parseTranscriptionProfileForm(r)
	if err != nil {
		h.renderIndex(w, r, "", "", err.Error(), "", "", "", &form, nil)
		return
	}

	if _, err := h.transcriptionSvc.SaveCurrent(r.Context(), profile); err != nil {
		logger.Warn("save transcription settings failed", slog.Any("error", err))
		h.renderIndex(w, r, "", "", "Не удалось сохранить настройки: "+err.Error(), "", "", "", &form, nil)
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
		h.renderIndex(w, r, "", "", "", "", "Не удалось прочитать форму правила триггера. Попробуйте ещё раз.", "", nil, buildTriggerRuleFormFromRequest(r))
		return
	}

	rule, form, err := parseTriggerRuleForm(r)
	if err != nil {
		h.renderIndex(w, r, "", "", "", "", err.Error(), "", nil, &form)
		return
	}

	if _, err := h.triggerRulesSvc.Create(r.Context(), rule); err != nil {
		logger.Warn("create trigger rule failed", slog.Any("error", err))
		h.renderIndex(w, r, "", "", "", "", "Не удалось сохранить правило триггера: "+err.Error(), "", nil, &form)
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
		h.renderIndex(w, r, "", "", "", "", "Не удалось прочитать действие для правила триггера.", "", nil, nil)
		return
	}

	enabled := r.FormValue("enabled") == "true"
	if err := h.triggerRulesSvc.SetEnabled(r.Context(), ruleID, enabled); err != nil {
		h.renderIndex(w, r, "", "", "", "", "Не удалось обновить правило триггера: "+err.Error(), "", nil, nil)
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
		h.renderIndex(w, r, "", "", "", "", "Не удалось удалить правило триггера: "+err.Error(), "", nil, nil)
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
		http.Error(w, "не удалось загрузить список файлов", http.StatusInternalServerError)
		return
	}
	currentProfile, err := h.transcriptionSvc.GetCurrent(r.Context())
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("load transcription settings failed", slog.Any("error", err))
		http.Error(w, "не удалось загрузить настройки распознавания", http.StatusInternalServerError)
		return
	}
	triggerRules, err := h.triggerRulesSvc.List(r.Context())
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("load trigger rules failed", slog.Any("error", err))
		http.Error(w, "не удалось загрузить правила триггеров", http.StatusInternalServerError)
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
	spotlight := pickSpotlightItem(viewItems)
	data := IndexViewData{
		Spotlight:           spotlight,
		Items:               viewItems,
		ItemsTotal:          len(viewItems),
		ItemsActive:         countActiveItems(viewItems),
		UploadError:         uploadError,
		UploadSuccess:       uploadSuccess,
		SettingsError:       settingsError,
		SettingsSuccess:     settingsSuccess,
		SettingsPanelOpen:   settingsError != "" || settingsSuccess != "" || triggerRuleError != "" || triggerRuleSuccess != "",
		SettingsWarnings:    buildSettingsWarnings(currentForm),
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
		http.Error(w, "не удалось отрисовать страницу", http.StatusInternalServerError)
	}
}

func (h *UploadHandler) buildMediaListItems(ctx context.Context) ([]MediaListItem, error) {
	items, err := h.uploadUC.ListRecent(ctx, h.listItemsMaxSize)
	if err != nil {
		return nil, err
	}

	viewItems := make([]MediaListItem, 0, len(items))
	for _, item := range items {
		jobs, err := h.jobReader.ListByMediaID(ctx, item.ID)
		if err != nil {
			return nil, fmt.Errorf("load jobs for media %d: %w", item.ID, err)
		}
		pipelineView := buildMediaPipelineView(item, jobs)
		viewItems = append(viewItems, MediaListItem{
			ID:                item.ID,
			OriginalName:      item.OriginalName,
			Extension:         item.Extension,
			SizeHuman:         HumanSize(item.SizeBytes),
			Status:            item.Status,
			StatusLabel:       pipelineView.StatusLabel,
			StatusTone:        pipelineView.StatusTone,
			StageLabel:        pipelineView.StageLabel,
			StageValue:        pipelineView.StageValue,
			StageTotal:        pipelineView.StageTotal,
			StagePercent:      stagePercent(pipelineView.StageValue, pipelineView.StageTotal),
			IsActive:          pipelineView.IsActive,
			CurrentStage:      pipelineView.CurrentStage,
			CurrentTimingText: pipelineView.CurrentTimingText,
			FailedStage:       pipelineView.FailedStage,
			ErrorSummary:      pipelineView.ErrorSummary,
			ErrorLocation:     pipelineView.ErrorLocation,
			Steps:             pipelineView.Steps,
			CreatedAtUTC:      item.CreatedAtUTC.UTC().Format("2006-01-02 15:04:05"),
			HasTranscript:     strings.TrimSpace(item.TranscriptText) != "",
			CanOpenTranscript: canOpenTranscript(item),
			TranscriptURL:     fmt.Sprintf("/media/%d/transcript", item.ID),
			DeleteURL:         fmt.Sprintf("/media/%d/delete", item.ID),
		})
	}

	return viewItems, nil
}

func pickSpotlightItem(items []MediaListItem) *MediaListItem {
	for _, item := range items {
		if item.IsActive {
			current := item
			return &current
		}
	}
	if len(items) == 0 {
		return nil
	}

	current := items[0]
	return &current
}

func countActiveItems(items []MediaListItem) int {
	total := 0
	for _, item := range items {
		if item.IsActive {
			total++
		}
	}

	return total
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
	return fmt.Sprintf("%s Максимальный размер файла: %s.", strings.TrimSpace(prefix), HumanSize(h.maxUploadSizeB))
}

func (h *UploadHandler) userFacingUploadError(err error) string {
	if err == nil {
		return ""
	}

	msg := err.Error()
	switch {
	case strings.Contains(msg, "unsupported file format"):
		return "Неподдерживаемое расширение файла. Разрешены только указанные аудио- и видеоформаты."
	case strings.Contains(msg, "content type is not supported"):
		return "Файл не похож на аудио или видео."
	case strings.Contains(msg, "empty file"):
		return "Пустой файл загружать нельзя."
	case strings.Contains(msg, "exceeds max size"):
		return h.uploadLimitMessage("Файл превышает допустимый размер.")
	case strings.Contains(msg, "upload canceled"):
		return "Загрузка была отменена до завершения."
	default:
		if errors.Is(err, http.ErrMissingFile) {
			return "Выберите файл для загрузки."
		}
		return "Ошибка загрузки: " + msg
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
		return transcription.Profile{}, *form, fmt.Errorf("Поле beam_size должно быть целым числом.")
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
		UITheme:     strings.TrimSpace(r.FormValue("ui_theme")),
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
		UITheme:     profile.UITheme,
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
		UITheme:     strings.TrimSpace(r.FormValue("ui_theme")),
	}
}

func buildSettingsWarnings(form TranscriptionSettingsForm) []string {
	settings := transcription.NormalizeSettings(transcription.Settings{
		Backend:     transcription.Backend(form.Backend),
		ModelName:   form.ModelName,
		Device:      form.Device,
		ComputeType: form.ComputeType,
		Language:    form.Language,
		BeamSize:    form.BeamSize,
		VADEnabled:  form.VADEnabled,
	})
	return transcription.BuildRuntimeSettingsWarnings(settings)
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

func buildRuntimeSnapshotJSON(r *http.Request) string {
	snapshot := media.RuntimeSnapshot{
		CapturedAtUTC:      time.Now().UTC(),
		RequestIP:          requestIPFromRequest(r),
		UserAgent:          strings.TrimSpace(r.UserAgent()),
		AcceptLanguage:     strings.TrimSpace(r.Header.Get("Accept-Language")),
		ClientLanguage:     strings.TrimSpace(r.FormValue("client_language")),
		ClientPlatform:     strings.TrimSpace(r.FormValue("client_platform")),
		ClientHintPlatform: strings.Trim(strings.TrimSpace(r.Header.Get("Sec-CH-UA-Platform")), `"`),
		ClientHintMobile:   strings.Trim(strings.TrimSpace(r.Header.Get("Sec-CH-UA-Mobile")), `"`),
		ClientHintArch:     strings.Trim(strings.TrimSpace(r.Header.Get("Sec-CH-UA-Arch")), `"`),
		ClientHintBitness:  strings.Trim(strings.TrimSpace(r.Header.Get("Sec-CH-UA-Bitness")), `"`),
	}

	if value, ok := parseOptionalInt(strings.TrimSpace(r.FormValue("hardware_concurrency"))); ok {
		snapshot.HardwareConcurrency = &value
	}
	if value, ok := parseOptionalFloat(strings.TrimSpace(r.FormValue("device_memory_gb"))); ok {
		snapshot.DeviceMemoryGB = &value
	}
	if value, ok := parseOptionalInt(strings.TrimSpace(r.FormValue("timezone_offset_minutes"))); ok {
		snapshot.TimezoneOffsetMinutes = &value
	}

	raw, err := media.EncodeRuntimeSnapshot(snapshot)
	if err != nil {
		return ""
	}

	return raw
}

func parseOptionalInt(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}

	return value, true
}

func parseOptionalFloat(raw string) (float64, bool) {
	if raw == "" {
		return 0, false
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}

	return value, true
}

func requestIPFromRequest(r *http.Request) string {
	hostPort := strings.TrimSpace(r.RemoteAddr)
	if hostPort == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(hostPort)
	if err == nil {
		return host
	}

	return hostPort
}
