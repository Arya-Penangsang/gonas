@echo off
title GoNAS WebDAV Windows Setup
color 0b

net session >nul 2>&1
if %errorlevel% neq 0 (
    echo ===================================================================
    echo   [PERHATIAN] FILE INI WAJIB DIJALANKAN SEBAGAI ADMINISTRATOR!
    echo ===================================================================
    echo.
    echo   Karena Windows memerlukan izin Admin untuk menyalakan WebClient,
    echo   silakan lakukan langkah berikut:
    echo.
    echo     1. TUTUP jendela hitam ini.
    echo     2. KLIK KANAN pada file ini di Desktop:
    echo        "Hubungkan WebDAV ke Drive Z (Admin).cmd"
    echo     3. Pilih menu "Run as administrator" (Jalankan sebagai administrator).
    echo     4. Klik "Yes" pada jendela konfirmasi Windows (UAC).
    echo.
    echo ===================================================================
    pause
    exit /b
)

echo ===================================================================
echo   GoNAS WebDAV - Menyalakan Layanan Windows & Memetakan Drive Z:
echo ===================================================================
echo.

echo [1/4] Mengatur Registry Windows agar mengizinkan WebDAV HTTP...
reg add "HKLM\SYSTEM\CurrentControlSet\Services\WebClient\Parameters" /v BasicAuthLevel /t REG_DWORD /d 2 /f
reg add "HKLM\SYSTEM\CurrentControlSet\Services\WebClient\Parameters" /v FileSizeLimitInBytes /t REG_DWORD /d 4294967295 /f
reg add "HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System" /v EnableLinkedConnections /t REG_DWORD /d 1 /f

echo.
echo [2/4] Mengaktifkan dan memulai layanan Windows WebClient...
sc config WebClient start= auto
net stop WebClient >nul 2>&1
net start WebClient

echo.
echo [3/4] Menghubungkan GoNAS WebDAV ke Drive Z:...
net use Z: /delete /y >nul 2>&1
timeout /t 2 >nul
net use Z: "http://172.16.63.151:8082/webdav" /user:admin admin123 /persistent:yes

if %errorlevel% neq 0 (
    echo.
    echo Mencoba format DavWWWRoot...
    net use Z: "\\172.16.63.151@8082\webdav" /user:admin admin123 /persistent:yes
)

echo.
echo [4/4] Membuka Drive Z: di Windows File Explorer...
start explorer Z:

echo.
echo ===================================================================
echo   BERHASIL! Drive Z: (GoNAS Cloud WebDAV) sudah aktif dan terbuka!
echo ===================================================================
pause
