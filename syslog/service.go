package syslog

import (
	"fmt"
	"log/syslog"
	"strings"

	"DICOMScanStation/config"

	"github.com/sirupsen/logrus"
)

type SyslogService struct {
	config  *config.Config
	writer  *syslog.Writer
	logger  *logrus.Logger
	enabled bool
}

func NewSyslogService(cfg *config.Config) *SyslogService {
	logger := logrus.New()

	// Check if syslog is enabled
	server := strings.TrimSpace(cfg.SyslogServer)
	enabled := server != "" && strings.ToLower(server) != "false"

	service := &SyslogService{
		config:  cfg,
		logger:  logger,
		enabled: enabled,
	}

	if enabled {
		// Try to connect to syslog server
		writer, err := syslog.Dial("udp", server, syslog.LOG_INFO|syslog.LOG_USER, "DICOMScanStation")
		if err != nil {
			logger.Errorf("Failed to connect to syslog server %s: %v", server, err)
			service.enabled = false
		} else {
			service.writer = writer
			logger.Infof("Connected to syslog server: %s", server)
		}
	} else {
		logger.Info("Syslog service disabled")
	}

	return service
}

// IsEnabled returns whether syslog is enabled
func (s *SyslogService) IsEnabled() bool {
	return s.enabled
}

// Close closes the syslog connection
func (s *SyslogService) Close() {
	if s.writer != nil {
		s.writer.Close()
	}
}

// ServiceStarted sends a message when the service starts
func (s *SyslogService) ServiceStarted(version string, host string, port string) {
	if !s.enabled {
		return
	}

	msg := fmt.Sprintf("DICOMScanStation v%s started on %s:%s", version, host, port)
	if err := s.writer.Info(msg); err != nil {
		s.logger.Errorf("Failed to send syslog message: %v", err)
	}
}

// ServiceStopped sends a message when the service stops normally
func (s *SyslogService) ServiceStopped() {
	if !s.enabled {
		return
	}

	msg := "DICOMScanStation stopped normally"
	if err := s.writer.Info(msg); err != nil {
		s.logger.Errorf("Failed to send syslog message: %v", err)
	}
}

// ClientConnected sends a message when a new client connects
func (s *SyslogService) ClientConnected(clientIP string, endpoint string) {
	if !s.enabled {
		return
	}

	msg := fmt.Sprintf("[Client: %s] New client connection to %s", clientIP, endpoint)
	if err := s.writer.Info(msg); err != nil {
		s.logger.Errorf("Failed to send syslog message: %v", err)
	}
}

// ScanCompleted sends a success message after a scan completes
func (s *SyslogService) ScanCompleted(clientIP string, deviceName string, pages int) {
	if !s.enabled {
		return
	}

	msg := fmt.Sprintf("[Client: %s] Scan completed successfully: device=%s, pages=%d", clientIP, deviceName, pages)
	if err := s.writer.Info(msg); err != nil {
		s.logger.Errorf("Failed to send syslog message: %v", err)
	}
}

// DicomSendCompleted sends a success message after DICOM files are sent
func (s *SyslogService) DicomSendCompleted(clientIP string, patientName string, patientID string, filesCount int) {
	if !s.enabled {
		return
	}

	msg := fmt.Sprintf("[Client: %s] DICOM files sent successfully: patient=%s, patientID=%s, files=%d",
		clientIP, patientName, patientID, filesCount)
	if err := s.writer.Info(msg); err != nil {
		s.logger.Errorf("Failed to send syslog message: %v", err)
	}
}

// Error sends an error message with function name, client IP, and error details
func (s *SyslogService) Error(clientIP string, functionName string, errorText string) {
	if !s.enabled {
		return
	}

	msg := fmt.Sprintf("ERROR [Client: %s] in %s: %s", clientIP, functionName, errorText)
	if err := s.writer.Err(msg); err != nil {
		s.logger.Errorf("Failed to send syslog error message: %v", err)
	}
}

// ErrorWithContext sends an error message with client IP and additional context
func (s *SyslogService) ErrorWithContext(clientIP string, functionName string, context string, errorText string) {
	if !s.enabled {
		return
	}

	msg := fmt.Sprintf("ERROR [Client: %s] in %s [%s]: %s", clientIP, functionName, context, errorText)
	if err := s.writer.Err(msg); err != nil {
		s.logger.Errorf("Failed to send syslog error message: %v", err)
	}
}
