package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppName             string
	AppVersion          string
	AppPort             string
	AppHost             string
	TempFilesDir        string
	MaxFileSize         int64
	AllowedExtensions   []string
	ScannerPollInterval int
	ScannerTimeout      int
	WebTitle            string
	WebDescription      string
	LogLevel            string
	LogFormat           string
	// DICOM Configuration for dcmtk findscu
	DicomAETitle       string
	DicomRemoteHost    string
	DicomFindscuPort   int
	DicomStorescuPort  int
	DicomRemoteAETitle string
	DcmtkPath          string
	// DICOM Station Configuration
	DicomStationName string
}

func LoadConfig() *Config {
	return &Config{
		AppName:             getEnv("APP_NAME", "DICOMScanStation"),
		AppVersion:          getEnv("APP_VERSION", "1.0.0"),
		AppPort:             getEnv("APP_PORT", "8081"),
		AppHost:             getEnv("APP_HOST", "0.0.0.0"),
		TempFilesDir:        getEnv("TEMP_FILES_DIR", "/tmp/DICOMScanStation/tempfiles"),
		MaxFileSize:         getEnvAsInt64("MAX_FILE_SIZE", 10485760),
		AllowedExtensions:   getEnvAsSlice("ALLOWED_EXTENSIONS", []string{"jpg", "jpeg", "png", "tiff", "tif"}),
		ScannerPollInterval: getEnvAsInt("SCANNER_POLL_INTERVAL", 5000),
		ScannerTimeout:      getEnvAsInt("SCANNER_TIMEOUT", 30000),
		WebTitle:            getEnv("WEB_TITLE", "DICOM Scan Station"),
		WebDescription:      getEnv("WEB_DESCRIPTION", "USB Document Scanner Web Interface"),
		LogLevel:            getEnv("LOG_LEVEL", "info"),
		LogFormat:           getEnv("LOG_FORMAT", "json"),
		// DICOM Configuration for dcmtk findscu
		DicomAETitle:       getEnv("DICOM_AETITLE", "DICOMScanStation"),
		DicomRemoteHost:    getEnv("DICOM_REMOTE_HOST", "localhost"),
		DicomFindscuPort:   getEnvAsInt("DICOM_FINDSCU_PORT", 11112),
		DicomStorescuPort:  getEnvAsInt("DICOM_STORESCU_PORT", 11113),
		DicomRemoteAETitle: getEnv("DICOM_REMOTE_AETITLE", "ANY-SCP"),
		DcmtkPath:          getEnv("DCMTK_PATH", "/usr/bin"),
		// DICOM Station Configuration
		DicomStationName: getEnv("DICOM_STATION_NAME", "DICOMScanStation"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}
