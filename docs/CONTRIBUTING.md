# 🤝 Panduan Kontribusi (Contributing Guide)

Terima kasih atas minat Anda untuk berkontribusi pada pengembangan **GoNAS**! Proyek ini terbuka untuk siapa saja yang ingin membantu menyempurnakan performa, keamanan, antarmuka, maupun dokumentasi.

---

## 🛠️ Prasyarat Pengembangan Lokal

Untuk membangun dan menguji GoNAS di komputer lokal, Anda membutuhkan:
1. **Go (Golang)**: Versi `1.22` atau lebih baru.
2. **Node.js**: Versi `18+` (hanya jika Anda memodifikasi tampilan frontend atau mengompilasi CSS minified).
3. **Git**

---

## 🚀 Alur Kerja Pengembangan

### 1. Fork & Clone Repositori
```bash
git clone https://github.com/<username-anda>/gonas.git
cd gonas
```

### 2. Menjalankan Server di Lingkungan Lokal
Jalankan GoNAS langsung menggunakan Go run:
```bash
go run .
```
Secara default, server akan mendengarkan di `http://localhost:8082`.

### 3. Memodifikasi Antarmuka & Rebuild CSS
Jika Anda menambahkan elemen HTML baru dengan class Tailwind baru di `web/index.html`, kompilasi ulang CSS minified:
```bash
npx --yes tailwindcss@3.4.17 -c tailwind.config.js -i ./web/input.css -o ./web/style.min.css --minify
```

### 4. Menguji Biner Bawaan
Kompilasi biner lokal untuk memastikan tidak ada kesalahan kompilasi:
```bash
# Windows
go build -o gonas.exe .

# Linux / macOS
go build -o gonas .
```

---

## 📋 Konvensi Kode
- Pastikan kode Anda rapi dan telah diformat menggunakan `gofmt` atau `go fmt ./...`.
- Jalankan verifikasi statis dengan `go vet ./...`.
- Jangan pernah meng-commit file database lokal, log, atau kredensial (`users.json`, `config.json`, `.env`).

---

## 📬 Mengirimkan Pull Request (PR)
1. Buat branch baru untuk fitur Anda: `git checkout -b feature/nama-fitur-anda`
2. Commit perubahan dengan pesan commit yang jelas mengikuti konvensi Conventional Commits (misal: `feat: ...`, `fix: ...`, `docs: ...`, `perf: ...`).
3. Push branch ke fork Anda: `git push origin feature/nama-fitur-anda`
4. Buka Pull Request di repositori utama dan isi template yang disediakan.
