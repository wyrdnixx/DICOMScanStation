package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"DICOMScanStation/config"
	"DICOMScanStation/dicom"
	"DICOMScanStation/scanner"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Router struct {
	router         *gin.Engine
	scannerManager *scanner.ScannerManager
	dicomService   *dicom.DicomService
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

	// Initialize DICOM service
	dicomService := dicom.NewDicomService(cfg)

	return &Router{
		router:         router,
		scannerManager: sm,
		dicomService:   dicomService,
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
		// DICOM endpoints
		api.GET("/dicom/search", r.searchPatients)
		api.POST("/dicom/send", r.sendToPacs)
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
		Device  string               `json:"device" binding:"required"`
		Options *scanner.ScanOptions `json:"options"`
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
	capabilities, err := r.scannerManager.GetScannerCapabilities(device)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, capabilities)
}

func (r *Router) searchPatients(c *gin.Context) {
	searchTerm := c.Query("q")
	searchType := c.Query("type")

	if searchTerm == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search term is required"})
		return
	}

	// Default to name search if type is not specified
	if searchType == "" {
		searchType = "name"
	}

	r.logger.Infof("Searching for patients with term: %s (type: %s)", searchTerm, searchType)

	patients, err := r.dicomService.SearchPatients(searchTerm, searchType)
	if err != nil {
		r.logger.Errorf("Patient search failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"patients": patients,
		"total":    len(patients),
	})
}

func (r *Router) sendToPacs(c *gin.Context) {
	var req struct {
		PatientIDs      []string          `json:"patientIds" binding:"required"`
		DocumentCreator string            `json:"documentCreator" binding:"required"`
		SelectedPatient dicom.PatientInfo `json:"selectedPatient" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Patient IDs, document creator, and selected patient are required"})
		return
	}

	// Get list of scanned files
	files, err := r.getFileList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get file list"})
		return
	}

	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No scanned files to send"})
		return
	}

	// Build file paths
	var filePaths []string
	for _, file := range files {
		filePaths = append(filePaths, filepath.Join(r.config.TempFilesDir, file.Name))
	}

	r.logger.Infof("Sending %d files to patient: %+v", len(filePaths), req.SelectedPatient)

	err = r.dicomService.SendToPacs(req.PatientIDs, req.DocumentCreator, filePaths, req.SelectedPatient)
	if err != nil {
		r.logger.Errorf("Failed to send to PACS: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Files sent to PACS successfully",
		"files":   len(filePaths),
		"patient": req.SelectedPatient.Name,
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
