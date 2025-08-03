package dicom

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"DICOMScanStation/config"

	"github.com/sirupsen/logrus"
)

type PatientInfo struct {
	PatientID string `json:"patientId"`
	Name      string `json:"name"`
	BirthDate string `json:"birthDate"`
	Gender    string `json:"gender"`
	StudyDate string `json:"studyDate"`
}

type DicomService struct {
	config *config.Config
	logger *logrus.Logger
}

func NewDicomService(cfg *config.Config) *DicomService {
	return &DicomService{
		config: cfg,
		logger: logrus.New(),
	}
}

func (ds *DicomService) SearchPatients(searchTerm string, searchType string) ([]PatientInfo, error) {
	ds.logger.Infof("DICOM service: Searching for patients with term: %s (type: %s)", searchTerm, searchType)

	var searchPatterns []string

	if searchType == "birthdate" {
		// For birthdate search, use exact match
		searchPatterns = []string{searchTerm}
	} else {
		// For name search, try multiple patterns
		searchPatterns = []string{
			fmt.Sprintf("%s*", searchTerm),  // Prefix match
			fmt.Sprintf("*%s*", searchTerm), // Substring match
			fmt.Sprintf("*%s", searchTerm),  // Suffix match
		}
	}

	ds.logger.Debugf("DICOM service: Trying search patterns: %v for term: %s", searchPatterns, searchTerm)

	// Try each search pattern and collect all unique results
	var allPatients []PatientInfo
	seenPatients := make(map[string]bool) // Track unique patients by ID

	for _, pattern := range searchPatterns {
		ds.logger.Debugf("DICOM service: Trying pattern: %s", pattern)

		// Build the findscu command based on search type
		var cmd *exec.Cmd
		if searchType == "birthdate" {
			cmd = exec.Command(
				ds.config.DcmtkPath+"/findscu",
				"-v",           // Verbose output
				"-S",           // Enable searching
				"-aet", "AET1", // Calling AE Title (use AET1 as it works)
				"-aec", ds.config.DicomRemoteAETitle, // Called AE Title
				"-k", "QueryRetrieveLevel=PATIENT", // Query level
				"-k", "PatientName", // Request Patient Name
				"-k", "PatientID", // Request Patient ID
				"-k", fmt.Sprintf("PatientBirthDate=%s", pattern), // Patient birthdate search
				"-k", "PatientSex", // Request Patient Sex
				ds.config.DicomRemoteHost,                    // Remote host (at the end)
				fmt.Sprintf("%d", ds.config.DicomRemotePort), // Remote port (at the end)
			)
		} else {
			// Name search
			cmd = exec.Command(
				ds.config.DcmtkPath+"/findscu",
				"-v",           // Verbose output
				"-S",           // Enable searching
				"-aet", "AET1", // Calling AE Title (use AET1 as it works)
				"-aec", ds.config.DicomRemoteAETitle, // Called AE Title
				"-k", "QueryRetrieveLevel=PATIENT", // Query level
				"-k", fmt.Sprintf("PatientName=%s", pattern), // Patient name search with pattern
				"-k", "PatientID", // Request Patient ID
				"-k", "PatientBirthDate", // Request Patient Birth Date
				"-k", "PatientSex", // Request Patient Sex
				ds.config.DicomRemoteHost,                    // Remote host (at the end)
				fmt.Sprintf("%d", ds.config.DicomRemotePort), // Remote port (at the end)
			)
		}

		ds.logger.Debugf("DICOM service: Executing command: %s", strings.Join(cmd.Args, " "))

		// Set timeout for the command
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)

		ds.logger.Debugf("DICOM service: Final command args: %v", cmd.Args)

		// Capture both stdout and stderr
		output, err := cmd.CombinedOutput()

		if err != nil {
			ds.logger.Debugf("DICOM service: Pattern %s failed: %v", pattern, err)
			ds.logger.Debugf("DICOM service: Command output: %s", string(output))
			continue // Try next pattern
		}

		ds.logger.Debugf("DICOM service: Pattern %s output: %s", pattern, string(output))

		// Parse the output to extract patient information
		patients, err := ds.parseFindscuOutput(string(output))
		if err != nil {
			ds.logger.Debugf("DICOM service: Failed to parse output for pattern %s: %v", pattern, err)
			continue // Try next pattern
		}

		// Add unique patients to the result
		for _, patient := range patients {
			if patient.PatientID != "" && !seenPatients[patient.PatientID] {
				allPatients = append(allPatients, patient)
				seenPatients[patient.PatientID] = true
			}
		}
	}

	ds.logger.Infof("DICOM service: Found %d unique patients", len(allPatients))
	return allPatients, nil
}

func (ds *DicomService) parseFindscuOutput(output string) ([]PatientInfo, error) {
	var patients []PatientInfo

	lines := strings.Split(output, "\n")
	var currentPatient *PatientInfo
	inResponse := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for "Find Response:" to start parsing a new patient
		if strings.Contains(line, "Find Response:") {
			if currentPatient != nil && currentPatient.Name != "" {
				patients = append(patients, *currentPatient)
			}
			currentPatient = &PatientInfo{}
			inResponse = true
			continue
		}

		// Skip lines that are not part of a response
		if !inResponse {
			continue
		}

		// Look for patient information in the output
		if strings.Contains(line, "PatientName") {
			// Extract patient name from line like: (0010,0010) PN [Rubo DEMO ]
			if idx := strings.Index(line, "["); idx != -1 {
				if endIdx := strings.Index(line, "]"); endIdx != -1 {
					name := strings.TrimSpace(line[idx+1 : endIdx])
					if name != "*" && name != "" { // Skip wildcard and empty names
						currentPatient.Name = name
					}
				}
			}
		} else if strings.Contains(line, "PatientID") && currentPatient != nil {
			// Extract patient ID
			if idx := strings.Index(line, "["); idx != -1 {
				if endIdx := strings.Index(line, "]"); endIdx != -1 {
					currentPatient.PatientID = strings.TrimSpace(line[idx+1 : endIdx])
				}
			}
		} else if strings.Contains(line, "PatientBirthDate") && currentPatient != nil {
			// Extract birth date
			if idx := strings.Index(line, "["); idx != -1 {
				if endIdx := strings.Index(line, "]"); endIdx != -1 {
					currentPatient.BirthDate = strings.TrimSpace(line[idx+1 : endIdx])
				}
			}
		} else if strings.Contains(line, "PatientSex") && currentPatient != nil {
			// Extract gender
			if idx := strings.Index(line, "["); idx != -1 {
				if endIdx := strings.Index(line, "]"); endIdx != -1 {
					currentPatient.Gender = strings.TrimSpace(line[idx+1 : endIdx])
				}
			}
		} else if strings.Contains(line, "StudyDate") && currentPatient != nil {
			// Extract study date
			if idx := strings.Index(line, "["); idx != -1 {
				if endIdx := strings.Index(line, "]"); endIdx != -1 {
					currentPatient.StudyDate = strings.TrimSpace(line[idx+1 : endIdx])
				}
			}
		}

		// Check for end of response (empty line or new section)
		if line == "" && inResponse {
			inResponse = false
		}
	}

	// Add the last patient if exists
	if currentPatient != nil && currentPatient.Name != "" {
		patients = append(patients, *currentPatient)
	}

	// If no patients found in output, return empty list
	if len(patients) == 0 {
		ds.logger.Warn("DICOM service: No patients found in findscu output")
		return []PatientInfo{}, nil
	}

	ds.logger.Debugf("DICOM service: Parsed %d patients from output", len(patients))
	return patients, nil
}

func (ds *DicomService) SendToPacs(patientIDs []string, documentCreator string, filePaths []string) error {
	ds.logger.Infof("DICOM service: Sending %d files to %d patients", len(filePaths), len(patientIDs))

	// For now, we'll just log the operation
	// In a real implementation, you would use storescu to send files to PACS
	ds.logger.Infof("DICOM service: Would send files to patients: %v", patientIDs)
	ds.logger.Infof("DICOM service: Document creator: %s", documentCreator)
	ds.logger.Infof("DICOM service: Files to send: %v", filePaths)

	return nil
}
