# Architecture & Technical Design: GoNAS

Dokumen ini menjelaskan arsitektur teknis, desain komponen, dan prinsip rekayasa di balik **GoNAS (Cross-Platform Cloud NAS)**.

---

## 1. Arsitektur Komponen

```
+-------------------------------------------------------------------+
|                        Client Web Browser                         |
|  - Vanilla ES6 Reactive Architecture (Zero JS Framework Overhead) |
|  - Tailwind CSS Glassmorphism & High-Contrast Light/Dark Themes   |
|  - Mozilla PDF.js (PDF Canvas Viewer)                             |
|  - Mammoth.js (Word .docx Direct Client Renderer)                 |
|  - SheetJS (Excel .xlsx / .csv Table Data Visualizer)             |
|  - HTML5 Audio/Video Players with HTTP 206 Partial Streaming      |
|  - Lucide SVG Vector Icons & Responsive Tablet/Touch Actions      |
+---------------------------------+---------------------------------+
                                  | HTTP/1.1 & HTTP/2 (Port 8082)
                                  v
+-------------------------------------------------------------------+
|                      GoNAS Core (Golang Engine)                   |
|                                                                   |
|  +--------------------+  +------------------+  +---------------+  |
|  | Authentication &   |  | Embedded Web UI  |  | In-Memory     |  |
|  | Role Hierarchy UAC |  | Assets           |  | Audit Trail   |  |
|  | (Session HMAC)     |  | (embed.FS)       |  | (150 logs)    |  |
|  +--------------------+  +------------------+  +---------------+  |
|                                                                   |
|  +--------------------+  +------------------+  +---------------+  |
|  | Multi-Volume Pool  |  | Smart Classifier |  | Native Kernel |  |
|  | Aggregator         |  | Auto-Arrange     |  | Telemetry     |  |
|  +--------------------+  +------------------+  +---------------+  |
|                                                                   |
|  +--------------------+  +------------------+  +---------------+  |
|  | Stream I/O Engine  |  | Safe Path &      |  | Collision     |  |
|  | (Chunk 256KB)      |  | Anti-Traversal   |  | Resolver      |  |
|  +--------------------+  +------------------+  +---------------+  |
+---------------------------------+---------------------------------+
                                  | Native OS File I/O
                                  v
+-------------------------------------------------------------------+
|                     Physical Storage Subsystem                    |
|   Primary Storage Volume (Local Disk / Mount Point / Container)   |
|   Secondary & Archival Pools (Additional Drives / Partitions)     |
+-------------------------------------------------------------------+
```

---

## 2. Poin-Poin Desain Kunci

### A. Zero External Dependencies & Single Binary Embedding
Aplikasi memanfaatkan fitur compiler bawaan Go:
```go
//go:embed web/*
var embeddedWebFS embed.FS
```
Dengan direktif ini, seluruh antarmuka Single-Page Application (`index.html`, Tailwind CSS, skrip viewer, dan ikon) langsung terkompilasi ke dalam biner portabel `gonas`. Tidak ada dependensi eksternal seperti Node.js, runtime Python, atau web server Apache/Nginx.

### B. Hierarchical User Access Control (UAC) & Persistent Store
GoNAS mengimplementasikan sistem UAC 4 tingkatan dengan penyimpanan persisten `users.json`:
1. **Superadmin**: Menguasai manajemen seluruh pengguna, konfigurasi volume, optimasi sistem, dan audit log.
2. **Admin**: Mengakses telemetri sistem dan pengelolaan volume disk.
3. **Staf (User)**: Akses file manager penuh (baca, tulis, unggah, pindah, hapus). Dashboard sistem disembunyikan secara visual dan dilindungi di sisi middleware API.
4. **Tamu (Viewer)**: Hak akses *read-only*. Seluruh aksi tulis (`POST /api/fs/*`) ditolak otomatis dengan status `403 Forbidden`.

Jika berkas basis data akun rusak atau belum tersedia, GoNAS secara otomatis memulihkan akun default `superadmin` dan `admin` untuk menjamin ketersediaan sistem.

### C. Universal Multi-Volume Storage Pooling
GoNAS memetakan banyak partisi penyimpanan fisik atau direktori mount ke dalam satu antarmuka terpadu:
- **Primary Volume (`vol-main`)**: Volume penyimpanan utama (ditentukan oleh argumen `-root` atau environment variable `NAS_ROOT_PATH`).
- **Secondary & Extra Volumes**: Direktori penyimpanan sekunder, partisi disk terpisah, atau media penyimpanan eksternal yang di-mount ke sistem host.

Setiap operasi API memvalidasi parameter `vol` melalui fungsi `safePathVol(vol, subpath)` untuk menjamin integritas isolasi volume.

### D. Cross-Platform Kernel Disk Telemetry via Go Build Tags
Pengukuran telemetri disk dan kapasitas ruang bebas dirancang modular menggunakan Go build tags:
- **Windows (`disk_windows.go`)**: Memanfaatkan Win32 Syscall langsung `GetDiskFreeSpaceExW` via `syscall.NewLazyDLL` dengan waktu eksekusi sub-milidetik (< 0.1 ms) tanpa overhead CPU.
- **POSIX Linux, Docker & macOS (`disk_posix.go`)**: Memanfaatkan panggilan standar `syscall.Statfs` native OS untuk menghitung total blok, ukuran blok, dan ruang bebas aktual.

### E. Streaming I/O Buffer Memory Management
Untuk menjaga konsumsi RAM server tetap stabil di kisaran ~6.3 MB saat menangani unggahan atau unduhan berkas berukuran gigabita, GoNAS menggunakan streaming chunked transfer:
```go
buf := make([]byte, 256*1024) // 256KB chunk buffer
_, err = io.CopyBuffer(out, part, buf)
```
Berkas langsung dialirkan dari soket TCP jaringan ke storage disk tanpa membebani memori utama server.

### F. Keamanan Path (Anti Directory Traversal)
Fungsi `safePathVol(vol, subpath)` melakukan sanitasi jalur ganda:
1. `filepath.Clean(filepath.FromSlash(subpath))` untuk meniadakan eksploitasi `../` atau `..\`.
2. Verifikasi batas (`strings.HasPrefix(fullClean, rootClean)`), memastikan akses berkas tidak dapat menyeberang ke luar partisi root volume yang ditentukan.

### G. Collision Avoidance & Self-Move Safety
- **Proteksi Pemindahan Berkas Rekursif**: Sistem secara otomatis menolak pemindahan direktori ke dalam dirinya sendiri atau ke dalam subfoldernya sendiri dengan HTTP 400.
- **Penyelesaian Tabrakan Nama**: Jika berkas dipindahkan ke direktori yang sudah memiliki nama berkas yang sama, GoNAS secara otomatis menghasilkan nama unik berurutan `nama (1).ext` untuk mencegah kegagalan operasi filesystem.
