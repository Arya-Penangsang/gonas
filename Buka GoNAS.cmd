@echo off
title GoNAS - Cloud NAS & Web File Manager
cd /d "%~dp0"

echo ==================================================================
echo    GoNAS - Ultra-Lightweight Self-Hosted Cloud NAS
echo    Repository: https://github.com/Arya-Penangsang/gonas
echo ==================================================================
echo.
echo Memulai server GoNAS...
echo Web Dashboard  : http://localhost:8082
echo WebDAV Server  : http://localhost:8082/webdav
echo Lokasi Storage : %~dp0storage
echo.
echo Akun Login Default:
echo   - superadmin : admin123  (Super Administrator)
echo   - admin      : admin123  (Administrator)
echo   - staf       : staf123   (Pengguna Staf)
echo   - tamu       : tamu123   (Viewer / Tamu)
echo.
echo CATATAN: JANGAN TUTUP JENDELA INI SELAMA GoNAS DIGUNAKAN.
echo Tekan [Ctrl+C] di jendela ini untuk menghentikan server.
echo ==================================================================
echo.

start "" http://localhost:8082

gonas.exe -port 8082 -root "%~dp0storage" -title "GoNAS Cloud Server"
pause
