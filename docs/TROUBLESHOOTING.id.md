# 🛠️ Panduan Pemecahan Masalah & Solusi Eror GoNAS

Panduan lengkap mengenai penyebab dan penanganan kendala umum yang sering terjadi saat instalasi, pengoperasian, maupun pengaksesan **GoNAS**.

---

## 📑 Navigasi Cepat Masalah
1. [Muncul Popup Login Berulang / Eror 405 saat Membuka `/webdav/` di Browser](#1-muncul-popup-login-berulang--eror-405-saat-membuka-webdav-di-browser)
2. [Windows Explorer Gagal Memetakan (Map) Network Drive WebDAV](#2-windows-explorer-gagal-memetakan-map-network-drive-webdav)
3. [Port 8082 Sudah Dipakai (`bind: address already in use`)](#3-port-8082-sudah-dipakai-bind-address-already-in-use)
4. [Tidak Bisa Diakses dari Perangkat Lain / HP / Tailscale / Jaringan Lokal](#4-tidak-bisa-diakses-dari-perangkat-lain--hp--tailscale--jaringan-lokal)
5. [Lupa Password Admin / Akun Terkunci](#5-lupa-password-admin--akun-terkunci)
6. [Gagal Mengunggah File Berukuran Besar (> 1 GB)](#6-gagal-mengunggah-file-berukuran-besar--1-gb)
7. [Perintah Cek Log & Pengelolaan Layanan](#7-perintah-cek-log--pengelolaan-layanan)

---

## 1. Muncul Popup Login Berulang / Eror 405 saat Membuka `/webdav/` di Browser

### 🔍 Gejala yang Terlihat
- Saat mengetik alamat `http://<IP>:8082/webdav/` di Google Chrome, Edge, atau browser HP, muncul dialog popup native:
  > *"Login: http://... memerlukan nama pengguna dan sandi. Koneksi Anda ke situs ini tidak bersifat pribadi."*
- Setelah memasukkan username dan password, popup muncul kembali atau menampilkan halaman kosong bertuliskan `405 Method Not Allowed`.

```
+-----------------------------------------------------------------+
|  Login                                                          |
|  http://172.16.63.15:8082 memerlukan nama pengguna dan sandi.   |
|  Nama Pengguna: [                  ]                            |
|  Sandi        : [                  ]                            |
|                                       [ Batal ] [ Login ]       |
+-----------------------------------------------------------------+
```

### 💡 Penyebab Masalah
- Alamat `/webdav/` merupakan **titik kait protokol jaringan WebDAV (RFC 4918)** yang dirancang khusus untuk File Manager sistem operasi (seperti Windows Explorer, macOS Finder, atau aplikasi File Manager di ponsel seperti Solid Explorer / Cx File Explorer).
- Browser web mengirimkan permintaan `GET` untuk merender website, sedangkan endpoint direktori WebDAV hanya merespons perintah WebDAV (`PROPFIND`, `OPTIONS`, `MKCOL`).

### ✅ Cara Mengatasinya
1. **Untuk membuka antarmuka Web Dashboard GoNAS**, gunakan URL utama tanpa `/webdav/`:
   👉 **`http://<IP_KOMPUTER>:8082`** (contoh: `http://172.16.63.15:8082` atau `http://localhost:8082`).
2. Mulai versi **GoNAS v2.5.0+**, jika Anda membuka `/webdav/` melalui browser web, sistem otomatis mendeteksi browser dan menampilkan halaman panduan interaktif lengkap dengan tombol pintasan sekali klik menuju Dashboard Web.

---

## 2. Windows Explorer Gagal Memetakan (Map) Network Drive WebDAV

### 🔍 Gejala yang Terlihat
- Di Windows Explorer, saat memilih **Map Network Drive** dan memasukkan `http://<IP>:8082/webdav`, Windows memunculkan peringatan:
  > *"The folder you entered does not appear to be valid. Please choose another."*
  atau
  > *"Windows cannot access http://... Check the spelling of the name."*

### 💡 Penyebab Masalah
Secara default, layanan WebClient di Windows 10/11 mengunci autentikasi Basic Authentication hanya untuk koneksi terenkripsi HTTPS (`BasicAuthLevel = 1`). Saat Anda menghubungkan WebDAV melalui jaringan lokal (LAN) atau Tailscale menggunakan protokol HTTP biasa (`http://`), Windows menolaknya sebelum paket dikirim ke GoNAS.

### ✅ Cara Mengatasinya

#### Cara Cepat (Jalankan di PowerShell Administrator):
Buka PowerShell sebagai Administrator dan tempelkan perintah berikut:
```powershell
Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\WebClient\Parameters" -Name "BasicAuthLevel" -Value 2 -Type DWord; Restart-Service -Name WebClient -Force
```

#### Cara Manual (Melalui Registry Editor):
1. Tekan tombol `Windows + R`, ketik `regedit`, lalu tekan **Enter**.
2. Arahkan ke folder:
   ```
   HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\WebClient\Parameters
   ```
3. Cari atau buat kunci `DWORD (32-bit) Value` bernama **`BasicAuthLevel`**.
4. Ubah nilainya menjadi **`2`**.
   - `0` = Basic Auth dimatikan.
   - `1` = Basic Auth hanya untuk SSL/HTTPS (standar bawaan Windows).
   - `2` = Basic Auth diizinkan untuk HTTP biasa dan SSL (diperlukan untuk LAN/Tailscale).
5. Buka Command Prompt (Administrator) dan restart layanannya:
   ```cmd
   net stop WebClient
   net start WebClient
   ```
6. Buka kembali File Explorer, lalu coba Map Network Drive kembali ke `http://<IP>:8082/webdav`.

---

## 3. Port 8082 Sudah Dipakai (`bind: address already in use`)

### 🔍 Gejala yang Terlihat
Saat menjalankan GoNAS, terminal mengeluarkan pesan kesalahan:
```text
listen tcp :8082: bind: address already in use
```

### 💡 Penyebab Masalah
Ada proses aplikasi lain (atau instans GoNAS sebelumnya) yang sedang berjalan dan menggunakan port TCP `8082`.

### ✅ Cara Mengatasinya

#### Pilihan A: Mengubah Port GoNAS
Anda dapat menjalankan GoNAS pada port lain menggunakan parameter `-port`:
```bash
./gonas -port 8085
```

#### Pilihan B: Mematikan Proses yang Sedang Berjalan
- **Di Linux:**
  ```bash
  # Cari nomor PID yang memakai port 8082
  sudo lsof -i :8082 -sTCP:LISTEN
  # Atau:
  sudo ss -tulpn | grep 8082
  # Hentikan proses tersebut
  sudo kill -9 <NOMOR_PID>
  ```
- **Di Windows (PowerShell Administrator):**
  ```powershell
  # Cari dan matikan proses yang menggunakan port 8082
  Get-Process -Id (Get-NetTCPConnection -LocalPort 8082).OwningProcess | Stop-Process -Force
  ```

---

## 4. Tidak Bisa Diakses dari Perangkat Lain / HP / Tailscale / Jaringan Lokal

### 🔍 Gejala yang Terlihat
- GoNAS bisa dibuka di komputer server (`http://localhost:8082`), namun saat dibuka dari HP atau laptop lain muncul:
  > *"This site can't be reached"* atau *"Koneksi terputus (ERR_CONNECTION_TIMED_OUT)"*.

### 💡 Penyebab Masalah
Firewall pada sistem operasi memblokir koneksi masuk (*inbound traffic*) pada port TCP `8082`.

### ✅ Cara Mengatasinya

#### Di Linux (Ubuntu / Debian / Armbian):
```bash
sudo ufw allow 8082/tcp
sudo ufw reload
```

#### Di Linux (RHEL / CentOS / AlmaLinux):
```bash
sudo firewall-cmd --permanent --add-port=8082/tcp
sudo firewall-cmd --reload
```

#### Di Windows (PowerShell Administrator):
```powershell
New-NetFirewallRule -DisplayName "GoNAS Server Port 8082" -Direction Inbound -LocalPort 8082 -Protocol TCP -Action Allow -Profile Any
```

#### Penggunaan Lewat Tailscale:
Jika Anda terhubung melalui jaringan Tailscale:
1. Pastikan aplikasi Tailscale aktif di kedua perangkat (server dan HP/klien).
2. Gunakan alamat **IP Tailscale 100.x.y.z**:
   `http://100.x.y.z:8082`

---

## 5. Lupa Password Admin / Akun Terkunci

### 🔍 Gejala yang Terlihat
- Lupa kata sandi administrator atau akun terkunci akibat salah memasukkan kata sandi berturut-turut.

### ✅ Cara Mengatasinya
GoNAS menyimpan data otentikasi di file `users.json` pada direktori data.

1. Hentikan layanan GoNAS terlebih dahulu:
   - Linux: `sudo systemctl stop gonas`
   - Windows: `Stop-Process -Name gonas -Force`
2. Buka file `users.json` menggunakan text editor (contoh: `nano users.json` atau Notepad).
3. Ganti bagian nilai `password` untuk akun `admin` dengan hash default berikut (kata sandi: `admin123`):
   ```json
   "password": "sha256$39949ccaee3a99696fad82e3085dbbf4$5cc2646eeb2f278131d3b15d92179984392c9de3d62d028fd3e009b61429dba5"
   ```
4. Simpan file tersebut lalu jalankan kembali GoNAS:
   - Linux: `sudo systemctl start gonas`
   - Windows: Jalankan kembali aplikasi atau Scheduled Task.
5. Masuk kembali menggunakan akun: **`admin`** dan password: **`admin123`**.

---

## 6. Gagal Mengunggah File Berukuran Besar (> 1 GB)

### 🔍 Penyebab & Solusi
- **Koneksi Langsung ke GoNAS:** GoNAS tidak memiliki batasan ukuran file buatan dalam kode biner aslinya (mendukung unggahan multi-gigabyte melalui streaming).
- **Jika Menggunakan Reverse Proxy (Nginx, Caddy, Apache):**
  Pastikan konfigurasi reverse proxy tidak membatasi ukuran request body:
  - **Nginx:**
    ```nginx
    client_max_body_size 0; # Menghapus limit upload
    proxy_read_timeout 600s;
    proxy_send_timeout 600s;
    ```
  - **Cloudflare (Paket Gratis):**
    Cloudflare memiliki limit bawaan 100 MB per request POST. Jika ingin mengunggah file di atas 100 MB, hubungkan klien melalui IP lokal atau Tailscale, atau nonaktifkan proxy oranye Cloudflare (DNS-only).

---

## 7. Perintah Cek Log & Pengelolaan Layanan

### Di Linux (Systemd):
```bash
# Cek status layanan
sudo systemctl status gonas

# Membaca log langsung secara realtime
sudo journalctl -u gonas -f -n 100

# Restart layanan
sudo systemctl restart gonas
```

### Di Windows (PowerShell):
```powershell
# Cek apakah proses gonas aktif
Get-Process -Name gonas

# Restart Scheduled Task GoNAS
Restart-ScheduledTask -TaskName "GoNAS_Server"
```

---

## 💬 Kendala Lain?
Jika kendala Anda belum tercantum di halaman ini, silakan buat tiket laporan kendala di:
👉 [https://github.com/Arya-Penangsang/gonas/issues](https://github.com/Arya-Penangsang/gonas/issues)
