package scanner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"DICOMScanStation/config"

	"github.com/sirupsen/logrus"
)

type ScannerInfo struct {
	Name      string `json:"name"`
	Device    string `json:"device"`
	Connected bool   `json:"connected"`
	Status    string `json:"status"`
	LastSeen  string `json:"last_seen"`
}

type ScanOptions struct {
	MultiPage  bool `json:"multi_page"`
	Duplex     bool `json:"duplex"`
	Color      bool `json:"color"`
	Resolution int  `json:"resolution"`
}

type ScannerManager struct {
	config   *config.Config
	logger   *logrus.Logger
	scanners map[string]*ScannerInfo
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	stopChan chan struct{}
}

func NewScannerManager(cfg *config.Config) *ScannerManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &ScannerManager{
		config:   cfg,
		logger:   logrus.New(),
		scanners: make(map[string]*ScannerInfo),
		ctx:      ctx,
		cancel:   cancel,
		stopChan: make(chan struct{}),
	}
}

func (sm *ScannerManager) StartMonitoring() {
	sm.logger.Info("Starting scanner monitoring...")

	ticker := time.NewTicker(time.Duration(sm.config.ScannerPollInterval) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			sm.logger.Info("Scanner monitoring stopped")
			return
		case <-ticker.C:
			sm.detectScanners()
		}
	}
}

func (sm *ScannerManager) Stop() {
	sm.logger.Info("Stopping scanner monitoring...")
	sm.cancel()
	close(sm.stopChan)
}

func (sm *ScannerManager) detectScanners() {
	// Use sane-find-scanner to detect USB scanners
	cmd := exec.Command("scanimage", "-L")
	output, err := cmd.Output()

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err != nil {
		sm.logger.Warnf("Failed to detect scanners: %v", err)
		// Mark all scanners as disconnected
		for _, scanner := range sm.scanners {
			scanner.Connected = false
			scanner.Status = "disconnected"
		}
		return
	}

	// Parse scanner output
	lines := strings.Split(string(output), "\n")
	currentScanners := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse lines like: device `fujitsu:fi-7030:211822' is a FUJITSU fi-7030 scanner
		if strings.Contains(line, "device") && strings.Contains(line, "is a") {
			// Extract device name (between backticks)
			deviceStart := strings.Index(line, "`")
			deviceEnd := strings.LastIndex(line, "'")
			if deviceStart == -1 || deviceEnd == -1 || deviceEnd <= deviceStart {
				continue
			}
			device := line[deviceStart+1 : deviceEnd]

			// Extract scanner name (after "is a")
			nameStart := strings.Index(line, "is a ")
			if nameStart == -1 {
				continue
			}
			name := strings.TrimSpace(line[nameStart+5:])

			currentScanners[device] = true

			if scanner, exists := sm.scanners[device]; exists {
				scanner.Connected = true
				scanner.Status = "connected"
				scanner.LastSeen = time.Now().Format(time.RFC3339)
			} else {
				sm.scanners[device] = &ScannerInfo{
					Name:      name,
					Device:    device,
					Connected: true,
					Status:    "connected",
					LastSeen:  time.Now().Format(time.RFC3339),
				}
				sm.logger.Infof("New scanner detected: %s (%s)", name, device)
			}
		}
	}

	// Mark scanners as disconnected if not found
	for device, scanner := range sm.scanners {
		if !currentScanners[device] {
			scanner.Connected = false
			scanner.Status = "disconnected"
		}
	}
}

func (sm *ScannerManager) GetScanners() []*ScannerInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	scanners := make([]*ScannerInfo, 0, len(sm.scanners))
	for _, scanner := range sm.scanners {
		scanners = append(scanners, scanner)
	}

	// Sort scanners alphabetically by name
	sort.Slice(scanners, func(i, j int) bool {
		return strings.ToLower(scanners[i].Name) < strings.ToLower(scanners[j].Name)
	})

	return scanners
}

func (sm *ScannerManager) GetConnectedScanners() []*ScannerInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var connected []*ScannerInfo
	for _, scanner := range sm.scanners {
		if scanner.Connected {
			connected = append(connected, scanner)
		}
	}

	// Sort connected scanners alphabetically by name
	sort.Slice(connected, func(i, j int) bool {
		return strings.ToLower(connected[i].Name) < strings.ToLower(connected[j].Name)
	})

	return connected
}

func (sm *ScannerManager) ScanDocument(device string, options *ScanOptions) ([]string, error) {
	sm.mu.RLock()
	scanner, exists := sm.scanners[device]
	sm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("scanner device '%s' not found", device)
	}
	if !scanner.Connected {
		return nil, fmt.Errorf("scanner '%s' is not connected", scanner.Name)
	}

	// Set default options if not provided
	if options == nil {
		options = &ScanOptions{
			MultiPage:  true,
			Duplex:     false,
			Color:      true,
			Resolution: 300,
		}
	}

	// Generate unique base filename
	timestamp := time.Now().Unix()
	baseFilename := fmt.Sprintf("scan_%d", timestamp)
	filepath := fmt.Sprintf("%s/%s", sm.config.TempFilesDir, baseFilename)

	// Build scanimage command with options
	args := []string{"-d", device}

	// Set format
	args = append(args, "--format=jpeg")

	// Set resolution
	args = append(args, "--resolution", fmt.Sprintf("%d", options.Resolution))

	// Set color mode
	if options.Color {
		args = append(args, "--mode", "Color")
	} else {
		args = append(args, "--mode", "Gray")
	}

	// Set multi-page options first
	if options.MultiPage {

		args = append(args, "--batch-start=1", "--batch-increment=1")
		// Use batch mode for multi-page scanning - use proper batch pattern
		batchPattern := sm.config.TempFilesDir + "/" + baseFilename + "_%d.jpg"
		sm.logger.Debugf("Batch pattern: %s", batchPattern)
		args = append(args, "--batch="+batchPattern)
	} else {
		// Single page scan
		args = append(args, "-o", fmt.Sprintf("%s.jpg", filepath))
	}

	// Set duplex if supported (after batch options)
	if options.Duplex {
		args = append(args, "--source", "ADF Duplex")
	} else {
		args = append(args, "--source", "ADF Front")
	}

	// Use scanimage to scan document

	sm.logger.Infof("Scan command: scanimage %v", args)
	cmd := exec.Command("scanimage", args...)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(sm.config.ScannerTimeout)*time.Millisecond)
	defer cancel()

	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)

	// Capture both stdout and stderr for better error reporting
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	sm.logger.Infof("Starting scan with options: multi_page=%v, duplex=%v, color=%v, resolution=%d",
		options.MultiPage, options.Duplex, options.Color, options.Resolution)
	sm.logger.Debugf("Scan command: scanimage %v", args)

	if err := cmd.Run(); err != nil {
		errorMsg := stderr.String()
		if errorMsg == "" {
			errorMsg = err.Error()
		}
		sm.logger.Errorf("Scan failed: %s \n %s", errorMsg, cmd.String())
		return nil, fmt.Errorf("scan failed: %s \n %s", errorMsg, cmd.String())
	}

	// Wait a moment to ensure files are fully written and flushed to disk
	time.Sleep(2 * time.Second)

	// Collect generated filenames
	var filenames []string
	if options.MultiPage {
		// Look for batch files
		pageNum := 1
		maxPages := 50 // Safety limit to prevent infinite loop
		sm.logger.Debugf("Looking for batch files with base: %s", baseFilename)
		for pageNum <= maxPages {
			filename := fmt.Sprintf("%s_%d.jpg", baseFilename, pageNum)
			fullPath := fmt.Sprintf("%s/%s", sm.config.TempFilesDir, filename)

			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				sm.logger.Debugf("File not found: %s", fullPath)
				break
			}
			filenames = append(filenames, filename)
			sm.logger.Debugf("Found page %d: %s", pageNum, filename)
			pageNum++
		}

		// For duplex scanning, we might need to look for additional patterns
		if options.Duplex && len(filenames) == 0 {
			// Try alternative naming patterns for duplex
			pageNum = 1
			for pageNum <= maxPages {
				// Try different naming patterns that some scanners use for duplex
				patterns := []string{
					fmt.Sprintf("%s_%d.jpg", baseFilename, pageNum),
					fmt.Sprintf("%s_front_%d.jpg", baseFilename, pageNum),
					fmt.Sprintf("%s_back_%d.jpg", baseFilename, pageNum),
					fmt.Sprintf("%s_%d_front.jpg", baseFilename, pageNum),
					fmt.Sprintf("%s_%d_back.jpg", baseFilename, pageNum),
				}

				found := false
				for _, pattern := range patterns {
					fullPath := fmt.Sprintf("%s/%s", sm.config.TempFilesDir, pattern)
					if _, err := os.Stat(fullPath); err == nil {
						filenames = append(filenames, pattern)
						sm.logger.Debugf("Found duplex page %d: %s", pageNum, pattern)
						found = true
					}
				}

				if !found {
					break
				}
				pageNum++
			}
		}

		// If still no files found, list all files in temp directory for debugging
		if len(filenames) == 0 {
			entries, err := os.ReadDir(sm.config.TempFilesDir)
			if err == nil {
				sm.logger.Debugf("No scan files found. Files in temp directory:")
				for _, entry := range entries {
					if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jpg") {
						sm.logger.Debugf("  - %s", entry.Name())
					}
				}
			}
		}
	} else {
		// Single page scan
		filename := fmt.Sprintf("%s.jpg", baseFilename)
		fullPath := fmt.Sprintf("%s/%s", sm.config.TempFilesDir, filename)

		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("scan completed but file was not created")
		}
		filenames = append(filenames, filename)
	}

	if len(filenames) == 0 {
		return nil, fmt.Errorf("scan completed but no files were created")
	}

	sm.logger.Infof("Document scanned successfully: %d pages", len(filenames))
	return filenames, nil
}

func (sm *ScannerManager) GetScannerCapabilities(device string) (map[string]interface{}, error) {
	sm.mu.RLock()
	scanner, exists := sm.scanners[device]
	sm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("scanner device '%s' not found", device)
	}
	if !scanner.Connected {
		return nil, fmt.Errorf("scanner '%s' is not connected", scanner.Name)
	}

	capabilities := make(map[string]interface{})

	// Get scanner options using scanimage -h
	cmd := exec.Command("scanimage", "-d", device, "-h")
	output, err := cmd.Output()
	if err != nil {
		sm.logger.Warnf("Failed to get scanner capabilities: %v", err)
		return capabilities, nil
	}

	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")

	// Parse capabilities
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for resolution options
		if strings.Contains(line, "resolution") {
			capabilities["resolution"] = true
		}

		// Check for color mode options
		if strings.Contains(line, "mode") {
			capabilities["color"] = true
		}

		// Check for source options (ADF, duplex)
		if strings.Contains(line, "source") {
			capabilities["source"] = true
		}

		// Check for batch options
		if strings.Contains(line, "batch") {
			capabilities["multi_page"] = true
		}
	}

	// Set default capabilities if not detected
	if capabilities["multi_page"] == nil {
		capabilities["multi_page"] = true // Most modern scanners support this
	}
	if capabilities["color"] == nil {
		capabilities["color"] = true // Most modern scanners support this
	}
	if capabilities["resolution"] == nil {
		capabilities["resolution"] = true // Most modern scanners support this
	}

	return capabilities, nil
}

func extractScannerName(device string) string {
	// Extract scanner name from device path
	parts := strings.Split(device, " ")
	if len(parts) > 0 {
		return parts[0]
	}
	return device
}
