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

### F. Keamanan Path (Strict Relpath Anti-Directory Traversal Sandboxing)
Fungsi `safePathVol(vol, subpath)` dan `safePath(subpath)` menerapkan isolasi jalur bertingkat:
1. Normalisasi jalur: `filepath.Clean(filepath.FromSlash(subpath))` untuk meniadakan eksploitasi `../` atau `..\`.
2. **Strict Containment via `filepath.Rel`**: Verifikasi relasi hierarki menggunakan `filepath.Rel(rootClean, fullClean)`. Jika hasil relasi diawali dengan `..` atau menghasilkan error (misal lompatan antar drive pada Windows), akses ditolak seketika (`HTTP 403 Forbidden`). Mekanisme ini mengeliminasi celah *sibling directory prefix bypass* (CWE-22).

### G. Collision Avoidance & Self-Move Safety
- **Proteksi Pemindahan Berkas Rekursif**: Sistem secara otomatis menolak pemindahan direktori ke dalam dirinya sendiri atau ke dalam subfoldernya sendiri dengan HTTP 400.
- **Penyelesaian Tabrakan Nama**: Jika berkas dipindahkan ke direktori yang sudah memiliki nama berkas yang sama, GoNAS secara otomatis menghasilkan nama unik berurutan `nama (1).ext` untuk mencegah kegagalan operasi filesystem.

---

## 3. Arsitektur Pertahanan & Keamanan Siber (v2.4.1 Hardened)

Berdasarkan hasil asesmen *Purple Teaming* (kolaborasi mitigasi ancaman MITRE ATT&CK for Web dan NIST CSF), GoNAS v2.4.1 mengimplementasikan arsitektur *Defense-in-Depth*:

1. **Cryptographic Salted Password Hashing (`sha256$<salt>$<hash>`)**:
   Kata sandi tidak lagi disimpan dalam plaintext. Sistem membuat garam acak 16-byte menggunakan CSPRNG `crypto/rand` dan meng-hash dengan SHA-256. Komparasi sandi dilakukan dengan `crypto/subtle.ConstantTimeCompare` untuk mencegah serangan analisis waktu (*timing attacks*). Izin berkas `users.json` dibatasi ke `0600`.
2. **In-Memory Brute-Force Rate Limiting Engine**:
   Pelacak upaya login berbasis IP secara thread-safe (`sync.Mutex`). Kegagalan login 5 kali berturut-turut memicu penguncian sementara selama 5 menit dengan respons `HTTP 429 Too Many Requests` dan header standar `Retry-After`.
3. **Stored XSS Neutralization via CSP Sandbox**:
   Setiap berkas interaktif atau markup (`.html`, `.htm`, `.svg`, `.xml`) yang disajikan melalui endpoint `/api/fs/preview` disuntikkan header `Content-Security-Policy: default-src 'none'; sandbox`. Peramban secara otomatis mematikan eksekusi JavaScript dan memperlakukan konten dalam origin tertutup.
4. **Global HTTP Security Headers**:
   Seluruh respons server dilindungi oleh middleware keamanan terpusat:
   - `X-Frame-Options: SAMEORIGIN` (Pencegahan Clickjacking)
   - `X-Content-Type-Options: nosniff` (Pencegahan MIME Confusion)
   - `Referrer-Policy: strict-origin-when-cross-origin`
   - `Permissions-Policy: camera=(), microphone=(), geolocation=()`
   - `X-XSS-Protection: 1; mode=block`
5. **Slowloris DoS Mitigation**:
   Konfigurasi HTTP Server menerapkan `ReadHeaderTimeout: 15 * time.Second` untuk memutus koneksi lambat berbahaya sebelum menghabiskan *file descriptor* atau goroutine server.
6. **Leaked Descriptor Prevention**:
   Operasi streaming arsip ZIP menutup berkas seketika di dalam iterasi *walk* peramban direktori tanpa penundaan `defer`.
