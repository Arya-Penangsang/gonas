# REST API Specification: GoNAS v2.4

Seluruh endpoint API privat dilindungi oleh middleware otentikasi dan menerima token sesi baik melalui cookie `gonas_session` (atau `aether_session`) maupun header `X-Auth-Token: <token>`.

---

## 1. Autentikasi & Sesi

### `POST /api/auth/login`
Memverifikasi kredensial pengguna dan menerbitkan token sesi serta informasi peran (UAC).
- **Request Body:**
  ```json
  {
    "username": "superadmin",
    "password": "admin123"
  }
  ```
- **Response Success (200 OK):**
  ```json
  {
    "success": true,
    "token": "49379c65a0b9e843c1782298ef9186638ad33b1e56b4f732",
    "user": "superadmin",
    "role": "superadmin",
    "display_name": "Super Administrator",
    "title": "GoNAS - Cloud Preservasi BPMN"
  }
  ```

### `POST /api/auth/logout`
Membatalkan token sesi aktif di memory server dan menghapus cookie sesi di peramban.

### `GET /api/auth/check`
Memverifikasi apakah token sesi saat ini masih sah dan mengembalikan status pengguna yang sedang masuk.

---

## 2. Operasi Berkas & Direktori (File System)

### `GET /api/fs/list?vol=<vol_id>&path=<relative_path>`
Mengambil daftar berkas dan folder pada volume dan path tertentu.
- **Query Parameters:**
  - `vol` (opsional): ID volume (`vol-n`, `vol-d`, `vol-e`). Default: volume utama (`vol-n`).
  - `path` (opsional): Jalur subdirektori relatif dari root volume.
- **Response Success (200 OK):**
  ```json
  {
    "current_path": "Dokumen",
    "breadcrumbs": [
      { "name": "Root (Storage Utama)", "path": "" },
      { "name": "Dokumen", "path": "Dokumen" }
    ],
    "items": [
      {
        "name": "Laporan.pdf",
        "path": "Dokumen/Laporan.pdf",
        "is_dir": false,
        "size": 145028,
        "size_display": "141.6 KB",
        "mod_time": "07 Nov 2026 13:20",
        "extension": ".pdf",
        "category": "doc",
        "owner": "superadmin"
      }
    ]
  }
  ```

### `POST /api/fs/upload`
Menerima upload berkas multipart (mendukung unggah banyak berkas sekaligus).
- **Form Data:**
  - `vol`: ID volume target.
  - `path`: Jalur subdirektori tujuan penyimpanan.
  - `files[]`: Berkas biner yang diunggah.

### `GET /api/fs/download?vol=<vol_id>&path=<relative_path>`
Mengunduh berkas tunggal (sebagai attachment) atau mengunduh folder (otomatis di-stream langsung sebagai arsip ZIP).

### `GET /api/fs/view?vol=<vol_id>&path=<relative_path>`
Streaming berkas inline untuk pratinjau dokumen (PDF, gambar, video, musik). Mendukung header HTTP `Range` (206 Partial Content) untuk pemutaran video berkecepatan tinggi tanpa buffer penuh.

### `POST /api/fs/move`
Memindahkan berkas atau folder tunggal ke folder tujuan lain.
- **Request Body:**
  ```json
  {
    "vol": "vol-n",
    "source": "Laporan.pdf",
    "target_dir": "Arsip_Preservasi"
  }
  ```
- **Response:**
  ```json
  {
    "success": true
  }
  ```

### `POST /api/fs/bulk-move`
Memindahkan beberapa berkas atau folder sekaligus ke direktori target.
- **Request Body:**
  ```json
  {
    "vol": "vol-n",
    "sources": ["Laporan1.pdf", "Foto.png", "Data.xlsx"],
    "target_dir": "Arsip_Preservasi"
  }
  ```

### `POST /api/fs/mkdir`
Membuat folder baru secara instan.
- **Request Body:**
  ```json
  {
    "vol": "vol-n",
    "parent": "",
    "name": "Koleksi_Naskah_Kuno"
  }
  ```

### `POST /api/fs/rename`
Mengganti nama berkas atau folder.
- **Request Body:**
  ```json
  {
    "vol": "vol-n",
    "old_path": "Draft.docx",
    "new_name": "Final.docx"
  }
  ```

### `POST /api/fs/delete`
Menghapus berkas atau direktori.
- **Request Body:**
  ```json
  {
    "vol": "vol-n",
    "path": "Dokumen/Temp.log"
  }
  ```

### `POST /api/fs/bulk-delete`
Menghapus banyak berkas/folder sekaligus.
- **Request Body:**
  ```json
  {
    "vol": "vol-n",
    "paths": ["file1.tmp", "file2.tmp"]
  }
  ```

### `POST /api/fs/group`
Membuat folder baru secara atomik dan memindahkan semua berkas terpilih ke dalamnya dalam satu operasi.
- **Request Body:**
  ```json
  {
    "vol": "vol-n",
    "folder_name": "Dokumen_Rapat_2026",
    "parent": "",
    "files": ["Notulen.pdf", "Presensi.xlsx"]
  }
  ```

### `POST /api/fs/bulk-zip`
Mengemas berkas-berkas terpilih ke dalam arsip ZIP secara streaming ke klien.
- **Request Body:**
  ```json
  {
    "vol": "vol-n",
    "paths": ["file1.pdf", "file2.pdf"]
  }
  ```

---

## 3. Smart Auto-Arrange (Rapikan Otomatis)

### `GET /api/fs/auto-arrange/scan?vol=<vol_id>`
Melakukan simulasi pemindaian berkas-berkas tercecer di direktori root dan mengembalikan rencana pengelompokan tematik tanpa mengubah data di disk.

### `POST /api/fs/auto-arrange/execute`
Mengeksekusi pemindahan berkas tercecer ke folder tematik yang sesuai secara fisik di disk.
- **Request Body:**
  ```json
  {
    "vol": "vol-n"
  }
  ```

---

## 4. Telemetri & Manajemen Sistem

### `GET /api/sys/stats`
Mengembalikan status telemetri disk seluruh volume (`vol-n`, `vol-d`, `vol-e`), kapasitas bebas/terpakai, RAM server, jumlah pengguna, dan versi GoNAS.

### `GET /api/sys/health`
Pemeriksaan kesehatan sistem (digunakan untuk Docker healthcheck). Mengembalikan `200 OK` `{ "status": "UP" }`.

### `GET /api/sys/audit`
Mengambil riwayat log audit in-memory (150 catatan aktivitas terakhir: user, IP klien, aksi, dan waktu).

### `POST /api/sys/password`
Mengubah kata sandi akun yang sedang aktif.

---

## 5. Manajemen Pengguna (UAC - Superadmin Only)

### `GET /api/users/list`
Mengambil daftar seluruh akun terdaftar beserta perannya (`superadmin`, `admin`, `user`, `viewer`).

### `POST /api/users/add`
Menambahkan akun pengguna baru.
- **Request Body:**
  ```json
  {
    "username": "petugas1",
    "display_name": "Petugas Preservasi",
    "password": "rahasia123",
    "role": "user"
  }
  ```

### `POST /api/users/delete`
Menghapus akun pengguna tertentu (proteksi akun master superadmin tidak dapat dihapus).
- **Request Body:**
  ```json
  {
    "username": "petugas1"
  }
  ```
