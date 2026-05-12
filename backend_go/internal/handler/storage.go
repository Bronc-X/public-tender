package handler

import (
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/gin-gonic/gin"
)

type StorageHandler struct{}

func NewStorageHandler() *StorageHandler {
	return &StorageHandler{}
}

func (h *StorageHandler) dbDirAbs() (string, error) {
	// Same as cmd/server/main.go dbPath
	dbFile := filepath.Join("data", "app.db")
	absFile, err := filepath.Abs(dbFile)
	if err != nil {
		return "", err
	}
	return filepath.Dir(absFile), nil
}

func (h *StorageHandler) incomingDirAbs() (string, error) {
	// Keep consistent with UploadFile() which writes under data/files/incoming/
	p := filepath.Join("data", "files", "incoming")
	if err := os.MkdirAll(p, 0o755); err != nil {
		return "", err
	}
	return filepath.Abs(p)
}

func (h *StorageHandler) GetStorageInfo(c *gin.Context) {
	incomingAbs, err := h.incomingDirAbs()
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to resolve incoming storage path")
		return
	}
	dbDirAbs, err := h.dbDirAbs()
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to resolve database storage path")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"incoming_dir": incomingAbs,
		"db_dir":       dbDirAbs,
	})
}

func (h *StorageHandler) OpenIncomingFolder(c *gin.Context) {
	incomingAbs, err := h.incomingDirAbs()
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to resolve incoming storage path")
		return
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", incomingAbs)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", incomingAbs)
	default:
		cmd = exec.Command("xdg-open", incomingAbs)
	}

	if startErr := cmd.Start(); startErr != nil {
		log.Printf("[Storage] open folder failed: %v", startErr)
		Error(c, http.StatusInternalServerError, "Failed to open folder")
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *StorageHandler) OpenDatabaseFolder(c *gin.Context) {
	dbDir, err := h.dbDirAbs()
	if err != nil {
		Error(c, http.StatusInternalServerError, "Failed to resolve database storage path")
		return
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", dbDir)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", dbDir)
	default:
		cmd = exec.Command("xdg-open", dbDir)
	}

	if startErr := cmd.Start(); startErr != nil {
		log.Printf("[Storage] open db folder failed: %v", startErr)
		Error(c, http.StatusInternalServerError, "Failed to open folder")
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

