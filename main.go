package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"DICOMScanStation/config"
	"DICOMScanStation/scanner"
	"DICOMScanStation/web"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

var (
	logger *logrus.Logger
	cfg    *config.Config
)

func main() {
	// Initialize logger
	logger = logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		logger.Warn("No .env file found, using system environment variables")
	}

	// Load configuration
	cfg = config.LoadConfig()

	// Set log level
	if level, err := logrus.ParseLevel(cfg.LogLevel); err == nil {
		logger.SetLevel(level)
	}

	logger.Info("Starting DICOMScanStation...")

	// Create temp directory
	if err := os.MkdirAll(cfg.TempFilesDir, 0755); err != nil {
		logger.Fatalf("Failed to create temp directory: %v", err)
	}

	// Initialize scanner manager
	scannerManager := scanner.NewScannerManager(cfg)
	go scannerManager.StartMonitoring()

	// Initialize web server
	router := setupRouter(scannerManager, cfg)

	// Create HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", cfg.AppHost, cfg.AppPort),
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		logger.Infof("Starting web server on %s:%s", cfg.AppHost, cfg.AppPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Create a deadline for server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown scanner manager
	scannerManager.Stop()

	// Shutdown server
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown:", err)
	}

	logger.Info("Server exited")
}

func setupRouter(scannerManager *scanner.ScannerManager, cfg *config.Config) *gin.Engine {
	router := web.NewRouter(scannerManager, cfg)
	router.SetupRoutes()
	return router.GetEngine()
}
