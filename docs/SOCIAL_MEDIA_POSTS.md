# 📱 Draf Postingan Media Sosial: GoNAS

Kumpulan draf teks postingan untuk Threads dan X (Twitter) dengan gaya bahasa natural, autentik, dan bercerita (*storytelling*) berbasis pengalaman nyata migrasi storage server.

---

## 🧵 Versi Meta Threads (Storytelling Santai)

### Pilihan 1: Cerita Pengalaman Migrasi & Ngoding Sendiri (Rekomendasi Utama)
```text
Lagi iseng nyoba-nyoba bikin project kecil-kecilan nih.

Ceritanya berawal dari rasa kesel sama storage server di kantor/rumah. Kemarin-kemarin sempat pakai FileBrowser, tapi tampilannya kaku banget dan autentikasinya sering rewel—database-nya suka nge-lock sendiri. Mau beralih ke Nextcloud atau solusi NAS lain, eh pas dicek makan RAM-nya gila-gilaan, belum lagi harus setup Docker, database, dan printilan lainnya yang bikin PC server ngos-ngosan.

Karena butuhnya sebenarnya simpel (cuma pengen simpan-buka file pribadi tanpa ribet), akhirnya saya putusin buat ngoding sendiri pakai Go.

Target awalnya cuma dua:
1. Harus satu file binary doang (single binary). Tinggal dobel klik atau jalanin langsung, nggak usah install runtime atau database apa-apa.
2. RAM-nya harus super irit tapi tampilannya tetap enak dilihat.

Pas udah jadi dan dites di server, ternyata konsumsi RAM-nya cuma sekitar 6–7 MB doang pas standby. Kemarin saya coba cross-compile buat dipasang di Raspberry Pi juga langsung jalan mulus tanpa beban. Tampilannya udah ada dark mode, bisa langsung preview PDF, dan bisa muter streaming video langsung dari browser.

Project-nya saya namain GoNAS dan sekarang udah saya rapikan serta open-source-kan di GitHub, siapa tahu teman-teman di sini ada yang lagi cari alternatif NAS ringan buat homelab atau server kantornya:

https://github.com/Arya-Penangsang/gonas

Barangkali ada yang mau coba atau punya masukan, feel free banget ya! Kira-kira fitur apa lagi ya yang enak ditambahin?
```

---

## 🐦 Versi X (Twitter)

### Pilihan A: Single Tweet Natural (Non-Sales)
```text
Berawal dari gregetan sama FileBrowser yang tampilannya kaku + sering rewel pas login, dan Nextcloud yang terlalu rakus RAM buat PC server kantor...

Akhirnya iseng ngoding private cloud sendiri pakai Go murni. 

Hasilnya lumayan puas: single binary tanpa dependensi apa-apa, konsumsi RAM cuma 6.4 MB, UI modern dark-mode, dan enteng banget pas dicoba di Raspberry Pi.

Iseng saya taruh di GitHub, barangkali ada yang butuh alternatif NAS super irit daya:
https://github.com/Arya-Penangsang/gonas
```

### Pilihan B: Mini-Thread X (2 Tweet)
**Tweet 1:**
```text
Lagi nyoba-nyoba bikin project NAS pribadi. Kemarin sempat bandingin beberapa server di lab: Nextcloud berat banget di RAM, sedangkan FileBrowser fiturnya nanggung dan sistem auth-nya suka ngunci sendiri.

Akhirnya bikin sendiri pakai Go: GoNAS. Targetnya simpel: zero dependency, satu file jalan, RAM <10MB, tapi UI-nya udah modern. 🧵👇
```

**Tweet 2:**
```text
Pas dites running di server kantor sama Raspberry Pi, idle-nya cuma kena 6.4 MB RAM doang dan CPU 0%. Udah ada built-in video player, preview PDF, sama drag-and-drop.

Kodenya udah saya buka di GitHub, feel free buat yang mau coba atau kasih kritik & saran:
https://github.com/Arya-Penangsang/gonas
```

---

## 💡 Tips Penerbitan
1. **Lampirkan Gambar:** Sertakan gambar `web/gonas-banner.png` atau tangkapan layar antarmuka GoNAS agar engagement & CTR maksimal.
2. **Waktu Unggah Terbaik:**
   - Siang: 11:30 - 13:00 WIB
   - Malam: 19:30 - 21:30 WIB
