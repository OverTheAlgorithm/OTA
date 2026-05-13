package handler

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Limits on uploaded inline images. 5 MB matches our existing collector images.
const (
	maxUploadBytes = 5 * 1024 * 1024
	editorImageDir = "editor"
)

// allowedImageMIMEs whitelists the MIME types we accept for inline images. The
// values must match what http.DetectContentType produces from the file's magic
// bytes, not the browser-supplied Content-Type.
var allowedImageMIMEs = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// EditorUploadHandler handles inline image uploads from the rich-text editor.
type EditorUploadHandler struct {
	baseDir   string // local disk root (matches collector imageBaseDir)
	publicURL string // URL prefix that maps to baseDir (e.g. /api/v1/images)
}

func NewEditorUploadHandler(baseDir, publicURL string) *EditorUploadHandler {
	return &EditorUploadHandler{baseDir: baseDir, publicURL: publicURL}
}

func (h *EditorUploadHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/upload-image", h.Upload)
}

// Upload handles POST /api/v1/editor/upload-image (multipart, field "file").
func (h *EditorUploadHandler) Upload(c *gin.Context) {
	// Cap the request body so a hostile client cannot stream gigabytes before
	// we even see the form header.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes+1024)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "파일이 첨부되지 않았습니다"})
		return
	}
	defer file.Close()

	if header.Size > maxUploadBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "파일 크기는 5MB를 넘을 수 없습니다"})
		return
	}

	// Sniff magic bytes — never trust browser-supplied Content-Type.
	head := make([]byte, 512)
	n, err := file.Read(head)
	if err != nil && err != io.EOF {
		slog.Error("editor upload sniff error", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "파일을 읽을 수 없습니다"})
		return
	}
	mime := http.DetectContentType(head[:n])
	ext, ok := allowedImageMIMEs[mime]
	if !ok {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "지원하지 않는 이미지 형식입니다 (jpeg, png, webp, gif만 가능)"})
		return
	}

	// Rewind so we can copy the full payload to disk.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		slog.Error("editor upload seek error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "파일 처리 중 오류가 발생했습니다"})
		return
	}

	relPath := buildEditorImagePath(uuid.New(), ext)
	absPath := filepath.Join(h.baseDir, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		slog.Error("editor upload mkdir error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "디렉토리 생성에 실패했습니다"})
		return
	}

	out, err := os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		slog.Error("editor upload create error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "파일을 저장할 수 없습니다"})
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		slog.Error("editor upload copy error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "파일 저장 중 오류가 발생했습니다"})
		return
	}

	// Convert backslashes (Windows) to forward slashes for the URL.
	url := h.publicURL + "/" + filepath.ToSlash(relPath)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"url": url}})
}

// buildEditorImagePath returns editor/YYYY/MM/{uuid}{ext}. KST date to keep the
// layout consistent with the collector's image paths.
func buildEditorImagePath(id uuid.UUID, ext string) string {
	now := time.Now().UTC().Add(9 * time.Hour) // KST
	return filepath.Join(
		editorImageDir,
		now.Format("2006"),
		now.Format("01"),
		fmt.Sprintf("%s%s", id.String(), ext),
	)
}
