# 🚀 GoNAS — High-Performance Lightweight Cloud NAS Server

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Platform](https://img.shields.io/badge/Platform-Windows%20x64%20|%20Linux%20|%20Docker-0078D6?style=flat&logo=windows)](https://microsoft.com)
[![Memory Footprint](https://img.shields.io/badge/RAM%20Usage-~6.4%20MB-10B981?style=flat)]()
[![Architecture](https://img.shields.io/badge/Architecture-Single%20Static%20Binary-6366F1?style=flat)]()
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**GoNAS** adalah engine Cloud Web NAS mandiri ultra-ringan, berkecepatan tinggi, dan berpenampilan modern yang dirancang khusus untuk lingkungan preservasi arsip digital BPMN dan perkantoran.

Dibangun dengan **Go murni (Golang)** dan mengintegrasikan antarmuka Single-Page Application (SPA) berbasis **Tailwind CSS & Lucide Icons** secara langsung ke dalam biner (`//go:embed`), GoNAS berjalan sebagai satu berkas portabel tanpa memerlukan runtime eksternal (zero-dependency, tanpa Node.js, Python, ataupun database SQL eksternal).

---

## ✨ Fitur Utama (v2.4 Move & UAC Edition)

### 1. 📁 Modern File Manager & Document Viewer
- **Dual View Modes**: Beralih seketika antara **List Table View** (berkontras tinggi dengan indikator ukuran folder dan pemilik) dan **Grid View** (kartu berkas responsif).
- **Embedded Document Viewer**:
  - **PDF Reader**: Canvas viewer interaktif berbasis Mozilla PDF.js dengan zoom in/out, navigasi halaman, dan pratinjau instan.
  - **Microsoft Word (.docx)**: Rendering HTML dokumen langsung di browser tanpa Microsoft Office via Mammoth.js.
  - **Excel & Spreadsheet (.xlsx, .xls, .csv)**: Visualisasi tabel spreadsheet interaktif via SheetJS.
  - **Multimedia Player**: Pemutar Video HTML5 & Pemutar Musik/Audio dengan dukungan HTTP 206 Partial Content (range-request seeking).
  - **Teks & Kode Sumber**: Penampil monospace dengan syntax formatting dan tombol salin instan.
- **Hierarchical Sidebar Folder Tree**: Navigasi pohon direktori bertingkat dengan ekspansi panah drop-down dinamis dan ukuran folder kalkulatif.
- **Whole-Window Drag & Drop Upload**: Seret berkas dari komputer mana saja ke browser dengan visual backdrop glow blur dan progress percentage upload yang halus.

### 2. 🚚 Seamless Move & Bulk Move Engine
- **Interactive Move Picker**: Modal pemindahan berkas dengan visual tree folder, navigasi breadcrumbs (`Root › Folder › Subfolder`), dan pemilihan folder target dalam satu ketukan.
- **Floating Bulk Action Bar**: Bilah aksi mengambang saat memilih banyak item untuk memindahkan (*bulk move*), merapikan (*auto-arrange*), mengunduh ZIP (*download*), atau menghapus (*delete*).
- **Self-Move & Collision Protection**: Proteksi otomatis agar folder tidak dapat dipindahkan ke dalam dirinya sendiri dan penanganan tabrakan nama berkas otomatis (*auto rename collision*).
- **Touch & Tablet Friendly**: Tombol aksi pada baris berkas dioptimalkan untuk perangkat sentuh/tablet tanpa bergantung pada kursor mouse hover.

### 3. 🛡️ User Access Control (UAC) & Role Hierarchy
Sistem otentikasi berbasis token HMAC / Session Cookies (`gonas_session`) dengan klasifikasi 4 tingkatan hak akses:

| Peran (Role) | Akses File Manager | Unggah & Hapus | Manajemen Pengguna | Dashboard Telemetri |
| :--- | :---: | :---: | :---: | :---: |
| **Superadmin** | ✅ Penuh | ✅ Ya | ✅ Ya | ✅ Ya |
| **Admin** | ✅ Penuh | ✅ Ya | ❌ Tidak | ✅ Ya |
| **Staf (User)** | ✅ Penuh | ✅ Ya | ❌ Tersembunyi | ❌ Tersembunyi |
| **Tamu (Viewer)** | 👁️ Baca & Unduh | ❌ Diblokir | ❌ Tersembunyi | ❌ Tersembunyi |

### 4. 💽 Multi-Volume Storage Pooling
Mendukung agregasi penyimpanan lintas partisi fisik secara transparan:
- **Storage Utama (`vol-n`)**: Volume primer dedicated NAS (`N:\`).
- **Gudang Ekstra (`vol-d`)**: Volume sekunder berkapasitas besar (`D:\`).
- **Arsip Dinas (`vol-e`)**: Volume preservasi arsip jangka panjang (`E:\`).

### 5. 🪄 Smart Auto-Arrange (Rapikan Otomatis)
Fitur klasifikasi cerdas untuk menyortir berkas tercecer di direktori root secara otomatis ke folder tematik terstandarisasi:
- `01_Preservasi_dan_Koleksi_Digital`
- `02_Administrasi_dan_Dokumen_Kedinasan`
- `03_Backup_dan_Database_Sistem`
- `04_Media_Dokumentasi_dan_Kamera`
- `05_Unduhan_dan_Installer`

### 6. ⚡ Kernel Syscall Telemetry & In-Memory Audit
- **Sub-milidetik Disk Telemetry**: Membaca kapasitas disk langsung melalui Windows Kernel32 `GetDiskFreeSpaceExW` tanpa overhead proses eksternal.
- **In-Memory Audit Logger**: Melacak seluruh aktivitas login, unggah, unduh, hapus, dan pemindahan berkas dengan pencatatan IP klien real-time.

---

## 🏗️ Spesifikasi Teknis & Arsitektur

```
+-------------------------------------------------------------------+
|                        Client Web Browser                         |
|  - Vanilla ES6 Reactive Architecture (Zero Framework Overhead)    |
|  - Tailwind CSS Glassmorphism Dark/Light Theme                    |
|  - Mozilla PDF.js + Mammoth.js + SheetJS Embedded Viewers         |
|  - Lucide Icons & Responsive Touch/Tablet Layout                  |
+---------------------------------+---------------------------------+
                                  | HTTP/1.1 & HTTP/2 (Port 8082)
                                  v
+-------------------------------------------------------------------+
|                     GoNAS Core (Golang Engine)                    |
|                                                                   |
|  +--------------------+  +------------------+  +---------------+  |
|  | Authentication &   |  | Embedded Web UI  |  | In-Memory     |  |
|  | Role Hierarchy UAC |  | (embed.FS)       |  | Audit Trail   |  |
|  +--------------------+  +------------------+  +---------------+  |
|                                                                   |
|  +--------------------+  +------------------+  +---------------+  |
|  | Multi-Volume Pool  |  | Smart Classifier |  | Win32 Syscall |  |
|  | (vol-n, d, e)      |  | Auto-Arrange     |  | Telemetry     |  |
|  +--------------------+  +------------------+  +---------------+  |
+---------------------------------+---------------------------------+
                                  | Native I/O (Streaming Buffer 256KB)
                                  v
+-------------------------------------------------------------------+
|                      Physical Storage Volumes                     |
|        Drive N:\ (Primary) | Drive D:\ & E:\ (Secondary/Arsip)    |
+-------------------------------------------------------------------+
```

- **Ukuran Biner**: ~6.4 MB (kompilasi statis dengan `-ldflags "-s -w"`).
- **Penggunaan RAM**: ~6.3 MB saat idle / normal load.
- **I/O Streaming**: Buffer chunked 256 KB mencegah penumpukan memori saat menangani file berukuran gigabita.

---

## 🚀 Panduan Instalasi & Menjalankan

### Cara 1: Menjalankan di Windows (Biner Portabel)
Unduh atau kompilasi berkas `gonas.exe`, lalu jalankan melalui PowerShell atau Command Prompt:
```powershell
./gonas.exe -port 8082 -root "N:\" -user admin -pass admin123 -title "GoNAS - Cloud Preservasi BPMN"
```

#### Parameter Baris Perintah (Flags):
| Parameter | Default | Keterangan |
| :--- | :--- | :--- |
| `-port` | `8082` | Port HTTP listening |
| `-root` | `N:\` (atau path lokal) | Jalur folder penyimpanan utama |
| `-user` | `admin` | Username default administrator |
| `-pass` | `admin123` | Password default administrator |
| `-title` | `GoNAS - Cloud Preservasi BPMN` | Judul aplikasi di browser |

---

### Cara 2: Menjalankan Otomatis via Windows Task Scheduler
Untuk menjalankan GoNAS sebagai background service otomatis saat Windows menyala:
```powershell
$action = New-ScheduledTaskAction -Execute 'C:\NAS\gonas.exe' -Argument '-port 8082 -root N:\ -user admin -pass admin123'
$trigger = New-ScheduledTaskTrigger -AtStartup
Register-ScheduledTask -TaskName 'GoNAS' -Action $action -Trigger $trigger -RunLevel Highest -User 'SYSTEM'
```

---

### Cara 3: Menjalankan Menggunakan Docker / Docker Compose
GoNAS siap di-deploy di server Linux/Docker melalui kontainer ultra-ringan (< 25 MB):
```bash
# Clone repository
git clone https://github.com/nafik-cs/gonas.git
cd gonas

# Jalankan dengan Docker Compose
docker compose up -d
```

Akses web di: `http://localhost:8082` atau `http://<IP-SERVER>:8082`.

---

## 🛠️ Kompilasi dari Kode Sumber

Pastikan Anda telah menginstal **Go 1.22 atau lebih baru**:
```bash
# 1. Unduh kode sumber
git clone https://github.com/nafik-cs/gonas.git
cd gonas

# 2. Kompilasi biner Windows
go build -ldflags="-s -w" -o gonas.exe main.go

# 3. Kompilasi biner Linux
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o gonas main.go
```

---

## 📂 Struktur Repositori

```
gonas/
├── .gitignore              # Konfigurasi file yang diabaikan git
├── API.md                  # Dokumentasi lengkap REST API v2.4
├── ARCHITECTURE.md         # Rincian desain teknis dan arsitektur internal
├── Dockerfile              # Multi-stage Docker build ultra-ringan
├── docker-compose.yml      # Konfigurasi deployment kontainer
├── go.mod                  # Definisi modul Go
├── LICENSE                 # Lisensi open-source (MIT License)
├── main.go                 # Engine backend Go utama
├── README.md               # Dokumentasi utama proyek
└── web/                    # Antarmuka SPA (ter-embed ke dalam biner)
    └── index.html          # Frontend Tailwind CSS, Lucide & Viewer scripts
```

---

## 📄 Lisensi

Proyek ini dilisensikan di bawah **[MIT License](LICENSE)**. Bebas digunakan, dimodifikasi, dan didistribusikan untuk keperluan instansi, riset, maupun komersial.
