# 📦 GoNAS Installation Guide

Comprehensive installation guide for **GoNAS** (Go Network Attached Storage & Web File Manager) across multiple operating systems and architectures.

---

## ⚡ 1-Minute Quick Start

Choose your platform below for the fastest automated installation:

### Linux / Armbian / Raspberry Pi / macOS
```bash
curl -fsSL https://raw.githubusercontent.com/Arya-Penangsang/gonas/main/install.sh | sudo bash
```
> **What it does automatically:** Detects your OS (`linux`/`darwin`) and CPU architecture (`amd64`, `arm64`, `armv7`), downloads the latest release binary, creates `/var/nas/storage`, seeds initial credentials (`admin` / `admin123`), and sets up a background **systemd** service (`gonas.service`).

### Windows (PowerShell Administrator)
```powershell
irm https://raw.githubusercontent.com/Arya-Penangsang/gonas/main/install.ps1 | iex
```
> **What it does automatically:** Downloads `gonas.exe` into `C:\GoNAS`, creates `storage`, opens Windows Firewall port `8082`, enables Windows WebDAV BasicAuth registry key (`BasicAuthLevel = 2`), and registers an auto-start Windows Scheduled Task (`GoNAS_Server`).

---

## 📑 Table of Contents
1. [Linux & Armbian (Systemd Service)](#-linux--armbian-systemd-service)
2. [Raspberry Pi & Single Board Computers](#-raspberry-pi--single-board-computers)
3. [Windows Setup (Service & Scheduled Task)](#-windows-setup)
4. [Docker & Docker Compose](#-docker--docker-compose)
5. [macOS (Darwin)](#-macos-darwin)
6. [Building from Source](#-building-from-source)
7. [Post-Installation & Default Credentials](#-post-installation--default-credentials)
8. [CLI Arguments & Configuration Flags](#-cli-arguments--configuration-flags)

---

## 🐧 Linux & Armbian (Systemd Service)

### Method A: Automated Installer (Recommended)
```bash
curl -fsSL https://raw.githubusercontent.com/Arya-Penangsang/gonas/main/install.sh | sudo bash
```

### Method B: Manual Installation

1. **Download the binary** matching your CPU architecture from [Releases](https://github.com/Arya-Penangsang/gonas/releases):
   ```bash
   # Example for x86_64 / amd64
   curl -LO https://github.com/Arya-Penangsang/gonas/releases/download/v2.5.0-beta.1/gonas-v2.5.0-beta.1-linux-amd64.tar.gz
   tar -xzf gonas-v2.5.0-beta.1-linux-amd64.tar.gz
   sudo mv gonas /usr/local/bin/gonas
   sudo chmod +x /usr/local/bin/gonas
   ```

2. **Prepare directories and storage path:**
   ```bash
   sudo mkdir -p /var/nas/storage
   sudo chown -R $USER:$USER /var/nas
   ```

3. **Create Systemd Service (`/etc/systemd/system/gonas.service`):**
   ```ini
   [Unit]
   Description=GoNAS - Ultra-Lightweight Cloud NAS
   After=network.target network-online.target
   Wants=network-online.target

   [Service]
   Type=simple
   User=root
   WorkingDirectory=/var/nas
   ExecStart=/usr/local/bin/gonas -port 8082 -root /var/nas/storage -title "GoNAS Cloud Server"
   Restart=always
   RestartSec=3s
   LimitNOFILE=65535

   [Install]
   WantedBy=multi-user.target
   ```

4. **Enable and Start Service:**
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now gonas.service
   sudo systemctl status gonas
   ```

---

## 🍓 Raspberry Pi & Single Board Computers

GoNAS is compiled in pure Go with zero CGO dependencies, making it remarkably fast and memory-efficient (< 30 MB RAM) on ARM SBCs such as Raspberry Pi (3B, 4B, 5, Zero 2W), Orange Pi, and Rock Pi.

### Architecture Support:
- **64-bit OS (Raspberry Pi OS 64-bit / Armbian ARM64):** Uses `arm64` binary.
- **32-bit OS (Raspberry Pi OS 32-bit / Raspbian):** Uses `arm-armv7` binary.

Run the automated installer on your SBC:
```bash
curl -fsSL https://raw.githubusercontent.com/Arya-Penangsang/gonas/main/install.sh | sudo bash
```

To mount an external USB drive as the GoNAS storage root:
```bash
# Example mounting an external drive at /mnt/usb_storage
sudo sed -i 's|-root /var/nas/storage|-root /mnt/usb_storage|g' /etc/systemd/system/gonas.service
sudo systemctl daemon-reload
sudo systemctl restart gonas
```

---

## 🪟 Windows Setup

### Method A: Automated PowerShell Installer (Recommended)
Open **PowerShell as Administrator** and execute:
```powershell
irm https://raw.githubusercontent.com/Arya-Penangsang/gonas/main/install.ps1 | iex
```

### Method B: Manual Setup

1. Download `gonas-v2.5.0-beta.1-windows-amd64.zip` from [Releases](https://github.com/Arya-Penangsang/gonas/releases).
2. Extract contents to `C:\GoNAS`.
3. Create a subfolder `C:\GoNAS\storage`.
4. Open **Command Prompt** or **PowerShell** and run:
   ```powershell
   cd C:\GoNAS
   .\gonas.exe -port 8082 -root "C:\GoNAS\storage"
   ```
5. *(Optional)* To run silently on boot as a Windows Scheduled Task:
   ```powershell
   $action = New-ScheduledTaskAction -Execute "C:\GoNAS\gonas.exe" -Argument "-port 8082 -root `"C:\GoNAS\storage`"" -WorkingDirectory "C:\GoNAS"
   $trigger = New-ScheduledTaskTrigger -AtStartup
   $principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
   Register-ScheduledTask -TaskName "GoNAS_Server" -Action $action -Trigger $trigger -Principal $principal
   Start-ScheduledTask -TaskName "GoNAS_Server"
   ```

---

## 🐳 Docker & Docker Compose

GoNAS provides an ultra-minimal Alpine/Scratch-based container image.

### `docker-compose.yml`
```yaml
version: '3.8'

services:
  gonas:
    image: ghcr.io/arya-penangsang/gonas:latest
    container_name: gonas
    restart: unless-stopped
    ports:
      - "8082:8082"
    volumes:
      - ./data:/app/data
      - /mnt/storage:/nas/storage
    environment:
      - PORT=8082
      - ROOT_PATH=/nas/storage
      - DATA_PATH=/app/data
```

Start the container:
```bash
docker compose up -d
```

Check logs:
```bash
docker compose logs -f gonas
```

---

## 🍎 macOS (Darwin)

GoNAS runs natively on macOS (Apple Silicon M1/M2/M3/M4 and Intel x86_64).

1. **Install via one-liner:**
   ```bash
   curl -fsSL https://raw.githubusercontent.com/Arya-Penangsang/gonas/main/install.sh | sudo bash
   ```
2. **Or run directly:**
   ```bash
   curl -LO https://github.com/Arya-Penangsang/gonas/releases/download/v2.5.0-beta.1/gonas-v2.5.0-beta.1-darwin-arm64.tar.gz
   tar -xzf gonas-v2.5.0-beta.1-darwin-arm64.tar.gz
   ./gonas -port 8082 -root "$HOME/Documents/NAS"
   ```

---

## 🔨 Building from Source

If you wish to compile GoNAS directly from the latest source code:

### Prerequisites:
- Go 1.21 or higher installed (`go version`).
- Git.

### Build Commands:
```bash
# 1. Clone repository
git clone https://github.com/Arya-Penangsang/gonas.git
cd gonas

# 2. Download dependencies
go mod download

# 3. Compile optimized single binary
# Linux / macOS:
CGO_ENABLED=0 go build -ldflags="-s -w" -o gonas .

# Windows (PowerShell):
$env:CGO_ENABLED="0"; go build -ldflags="-s -w" -o gonas.exe .
```

---

## 🔑 Post-Installation & Default Credentials

After launching GoNAS, open your favorite web browser and visit:
👉 **`http://<SERVER_IP>:8082`** (or `http://localhost:8082` locally).

| Role | Username | Default Password | Permissions |
| :--- | :--- | :--- | :--- |
| **Administrator** | `admin` | `admin123` | Full access, user management, volume manager, terminal audit logs. |
| **Standard User** | `staf` | `staf123` | Read, upload, download, create folders. |

> [!WARNING]
> Please change the default passwords immediately after your first login via the **Admin -> User Management** settings panel.

---

## ⚙️ CLI Arguments & Configuration Flags

GoNAS supports flexible startup flags:

```text
Usage of gonas:
  -port int
        HTTP & WebDAV server listening port (default: 8082)
  -root string
        Root directory path for file storage (default: current directory or /storage)
  -data string
        Directory path for user database and audit logs (default: same as executable directory)
  -title string
        Custom organization title displayed on the dashboard (default: "GoNAS Private Cloud")
  -debug
        Enable verbose debug logging
```

Example custom execution:
```bash
./gonas -port 9000 -root "/mnt/hdd/data" -title "My Home NAS"
```

---

## ❓ Need Help?
- Refer to [Troubleshooting & Error Guide](TROUBLESHOOTING.md) for solutions to common issues (WebDAV 405 loop, Windows registry fix, port conflicts, Tailscale setup).
- File an issue or feature request on [GitHub Issues](https://github.com/Arya-Penangsang/gonas/issues).
