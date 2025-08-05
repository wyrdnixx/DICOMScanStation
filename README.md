# DICOMScanStation

A Go-based web service for USB document scanner management with a modern web interface.

## Features

- **USB Scanner Detection**: Automatically detects and monitors connected USB document scanners
- **Multi-page Scanning**: Support for scanning multiple pages from document feeders
- **Duplex Scanning**: Scan both sides of documents (front and back)
- **Color Scanning**: Full color and grayscale scanning options
- **Configurable Resolution**: Adjustable DPI settings (150, 300, 600 DPI)
- **Web Interface**: Modern, responsive web UI built with Bootstrap and Font Awesome
- **Document Scanning**: Start scans directly from the web interface with custom options
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
### Install dcmtk (Dicom Toolkit)
```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install dcmtk

```

### Install DICOMScanStation and setup Systemd Service (Ubuntu 24.04)

```
dcmst@DICOMScanStation1:~$ cd /opt/
dcmst@DICOMScanStation1:/opt$ sudo git clone https://github.com/wyrdnixx/DICOMScanStation

dcmst@DICOMScanStation1:/opt$ sudo nano /etc/systemd/system/dicomscanstation.service
// check executable ! DICOMScanStation for x86 / DICOMScanStation_ARM for ARM CPU like RaspberryPi

---

[Unit]
Description=DICOMScanStation Web Service
After=network.target

[Service]
ExecStart=/opt/DICOMScanStation/DICOMScanStation
Restart=on-failure
User=www-data
Group=www-data
WorkingDirectory=/opt/DICOMScanStation
Environment=ENV=production
# Add other Environment=... lines if needed

[Install]
WantedBy=multi-user.target

---

### Add user www-data to Sane Scanner group to allow access to scanner devices:

```

dcmst@DICOMScanStation1:/opt/DICOMScanStation$ sudo usermod -aG scanner www-data

```


sudo systemctl daemon-reexec      # Recommended on Ubuntu 24.04+
sudo systemctl daemon-reload
sudo systemctl enable dicomscanstation.service
sudo systemctl start dicomscanstation.service
sudo systemctl status dicomscanstation.service


```

### Access the webinterface

http://<IP>:8081



---



### Install Go -- only if you want to compile yourself
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

   // compile for ARM CPUs
   GOOS=linux GOARCH=arm GOARM=7 go build -o DICOMScanStation_ARM

   ```

## Configuration

The application uses environment variables for configuration. Copy `env.example` to `.env` and modify as needed:

```bash
 DICOMScanStation Configuration
APP_NAME=DICOMScanStation
APP_VERSION=1.0.0
APP_PORT=8081
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

# DICOM Configuration for dcmtk findscu
DICOM_LOCAL_AETITLE=DICOMScanStation
DICOM_QUERY_AETITLE=DICOM_QR_SCP
DICOM_STORE_AETITLE=DICOM_STORAGE
DICOM_REMOTE_HOST=PACS-Server.fqdn
DICOM_FINDSCU_PORT=7840
DICOM_STORESCU_PORT=7810
DCMTK_PATH=/usr/bin

# DICOM Station Configuration
DICOM_STATION_NAME=DICOMScanStation 
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
- **Scan Options**: Configure multi-page, duplex, color, and resolution settings
- **File Management**: View thumbnails, download, and delete scanned files
- **Real-time Updates**: Automatic refresh of scanner status and file list
- **Multi-page Support**: Automatically handles multiple pages from document feeders

### API Endpoints

- `GET /api/scanners` - Get list of all scanners
- `GET /api/scanners/:device/capabilities` - Get scanner capabilities
- `GET /api/files` - Get list of scanned files
- `POST /api/scan` - Start a document scan with options
- `GET /api/files/:filename` - Download a specific file
- `DELETE /api/files/:filename` - Delete a specific file

### Scan Options

The scan API supports the following options:

```json
{
  "device": "scanner_device_name",
  "options": {
    "multi_page": true,
    "duplex": false,
    "color": true,
    "resolution": 300
  }
}
```

- **multi_page**: Enable multi-page scanning from document feeder
- **duplex**: Scan both sides of documents (requires duplex-capable scanner)
- **color**: Enable color scanning (false for grayscale)
- **resolution**: DPI setting (150, 300, or 600)

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



---

➜  DICOMScanStation git:(main) ✗ scanimage -d 'fujitsu:fi-7030:211822' -h
Usage: Cursor-1.3.8-x86_64.AppImage [OPTION]...

Start image acquisition on a scanner device and write image data to
standard output.

Parameters are separated by a blank from single-character options (e.g.
-d epson) and by a "=" from multi-character options (e.g. --device-name=epson).
-d, --device-name=DEVICE   use a given scanner device (e.g. hp:/dev/scanner)
    --format=pnm|tiff|png|jpeg|pdf  file format of output file
-i, --icc-profile=PROFILE  include this ICC profile into TIFF file
-L, --list-devices         show available scanner devices
-f, --formatted-device-list=FORMAT similar to -L, but the FORMAT of the output
                           can be specified: %d (device name), %v (vendor),
                           %m (model), %t (type), %i (index number), and
                           %n (newline)
-b, --batch[=FORMAT]       working in batch mode, FORMAT is `out%d.pnm' `out%d.tif'
                           `out%d.png' or `out%d.jpg' by default depending on --format
                           This option is incompatible with --output-file.
    --batch-start=#        page number to start naming files with
    --batch-count=#        how many pages to scan in batch mode
    --batch-increment=#    increase page number in filename by #
    --batch-double         increment page number by two, same as
                           --batch-increment=2
    --batch-print          print image filenames to stdout
    --batch-prompt         ask for pressing a key before scanning a page
    --accept-md5-only      only accept authorization requests using md5
-p, --progress             print progress messages
-o, --output-file=PATH     save output to the given file instead of stdout.
                           This option is incompatible with --batch.
-n, --dont-scan            only set options, don't actually scan
-T, --test                 test backend thoroughly
-A, --all-options          list all available backend options
-h, --help                 display this help message and exit
-v, --verbose              give even more status messages
-B, --buffer-size=#        change input buffer size (in kB, default 32)
-V, --version              print version information
Output format is not set, using pnm as a default.

Options specific to device `fujitsu:fi-7030:211822':
  Standard:
    --source ADF Front|ADF Back|ADF Duplex [ADF Front]
        Selects the scan source (such as a document-feeder).
    --mode Lineart|Halftone|Gray|Color [Lineart]
        Selects the scan mode (e.g., lineart, monochrome, or color).
    --resolution 50..600dpi (in steps of 1) [600]
        Sets the resolution of the scanned image.
  Geometry:
    --page-width 0..221.121mm (in steps of 0.0211639) [215.872]
        Specifies the width of the media.  Required for automatic centering of
        sheet-fed scans.
    --page-height 0..2712.88mm (in steps of 0.0211639) [279.364]
        Specifies the height of the media.
    -l 0..215.872mm (in steps of 0.0211639) [0]
        Top-left x position of scan area.
    -t 0..279.364mm (in steps of 0.0211639) [0]
        Top-left y position of scan area.
    -x 0..215.872mm (in steps of 0.0211639) [215.872]
        Width of scan-area.
    -y 0..279.364mm (in steps of 0.0211639) [279.364]
        Height of scan-area.
  Enhancement:
    --brightness -127..127 (in steps of 1) [0]
        Controls the brightness of the acquired image.
    --contrast -127..127 (in steps of 1) [0]
        Controls the contrast of the acquired image.
    --threshold 0..255 (in steps of 1) [0]
        Select minimum-brightness to get a white point
    --rif[=(yes|no)] [no]
        Reverse image format
    --ht-type Default|Dither|Diffusion [inactive]
        Control type of halftone filter
    --ht-pattern 0..3 (in steps of 1) [inactive]
        Control pattern of halftone filter
    --emphasis -128..127 (in steps of 1) [0]
        Negative to smooth or positive to sharpen image
    --variance 0..255 (in steps of 1) [0]
        Set SDTC variance rate (sensitivity), 0 equals 127
  Advanced:
    --ald[=(yes|no)] [no] [advanced]
        Scanner detects paper lower edge. May confuse some frontends.
    --df-action Default|Continue|Stop [Default] [advanced]
        Action following double feed error
    --df-skew[=(yes|no)] [inactive]
        Enable double feed error due to skew
    --df-thickness[=(yes|no)] [inactive]
        Enable double feed error due to paper thickness
    --df-length[=(yes|no)] [inactive]
        Enable double feed error due to paper length
    --df-diff Default|10mm|15mm|20mm [inactive]
        Difference in page length to trigger double feed error
    --df-recovery Default|Off|On [On] [advanced]
        Request scanner to reverse feed on paper jam
    --bgcolor Default|White|Black [Default] [advanced]
        Set color of background for scans. May conflict with overscan option
    --dropoutcolor Default|Red|Green|Blue [Default] [advanced]
        One-pass scanners use only one color during gray or binary scanning,
        useful for colored paper or ink
    --buffermode Default|Off|On [Off] [advanced]
        Request scanner to read pages quickly from ADF into internal memory
    --prepick Default|Off|On [Default] [advanced]
        Request scanner to grab next page from ADF
    --overscan Default|Off|On [Default] [advanced]
        Collect a few mm of background on top side of scan, before paper
        enters ADF, and increase maximum scan area beyond paper size, to allow
        collection on remaining sides. May conflict with bgcolor option
    --sleeptimer 0..60 (in steps of 1) [0] [advanced]
        Time in minutes until the internal power supply switches to sleep mode
    --offtimer 0..960 (in steps of 1) [240] [advanced]
        Time in minutes until the internal power supply switches the scanner
        off. Will be rounded to nearest 15 minutes. Zero means never power off.
    --lowmemory[=(yes|no)] [no] [advanced]
        Limit driver memory usage for use in embedded systems. Causes some
        duplex transfers to alternate sides on each call to sane_read. Value of
        option 'side' can be used to determine correct image. This option
        should only be used with custom front-end software.
    --hwdeskewcrop[=(yes|no)] [no] [advanced]
        Request scanner to rotate and crop pages digitally.
    --swdeskew[=(yes|no)] [no] [advanced]
        Request driver to rotate skewed pages digitally.
    --swdespeck 0..9 (in steps of 1) [0]
        Maximum diameter of lone dots to remove from scan.
    --swcrop[=(yes|no)] [no] [advanced]
        Request driver to remove border from pages digitally.
    --swskip 0..100% (in steps of 0.100006) [0]
        Request driver to discard pages with low percentage of dark pixels
  Sensors:
