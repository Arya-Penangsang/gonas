# 📦 Panduan Instalasi GoNAS

Panduan komprehensif cara memasang dan menjalankan **GoNAS** (Go Network Attached Storage & Web File Manager) di berbagai sistem operasi dan arsitektur perangkat keras.

---

## ⚡ Quick Start 1 Menit

### 🤖 Opsi 1: Pemasangan Otomatis Menggunakan AI Coding Agent (Paling Mudah)
Jika Anda bekerja bersama asisten AI (seperti **Google Antigravity**, **Claude Code**, **Cursor**, **Windsurf**, atau **GitHub Copilot**), Anda tidak perlu mengetik perintah terminal secara manual. Cukup salin perintah berikut dan kirimkan ke chat AI Agent Anda:

```text
Tolong pasang dan jalankan GoNAS dari https://github.com/Arya-Penangsang/gonas di komputer ini:
1. Unduh atau compile biner native GoNAS sesuai sistem operasi mesin ini.
2. Siapkan direktori penyimpanan di ./storage dan jalankan GoNAS sebagai background service di port 8082.
3. Jika di Windows, pastikan service WebClient aktif, setel registry BasicAuthLevel = 2, dan hubungkan WebDAV sebagai Network Drive Z: (URL: http://localhost:8082/webdav, user: admin, pass: admin123).
4. Verifikasi bahwa server berjalan normal dan berikan saya link Web Dashboard yang bisa langsung diklik.
```

---

### 💻 Opsi 2: Skrip Otomatis Satu Baris (Terminal / CLI)

Pilih sistem operasi Anda untuk menjalankan installer otomatis:

### Linux / Armbian / Raspberry Pi / macOS
Buka terminal dan jalankan perintah satu baris berikut:
```bash
curl -fsSL https://raw.githubusercontent.com/Arya-Penangsang/gonas/main/install.sh | sudo bash
```
> **Yang dilakukan skrip ini secara otomatis:** Mendeteksi OS (`linux`/`darwin`) dan arsitektur CPU (`amd64`, `arm64`, `armv7`), mengunduh biner rilis terbaru, menyiapkan folder penyimpanan `/var/nas/storage`, membuat akun awal (`admin` / `admin123`), dan mendaftarkan service **systemd** di latar belakang (`gonas.service`).

### Windows (PowerShell Administrator)
Buka **PowerShell sebagai Administrator** dan jalankan:
```powershell
irm https://raw.githubusercontent.com/Arya-Penangsang/gonas/main/install.ps1 | iex
```
> **Yang dilakukan skrip ini secara otomatis:** Mengunduh biner `gonas.exe` ke `C:\GoNAS`, menyiapkan folder penyimpanan, membuka port `8082` di Windows Firewall, mengaktifkan registry WebClient WebDAV (`BasicAuthLevel = 2`), dan mendaftarkan service otomatis via Windows Task Scheduler (`GoNAS_Server`).

---

## 📑 Daftar Isi
1. [Linux & Armbian (Systemd Service)](#-linux--armbian-systemd-service)
2. [Raspberry Pi & Single Board Computer (SBC)](#-raspberry-pi--single-board-computer-sbc)
3. [Windows (Aplikasi & Scheduled Task)](#-windows-setup)
4. [Docker & Docker Compose](#-docker--docker-compose)
5. [macOS (Apple Silicon & Intel)](#-macos-darwin)
6. [Kompilasi Manual dari Source Code](#-kompilasi-dari-source-code)
7. [Kredensial Default & Pasca Instalasi](#-kredensial-default--pasca-instalasi)
8. [Argumen Baris Perintah (CLI Flags)](#-argumen-baris-perintah-cli-flags)

---

## 🐧 Linux & Armbian (Systemd Service)

### Cara A: Installer Otomatis (Direkomendasikan)
```bash
curl -fsSL https://raw.githubusercontent.com/Arya-Penangsang/gonas/main/install.sh | sudo bash
```

### Cara B: Instalasi Manual Langkah-demi-Langkah

1. **Unduh biner rilis** sesuai arsitektur CPU Anda dari [Halaman Release GitHub](https://github.com/Arya-Penangsang/gonas/releases):
   ```bash
   # Contoh untuk arsitektur x86_64 / amd64
   curl -LO https://github.com/Arya-Penangsang/gonas/releases/download/v2.5.0-beta.1/gonas-v2.5.0-beta.1-linux-amd64.tar.gz
   tar -xzf gonas-v2.5.0-beta.1-linux-amd64.tar.gz
   sudo mv gonas /usr/local/bin/gonas
   sudo chmod +x /usr/local/bin/gonas
   ```

2. **Siapkan direktori data dan penyimpanan:**
   ```bash
   sudo mkdir -p /var/nas/storage
   sudo chown -R $USER:$USER /var/nas
   ```

3. **Buat file service Systemd (`/etc/systemd/system/gonas.service`):**
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

4. **Aktifkan dan Jalankan Layanan:**
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now gonas.service
   sudo systemctl status gonas
   ```

---

## 🍓 Raspberry Pi & Single Board Computer (SBC)

GoNAS dikompilasi murni menggunakan bahasa Go tanpa dependensi CGO luar. Sangat hemat memori RAM (< 30 MB) dan sangat cocok dipasang di Raspberry Pi (3B, 4B, 5, Zero 2W), Orange Pi, STB Armbian, atau Rock Pi.

### Panduan Arsitektur:
- **OS 64-bit (Raspberry Pi OS 64-bit / Armbian aarch64):** Otomatis menggunakan biner `arm64`.
- **OS 32-bit (Raspberry Pi OS 32-bit / Raspbian armhf):** Otomatis menggunakan biner `arm-armv7`.

Jalankan installer satu baris di terminal SBC Anda:
```bash
curl -fsSL https://raw.githubusercontent.com/Arya-Penangsang/gonas/main/install.sh | sudo bash
```

Jika Anda ingin menggunakan Harddisk Eksternal / Flashdisk USB sebagai penyimpanan utama:
```bash
# Contoh jika flashdisk dipasang di /mnt/hdd_eksternal
sudo sed -i 's|-root /var/nas/storage|-root /mnt/hdd_eksternal|g' /etc/systemd/system/gonas.service
sudo systemctl daemon-reload
sudo systemctl restart gonas
```

---

## 🪟 Windows Setup

### Cara A: Installer Otomatis PowerShell (Direkomendasikan)
Buka **PowerShell sebagai Administrator**, salin dan tempel perintah ini:
```powershell
irm https://raw.githubusercontent.com/Arya-Penangsang/gonas/main/install.ps1 | iex
```

### Cara B: Instalasi Manual

1. Unduh `gonas-v2.5.0-beta.1-windows-amd64.zip` dari [Halaman Releases](https://github.com/Arya-Penangsang/gonas/releases).
2. Ekstrak file zip ke folder pilihan Anda, contoh: `C:\GoNAS`.
3. Buat subfolder penyimpanan di `C:\GoNAS\storage`.
4. Buka **Command Prompt** atau **PowerShell** dan jalankan:
   ```powershell
   cd C:\GoNAS
   .\gonas.exe -port 8082 -root "C:\GoNAS\storage"
   ```
5. *(Opsional)* Agar berjalan otomatis saat komputer dinyalakan tanpa jendela hitam:
   ```powershell
   $action = New-ScheduledTaskAction -Execute "C:\GoNAS\gonas.exe" -Argument "-port 8082 -root `"C:\GoNAS\storage`"" -WorkingDirectory "C:\GoNAS"
   $trigger = New-ScheduledTaskTrigger -AtStartup
   $principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
   Register-ScheduledTask -TaskName "GoNAS_Server" -Action $action -Trigger $trigger -Principal $principal
   Start-ScheduledTask -TaskName "GoNAS_Server"
   ```

---

## 🐳 Docker & Docker Compose

GoNAS menyediakan image container berbasis Alpine/Scratch yang sangat ringan.

### File `docker-compose.yml`
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

Jalankan container:
```bash
docker compose up -d
```

Melihat status log:
```bash
docker compose logs -f gonas
```

---

## 🍎 macOS (Darwin)

GoNAS berjalan optimal di Apple Silicon (M1/M2/M3/M4) maupun Mac Intel x86_64.

1. **Jalankan skrip satu baris di Terminal:**
   ```bash
   curl -fsSL https://raw.githubusercontent.com/Arya-Penangsang/gonas/main/install.sh | sudo bash
   ```
2. **Atau jalankan biner secara portabel:**
   ```bash
   curl -LO https://github.com/Arya-Penangsang/gonas/releases/download/v2.5.0-beta.1/gonas-v2.5.0-beta.1-darwin-arm64.tar.gz
   tar -xzf gonas-v2.5.0-beta.1-darwin-arm64.tar.gz
   ./gonas -port 8082 -root "$HOME/Documents/NAS"
   ```

---

## 🔨 Kompilasi dari Source Code

Jika Anda ingin membangun biner GoNAS langsung dari kode sumber terbaru:

### Persyaratan:
- Terpasang Go 1.21 ke atas (`go version`).
- Git.

### Langkah Kompilasi:
```bash
# 1. Clone repositori
git clone https://github.com/Arya-Penangsang/gonas.git
cd gonas

# 2. Unduh dependensi
go mod download

# 3. Kompilasi biner tunggal teroptimasi
# Untuk Linux / macOS:
CGO_ENABLED=0 go build -ldflags="-s -w" -o gonas .

# Untuk Windows (PowerShell):
$env:CGO_ENABLED="0"; go build -ldflags="-s -w" -o gonas.exe .
```

---

## 🔑 Kredensial Default & Pasca Instalasi

Setelah GoNAS berjalan, buka browser web pilihan Anda (Chrome, Edge, Firefox, Safari) dan akses:
👉 **`http://<IP_KOMPUTER>:8082`** (atau `http://localhost:8082` jika di komputer yang sama).

| Peran (Role) | Username | Password Default | Hak Akses |
| :--- | :--- | :--- | :--- |
| **Administrator** | `admin` | `admin123` | Akses penuh, manajemen pengguna, ganti volume storage, audit log. |
| **Pengguna Standar** | `staf` | `staf123` | Baca, upload, download, buat folder. |

> [!WARNING]
> Sangat disarankan untuk segera mengganti kata sandi bawaan melalui panel **Admin -> User Management** setelah pertama kali masuk.

---

## ⚙️ Argumen Baris Perintah (CLI Flags)

Parameter yang dapat digunakan saat menjalankan biner `gonas`:

```text
Penggunaan gonas:
  -port int
        Port server HTTP & WebDAV (default: 8082)
  -root string
        Path folder utama penyimpanan file (default: direktori saat ini atau /storage)
  -data string
        Path folder tempat database pengguna & audit log users.json (default: direktori biner)
  -title string
        Nama judul instansi yang tampil di dashboard Web (default: "GoNAS Private Cloud")
  -debug
        Mengaktifkan mode log debug terperinci
```

Contoh kustomisasi:
```bash
./gonas -port 9000 -root "/mnt/datadisk/nas" -title "NAS Kantor Pusat"
```

---

## ❓ Butuh Bantuan Tambahan?
- Lihat [Panduan Penanganan Eror & Troubleshooting](TROUBLESHOOTING.id.md) untuk solusi kendala umum (popup WebDAV di browser, registry WebClient Windows, port terpakai, dsb).
- Laporkan kendala atau saran fitur baru di [GitHub Issues](https://github.com/Arya-Penangsang/gonas/issues).
