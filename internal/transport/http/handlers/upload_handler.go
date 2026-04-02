package handlers

import (
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
	"media-pipeline/internal/observability"
)

type UploadHandler struct {
	uploadUC         *command.UploadMediaUseCase
	tmpl             *template.Template
	maxUploadSizeB   int64
	maxRequestBodyB  int64
	maxFormMemoryB   int64
	listItemsMaxSize int
	logger           *slog.Logger
}

type MediaListItem struct {
	ID           int64
	OriginalName string
	Extension    string
	SizeHuman    string
	Status       media.Status
	CreatedAtUTC string
}

type IndexViewData struct {
	Items          []MediaListItem
	Error          string
	Success        string
	MaxUploadMB    string
	MaxUploadHuman string
}

func NewUploadHandler(uploadUC *command.UploadMediaUseCase, templatePath string, maxUploadSizeB int64, logger *slog.Logger) (*UploadHandler, error) {
	tmpl, err := template.New("index.html").Funcs(template.FuncMap{
		"humanSize": HumanSize,
	}).ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("parse index template: %w", err)
	}

	return &UploadHandler{
		uploadUC:         uploadUC,
		tmpl:             tmpl,
		maxUploadSizeB:   maxUploadSizeB,
		maxRequestBodyB:  maxUploadSizeB + (1 << 20),
		maxFormMemoryB:   32 << 20,
		listItemsMaxSize: 100,
		logger:           logger,
	}, nil
}

func (h *UploadHandler) Index(w http.ResponseWriter, r *http.Request) {
	successMessage := ""
	if r.URL.Query().Get("status") == "uploaded" {
		successMessage = "Upload completed. Media record and pending job were created."
	}

	h.renderIndex(w, r, "", successMessage)
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
			h.renderIndex(w, r, "File is too large. Please check the upload limit and try again.", "")
			return
		}

		logger.Warn("upload request rejected: invalid multipart form", slog.Any("error", err))
		h.renderIndex(w, r, "Could not read the upload form. Please choose the file again.", "")
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
		h.renderIndex(w, r, "Please choose a file in the media field.", "")
		return
	}
	defer file.Close()

	if strings.TrimSpace(fileHeader.Filename) == "" {
		h.renderIndex(w, r, "File name is required.", "")
		return
	}
	if fileHeader.Size == 0 {
		h.renderIndex(w, r, "Empty upload is not allowed.", "")
		return
	}

	logger.Info("upload accepted",
		slog.String("filename", fileHeader.Filename),
		slog.Int64("declared_size_bytes", fileHeader.Size),
	)

	err = h.uploadUC.Upload(r.Context(), command.UploadMediaInput{
		OriginalName: fileHeader.Filename,
		MIMEType:     normalizeMIMEType(fileHeader.Header.Get("Content-Type")),
		SizeBytes:    fileHeader.Size,
		Content:      file,
	})
	if err != nil {
		logger.Warn("upload failed", slog.Any("error", err), slog.String("filename", fileHeader.Filename))
		h.renderIndex(w, r, userFacingUploadError(err), "")
		return
	}

	logger.Info("upload succeeded", slog.String("filename", fileHeader.Filename))
	http.Redirect(w, r, "/?status=uploaded", http.StatusSeeOther)
}

func (h *UploadHandler) renderIndex(w http.ResponseWriter, r *http.Request, errMessage, successMessage string) {
	items, err := h.uploadUC.ListRecent(r.Context(), h.listItemsMaxSize)
	if err != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("load media list failed", slog.Any("error", err))
		http.Error(w, "failed to load media list", http.StatusInternalServerError)
		return
	}

	viewItems := make([]MediaListItem, 0, len(items))
	for _, item := range items {
		viewItems = append(viewItems, MediaListItem{
			ID:           item.ID,
			OriginalName: item.OriginalName,
			Extension:    item.Extension,
			SizeHuman:    HumanSize(item.SizeBytes),
			Status:       item.Status,
			CreatedAtUTC: item.CreatedAtUTC.UTC().Format("2006-01-02 15:04:05"),
		})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := IndexViewData{
		Items:          viewItems,
		Error:          errMessage,
		Success:        successMessage,
		MaxUploadMB:    strconv.FormatInt(h.maxUploadSizeB/(1024*1024), 10),
		MaxUploadHuman: HumanSize(h.maxUploadSizeB),
	}
	if execErr := h.tmpl.ExecuteTemplate(w, "index.html", data); execErr != nil {
		observability.LoggerFromContext(r.Context(), h.logger).Error("render index template failed", slog.Any("error", execErr))
		http.Error(w, "failed to render page", http.StatusInternalServerError)
	}
}

func userFacingUploadError(err error) string {
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
		return "File exceeds the configured upload limit."
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
