# DICOMScanStation

A Go-based web service for USB document scanner management with a modern web interface.

## Features

- **USB Scanner Detection**: Automatically detects and monitors connected USB document scanners
- **Web Interface**: Modern, responsive web UI built with Bootstrap and Font Awesome
- **Document Scanning**: Start scans directly from the web interface
- **File Management**: View, download, and delete scanned documents
- **Real-time Updates**: Live scanner status and file list updates
- **Thumbnail Preview**: Click thumbnails to view full-size images
- **Configuration**: Environment-based configuration system

## Prerequisites

### System Requirements
- Linux system with USB support
- SANE (Scanner Access Now Easy) libraries
- Go 1.21 or later

### Install SANE
```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install sane sane-utils libsane-dev

# CentOS/RHEL/Fedora
sudo yum install sane-backends sane-frontends
# or
sudo dnf install sane-backends sane-frontends

# Arch Linux
sudo pacman -S sane
```

### Install Go
```bash
# Download and install Go from https://golang.org/dl/
# or use your distribution's package manager
```

## Installation

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd DICOMScanStation
   ```

2. **Install dependencies**
   ```bash
   go mod tidy
   ```

3. **Create environment file**
   ```bash
   cp env.example .env
   # Edit .env file with your preferred settings
   ```

4. **Build the application**
   ```bash
   go build -o DICOMScanStation
   ```

## Configuration

The application uses environment variables for configuration. Copy `env.example` to `.env` and modify as needed:

```bash
# Application Settings
APP_NAME=DICOMScanStation
APP_VERSION=1.0.0
APP_PORT=8080
APP_HOST=0.0.0.0

# File Storage
TEMP_FILES_DIR=/tmp/DICOMScanStation/tempfiles
MAX_FILE_SIZE=10485760
ALLOWED_EXTENSIONS=jpg,jpeg,png,tiff,tif

# Scanner Settings
SCANNER_POLL_INTERVAL=5000
SCANNER_TIMEOUT=30000

# Web Interface
WEB_TITLE=DICOM Scan Station
WEB_DESCRIPTION=USB Document Scanner Web Interface

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
```

## Usage

### Running the Application

1. **Start the service**
   ```bash
   ./DICOMScanStation
   ```

2. **Access the web interface**
   Open your browser and navigate to `http://localhost:8080`

### Web Interface Features

- **Scanner Status**: View all detected scanners and their connection status
- **Scan Control**: Start document scans when a scanner is connected
- **File Management**: View thumbnails, download, and delete scanned files
- **Real-time Updates**: Automatic refresh of scanner status and file list

### API Endpoints

- `GET /api/scanners` - Get list of all scanners
- `GET /api/files` - Get list of scanned files
- `POST /api/scan` - Start a document scan
- `GET /api/files/:filename` - Download a specific file
- `DELETE /api/files/:filename` - Delete a specific file

## Scanner Support

The application uses SANE (Scanner Access Now Easy) to detect and control USB scanners. Supported scanners include:

- Most USB document scanners
- Flatbed scanners
- Sheet-fed scanners
- Multi-function devices with scanning capability

### Testing Scanner Detection

To test if your scanner is detected by SANE:

```bash
# List all available scanners
scanimage -L

# Test scanning (replace 'device_name' with your scanner)
scanimage -d 'device_name' --test
```

## File Storage

Scanned documents are stored in the configured temporary directory (`/tmp/DICOMScanStation/tempfiles` by default). The application:

- Prevents new scans when files already exist
- Displays thumbnails for all scanned files
- Allows individual file deletion
- Provides full-size image viewing

## Development

### Project Structure
```
DICOMScanStation/
├── main.go                 # Application entry point
├── go.mod                  # Go module file
├── env.example            # Environment configuration example
├── README.md              # This file
├── config/
│   └── config.go          # Configuration management
├── scanner/
│   └── manager.go         # Scanner detection and management
└── web/
    ├── router.go          # HTTP router and API endpoints
    └── templates/
        └── index.html     # Web interface template
```

### Building for Development
```bash
# Run with hot reload (requires air)
go install github.com/cosmtrek/air@latest
air

# Or run directly
go run main.go
```

## Troubleshooting

### Scanner Not Detected
1. Ensure SANE is properly installed
2. Check USB permissions: `sudo usermod -a -G scanner $USER`
3. Test scanner detection: `scanimage -L`
4. Check system logs for USB device issues

### Permission Issues
```bash
# Add user to scanner group
sudo usermod -a -G scanner $USER

# Create udev rules for scanner access
sudo nano /etc/udev/rules.d/99-scanner.rules
# Add: SUBSYSTEM=="usb", ATTRS{idVendor}=="xxxx", ATTRS{idProduct}=="yyyy", MODE="0666"
```

### Web Interface Not Loading
1. Check if the service is running: `ps aux | grep DICOMScanStation`
2. Verify port is not in use: `netstat -tlnp | grep 8080`
3. Check firewall settings
4. Review application logs

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## Support

For issues and questions:
1. Check the troubleshooting section
2. Review system logs
3. Test scanner detection manually
4. Open an issue with detailed information 