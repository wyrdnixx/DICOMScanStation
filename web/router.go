package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"DICOMScanStation/config"
	"DICOMScanStation/scanner"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Router struct {
	router         *gin.Engine
	scannerManager *scanner.ScannerManager
	config         *config.Config
	logger         *logrus.Logger
}

func NewRouter(sm *scanner.ScannerManager, cfg *config.Config) *Router {
	router := gin.Default()

	// Set up CORS
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	return &Router{
		router:         router,
		scannerManager: sm,
		config:         cfg,
		logger:         logrus.New(),
	}
}

func (r *Router) SetupRoutes() {
	// Serve static files
	r.router.Static("/static", "./web/static")
	r.router.LoadHTMLGlob("web/templates/*")

	// API routes
	api := r.router.Group("/api")
	{
		api.GET("/scanners", r.getScanners)
		api.GET("/scanners/:device/capabilities", r.getScannerCapabilities)
		api.GET("/files", r.getFiles)
		api.POST("/scan", r.startScan)
		api.GET("/files/:filename", r.getFile)
		api.DELETE("/files/:filename", r.deleteFile)
	}

	// Web routes
	r.router.GET("/", r.indexPage)
}

func (r *Router) getScanners(c *gin.Context) {
	scanners := r.scannerManager.GetScanners()
	c.JSON(http.StatusOK, gin.H{
		"scanners": scanners,
		"total":    len(scanners),
	})
}

func (r *Router) getFiles(c *gin.Context) {
	files, err := r.getFileList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"files": files,
		"total": len(files),
	})
}

func (r *Router) startScan(c *gin.Context) {
	var req struct {
		Device  string                 `json:"device" binding:"required"`
		Options *scanner.ScanOptions   `json:"options"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device is required"})
		return
	}

	// Check if files already exist
	files, err := r.getFileList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(files) > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Files already exist. Please delete existing files before scanning.",
			"files": files,
		})
		return
	}

	filenames, err := r.scannerManager.ScanDocument(req.Device, req.Options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Scan completed successfully",
		"filenames": filenames,
		"pages":     len(filenames),
	})
}

func (r *Router) getFile(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Filename is required"})
		return
	}

	filepath := filepath.Join(r.config.TempFilesDir, filename)

	// Check if file exists
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	c.File(filepath)
}

func (r *Router) deleteFile(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Filename is required"})
		return
	}

	filepath := filepath.Join(r.config.TempFilesDir, filename)

	// Check if file exists
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Delete file
	if err := os.Remove(filepath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "File deleted successfully"})
}

func (r *Router) indexPage(c *gin.Context) {
	scanners := r.scannerManager.GetScanners()
	files, _ := r.getFileList()

	c.HTML(http.StatusOK, "index.html", gin.H{
		"title":    r.config.WebTitle,
		"scanners": scanners,
		"files":    files,
		"config":   r.config,
	})
}

func (r *Router) getFileList() ([]FileInfo, error) {
	var files []FileInfo

	entries, err := os.ReadDir(r.config.TempFilesDir)
	if err != nil {
		return files, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if r.isAllowedExtension(ext) {
				info, err := entry.Info()
				if err != nil {
					continue
				}

				files = append(files, FileInfo{
					Name:         entry.Name(),
					Size:         info.Size(),
					ModifiedTime: info.ModTime().Format("2006-01-02 15:04:05"),
					Extension:    ext,
				})
			}
		}
	}

	return files, nil
}

func (r *Router) isAllowedExtension(ext string) bool {
	for _, allowed := range r.config.AllowedExtensions {
		if "."+allowed == ext {
			return true
		}
	}
	return false
}

func (r *Router) getScannerCapabilities(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device parameter is required"})
		return
	}

	capabilities, err := r.scannerManager.GetScannerCapabilities(device)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"device":        device,
		"capabilities":  capabilities,
	})
}

func (r *Router) GetEngine() *gin.Engine {
	return r.router
}

type FileInfo struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	ModifiedTime string `json:"modified_time"`
	Extension    string `json:"extension"`
}
