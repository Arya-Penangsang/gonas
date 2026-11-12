@echo off
title GoNAS WebDAV Windows Auto-Mounter
color 0b

:: 1. Self-elevation to Administrator
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo ===================================================================
    echo   Meminta Izin Administrator (UAC) untuk mengaktifkan WebClient...
    echo ===================================================================
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process cmd -ArgumentList '/c \"\"%~f0\"\"' -Verb RunAs"
    exit /b
)

echo ===================================================================
echo   GoNAS WebDAV - Windows Service ^& Drive Mounter
echo ===================================================================
echo.

:: 2. Enable & Start WebClient Service
echo [1/4] Mengaktifkan layanan Windows WebClient (WebDAV Engine)...
sc config WebClient start= auto >nul 2>&1
net start WebClient >nul 2>&1

:: 3. Configure Windows Registry for HTTP Basic Auth & File Size Limit (4GB)
echo [2/4] Mengatur Registry WebClient (BasicAuthLevel=2 ^& FileLimit 4GB)...
reg add "HKLM\SYSTEM\CurrentControlSet\Services\WebClient\Parameters" /v BasicAuthLevel /t REG_DWORD /d 2 /f >nul
reg add "HKLM\SYSTEM\CurrentControlSet\Services\WebClient\Parameters" /v FileSizeLimitInBytes /t REG_DWORD /d 4294967295 /f >nul

:: Restart WebClient service to apply registry changes
echo [3/4] Menerapkan konfigurasi WebClient...
net stop WebClient >nul 2>&1
net start WebClient >nul 2>&1

:: 4. Map Network Drive to Z:
echo [4/4] Memetakan WebDAV ke Drive Jaringan Z:...
net use Z: /delete /y >nul 2>&1
net use Z: "http://127.0.0.1:8082/webdav" /user:admin admin123 /persistent:yes >nul 2>&1

if %errorLevel% equ 0 (
    goto :success
)

:: Fallback format (DavWWWRoot)
net use Z: "\\127.0.0.1@8082\webdav" /user:admin admin123 /persistent:yes >nul 2>&1
if %errorLevel% equ 0 (
    goto :success
)

echo.
echo [PERINGATAN] Gagal memetakan langsung via baris perintah.
echo Silakan buka File Explorer -> This PC -> titik tiga (...) -> Map network drive:
echo   Folder : http://127.0.0.1:8082/webdav/
echo   User   : admin
echo   Pass   : admin123
echo Layanan WebClient dan Registry Windows sudah AKTIF dan SIAP digunakan.
goto :end

:success
echo.
echo ===================================================================
echo   SUKSES! Drive Z: (GoNAS Cloud Storage) Berhasil Terhubung!
echo   Membuka Drive Z: di Windows File Explorer...
echo ===================================================================
start explorer Z:

:end
echo.
timeout /t 5
