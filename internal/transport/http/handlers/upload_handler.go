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
	"strconv"
	"strings"

	"media-pipeline/internal/app/command"
	"media-pipeline/internal/domain/media"
	"media-pipeline/internal/domain/transcription"
	"media-pipeline/internal/observability"
)

type UploadHandler struct {
	uploadUC         *command.UploadMediaUseCase
	transcriptionSvc TranscriptionSettingsService
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

type MediaListItem struct {
	ID           int64
	OriginalName string
	Extension    string
	SizeHuman    string
	Status       media.Status
	StatusLabel  string
	StatusTone   string
	StageLabel   string
	StageValue   int
	StageTotal   int
	StagePercent int
	IsActive     bool
	CreatedAtUTC string
}

type IndexViewData struct {
	Items               []MediaListItem
	UploadError         string
	UploadSuccess       string
	SettingsError       string
	SettingsSuccess     string
	MaxUploadMB         string
	MaxUploadHuman      string
	SettingsForm        TranscriptionSettingsForm
	BackendOptions      []string
	ModelOptions        []string
	DeviceOptions       []string
	CurrentComputeTypes []string
	CPUComputeTypes     []string
	CUDAComputeTypes    []string
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

func NewUploadHandler(
	uploadUC *command.UploadMediaUseCase,
	transcriptionSvc TranscriptionSettingsService,
	templatePath string,
	maxUploadSizeB int64,
	logger *slog.Logger,
) (*UploadHandler, error) {
	tmpl, err := template.New("index.html").Funcs(template.FuncMap{
		"humanSize": HumanSize,
	}).ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("parse index template: %w", err)
	}

	return &UploadHandler{
		uploadUC:         uploadUC,
		transcriptionSvc: transcriptionSvc,
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
	if r.URL.Query().Get("status") == "uploaded" {
		successMessage = "Upload completed. Media record and pending job were created."
	}
	settingsSuccess := ""
	if r.URL.Query().Get("status") == "settings_saved" {
		settingsSuccess = "Transcription settings were saved."
	}

	h.renderIndex(w, r, "", successMessage, "", settingsSuccess, nil)
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

	h.renderIndex(w, r, message, "", "", "", nil)
}

func (h *UploadHandler) SaveTranscriptionSettings(w http.ResponseWriter, r *http.Request) {
	logger := observability.LoggerFromContext(r.Context(), h.logger).With(
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
	)

	r.Body = http.MaxBytesReader(w, r.Body, h.maxSettingsBodyB)
	if err := r.ParseForm(); err != nil {
		logger.Warn("settings request rejected: invalid form", slog.Any("error", err))
		h.renderIndex(w, r, "", "", "Could not read the settings form. Please try again.", "", buildSettingsFormFromRequest(r))
		return
	}

	profile, form, err := parseTranscriptionProfileForm(r)
	if err != nil {
		h.renderIndex(w, r, "", "", err.Error(), "", &form)
		return
	}

	if _, err := h.transcriptionSvc.SaveCurrent(r.Context(), profile); err != nil {
		logger.Warn("save transcription settings failed", slog.Any("error", err))
		h.renderIndex(w, r, "", "", "Could not save settings: "+err.Error(), "", &form)
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

func (h *UploadHandler) renderIndex(
	w http.ResponseWriter,
	r *http.Request,
	uploadError string,
	uploadSuccess string,
	settingsError string,
	settingsSuccess string,
	settingsForm *TranscriptionSettingsForm,
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
	currentForm := buildSettingsForm(currentProfile)
	if settingsForm != nil {
		currentForm = *settingsForm
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
		MaxUploadMB:         strconv.FormatInt(h.maxUploadSizeB/(1024*1024), 10),
		MaxUploadHuman:      HumanSize(h.maxUploadSizeB),
		SettingsForm:        currentForm,
		BackendOptions:      backendOptions(),
		ModelOptions:        transcription.SupportedModels(),
		DeviceOptions:       transcription.SupportedDevices(),
		CurrentComputeTypes: computeTypes,
		CPUComputeTypes:     transcription.SupportedComputeTypes("cpu"),
		CUDAComputeTypes:    transcription.SupportedComputeTypes("cuda"),
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
			ID:           item.ID,
			OriginalName: item.OriginalName,
			Extension:    item.Extension,
			SizeHuman:    HumanSize(item.SizeBytes),
			Status:       item.Status,
			StatusLabel:  statusLabel,
			StatusTone:   statusTone,
			StageLabel:   stageLabel,
			StageValue:   stageValue,
			StageTotal:   5,
			StagePercent: stagePercent(stageValue, 5),
			IsActive:     active,
			CreatedAtUTC: item.CreatedAtUTC.UTC().Format("2006-01-02 15:04:05"),
		})
	}

	return viewItems, nil
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
