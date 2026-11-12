@echo off
title Hentikan GoNAS Server
echo Menghentikan proses GoNAS...
taskkill /F /IM gonas.exe >nul 2>&1
echo GoNAS berhasil dihentikan.
timeout /t 3
