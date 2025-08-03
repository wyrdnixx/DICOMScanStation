package dicom

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
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

			// Check for connection errors based on findscu output
			outputStr := string(output)
			if strings.Contains(outputStr, "Association Request Failed") {
				// Return the exact findscu error message
				ds.logger.Errorf("DICOM service: findscu error: %s", outputStr)
				return nil, fmt.Errorf("DICOM error: %s", strings.TrimSpace(outputStr))
			}

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

	// If no patients found and we tried all patterns, check if it was due to connection issues
	if len(allPatients) == 0 {
		ds.logger.Warn("DICOM service: No patients found after trying all patterns")
		// Try a simple connection test
		testCmd := exec.Command(
			ds.config.DcmtkPath+"/findscu",
			"-v",
			"-S",
			"-aet", "AET1",
			"-aec", ds.config.DicomRemoteAETitle,
			"-k", "QueryRetrieveLevel=PATIENT",
			"-k", "PatientName=*",
			ds.config.DicomRemoteHost,
			fmt.Sprintf("%d", ds.config.DicomRemotePort),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		testCmd = exec.CommandContext(ctx, testCmd.Path, testCmd.Args[1:]...)

		_, testErr := testCmd.CombinedOutput()
		if testErr != nil {
			ds.logger.Errorf("DICOM service: Connection test failed: %v", testErr)
			return nil, fmt.Errorf("unable to connect to DICOM server at %s:%d", ds.config.DicomRemoteHost, ds.config.DicomRemotePort)
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
			ds.logger.Debugf("DICOM service: Found PatientSex line: %s", line)
			if idx := strings.Index(line, "["); idx != -1 {
				if endIdx := strings.Index(line, "]"); endIdx != -1 {
					currentPatient.Gender = strings.TrimSpace(line[idx+1 : endIdx])
					ds.logger.Debugf("DICOM service: Extracted gender: '%s'", currentPatient.Gender)
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

type FileProgress struct {
	Filename string `json:"filename"`
	Status   string `json:"status"` // "converting", "updating", "sending", "completed", "failed"
	Message  string `json:"message"`
	Progress int    `json:"progress"` // 0-100
}

func (ds *DicomService) SendToPacs(patientIDs []string, documentCreator string, filePaths []string, selectedPatient PatientInfo) ([]FileProgress, error) {
	ds.logger.Infof("DICOM service: Starting PACs upload process")
	ds.logger.Infof("DICOM service: Selected patient: %+v", selectedPatient)
	ds.logger.Infof("DICOM service: Document creator: %s", documentCreator)
	ds.logger.Infof("DICOM service: Files to process: %v", filePaths)

	// Get all JPG files from temp directory
	jpgFiles, err := ds.getJpgFilesFromTempDir()
	if err != nil {
		ds.logger.Errorf("DICOM service: Failed to get JPG files: %v", err)
		return nil, fmt.Errorf("failed to get JPG files: %v", err)
	}

	ds.logger.Infof("DICOM service: Found %d JPG files to convert", len(jpgFiles))

	var progress []FileProgress

	// Process each JPG file
	for i, jpgFile := range jpgFiles {
		filename := filepath.Base(jpgFile)

		// Initialize progress for this file
		fileProgress := FileProgress{
			Filename: filename,
			Status:   "converting",
			Message:  "Converting JPG to DICOM format...",
			Progress: 0,
		}
		progress = append(progress, fileProgress)

		ds.logger.Infof("DICOM service: Processing file: %s", jpgFile)

		// Step 1: Convert JPG to DICOM using img2dcm
		fileProgress.Status = "converting"
		fileProgress.Message = "Converting JPG to DICOM format..."
		fileProgress.Progress = 20
		progress[i] = fileProgress

		dcmFile, err := ds.convertJpgToDicom(jpgFile)
		if err != nil {
			ds.logger.Errorf("DICOM service: Failed to convert %s to DICOM: %v", jpgFile, err)
			fileProgress.Status = "failed"
			fileProgress.Message = fmt.Sprintf("Conversion failed: %v", err)
			fileProgress.Progress = 0
			progress[i] = fileProgress
			continue
		}

		// Step 2: Update DICOM file with patient data
		fileProgress.Status = "updating"
		fileProgress.Message = "Updating DICOM with patient data..."
		fileProgress.Progress = 50
		progress[i] = fileProgress

		err = ds.updateDicomWithPatientData(dcmFile, selectedPatient, documentCreator)
		if err != nil {
			ds.logger.Errorf("DICOM service: Failed to update DICOM file %s: %v", dcmFile, err)
			fileProgress.Status = "failed"
			fileProgress.Message = fmt.Sprintf("Update failed: %v", err)
			fileProgress.Progress = 0
			progress[i] = fileProgress
			continue
		}

		// Step 3: Send DICOM file to PACs server
		fileProgress.Status = "sending"
		fileProgress.Message = "Sending to PACs server..."
		fileProgress.Progress = 80
		progress[i] = fileProgress

		err = ds.sendDicomToPacs(dcmFile)
		if err != nil {
			ds.logger.Errorf("DICOM service: Failed to send %s to PACs: %v", dcmFile, err)
			fileProgress.Status = "failed"
			fileProgress.Message = fmt.Sprintf("Upload failed: %v", err)
			fileProgress.Progress = 0
			progress[i] = fileProgress
			continue
		}

		// Step 4: Completed successfully
		fileProgress.Status = "completed"
		fileProgress.Message = "Successfully uploaded to PACs"
		fileProgress.Progress = 100
		progress[i] = fileProgress

		ds.logger.Infof("DICOM service: Successfully processed and sent %s", jpgFile)
	}

	ds.logger.Infof("DICOM service: PACs upload process completed")
	return progress, nil
}

func (ds *DicomService) getJpgFilesFromTempDir() ([]string, error) {
	ds.logger.Debugf("DICOM service: Scanning for JPG files in: %s", ds.config.TempFilesDir)

	// Use find command to get all JPG files
	cmd := exec.Command("find", ds.config.TempFilesDir, "-name", "*.jpg", "-type", "f")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to find JPG files: %v", err)
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	var jpgFiles []string

	for _, file := range files {
		if strings.TrimSpace(file) != "" {
			jpgFiles = append(jpgFiles, strings.TrimSpace(file))
		}
	}

	ds.logger.Debugf("DICOM service: Found %d JPG files", len(jpgFiles))
	return jpgFiles, nil
}

func (ds *DicomService) convertJpgToDicom(jpgFile string) (string, error) {
	// Generate DICOM filename
	dcmFile := strings.Replace(jpgFile, ".jpg", ".dcm", 1)

	ds.logger.Debugf("DICOM service: Converting %s to %s", jpgFile, dcmFile)

	// Run img2dcm command
	cmd := exec.Command(
		ds.config.DcmtkPath+"/img2dcm",
		jpgFile,
		dcmFile,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("img2dcm failed: %v, output: %s", err, string(output))
	}

	ds.logger.Debugf("DICOM service: img2dcm output: %s", string(output))
	return dcmFile, nil
}

func (ds *DicomService) updateDicomWithPatientData(dcmFile string, patient PatientInfo, documentCreator string) error {
	ds.logger.Debugf("DICOM service: Updating DICOM file %s with patient data", dcmFile)

	// Build dcmodify command with patient data
	cmd := exec.Command(
		ds.config.DcmtkPath+"/dcmodify",
		"-nb",                                             // No backup
		"-gin",                                            // Group length implicit
		"-i", fmt.Sprintf("(0010,0010)=%s", patient.Name), // PatientName
		"-i", fmt.Sprintf("(0010,0020)=%s", patient.PatientID), // PatientID
		"-i", fmt.Sprintf("(0010,0030)=%s", patient.BirthDate), // PatientBirthDate
		"-i", fmt.Sprintf("(0010,0040)=%s", patient.Gender), // PatientSex
		"-i", fmt.Sprintf("(0008,0080)=%s", documentCreator), // InstitutionName
		"-i", fmt.Sprintf("(0008,1010)=%s", ds.config.DicomStationName), // StationName
		dcmFile,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dcmodify failed: %v, output: %s", err, string(output))
	}

	ds.logger.Debugf("DICOM service: dcmodify output: %s", string(output))
	return nil
}

func (ds *DicomService) sendDicomToPacs(dcmFile string) error {
	ds.logger.Debugf("DICOM service: Sending %s to PACs server", dcmFile)

	// Run dcmsend command
	cmd := exec.Command(
		ds.config.DcmtkPath+"/dcmsend",
		"-aet", ds.config.DicomAETitle,
		"-aec", ds.config.DicomRemoteAETitle,
		ds.config.DicomRemoteHost,
		fmt.Sprintf("%d", ds.config.DicomRemotePort),
		dcmFile,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dcmsend failed: %v, output: %s", err, string(output))
	}

	ds.logger.Debugf("DICOM service: dcmsend output: %s", string(output))
	return nil
}
