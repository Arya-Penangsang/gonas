# ==============================================================================
# GoNAS Universal Automated Installer for Windows (PowerShell)
# Repository: https://github.com/Arya-Penangsang/gonas
# ==============================================================================
# Usage (Run in PowerShell as Administrator):
#   irm https://raw.githubusercontent.com/Arya-Penangsang/gonas/main/install.ps1 | iex
# ==============================================================================

[CmdletBinding()]
param(
    [string]$InstallPath = "C:\GoNAS",
    [int]$Port = 8082,
    [string]$ReleaseTag = ""
)

$ErrorActionPreference = "Stop"

Write-Host "================================================================" -ForegroundColor Cyan
Write-Host "  🚀 GoNAS Universal Automated Windows Installer                " -ForegroundColor Cyan
Write-Host "  Lightweight Private Cloud NAS & File Manager in Pure Go       " -ForegroundColor Cyan
Write-Host "================================================================" -ForegroundColor Cyan

# 1. Administrator Check
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[WARN] Skrip ini tidak dijalankan sebagai Administrator." -ForegroundColor Yellow
    Write-Host "Beberapa fitur otomatis (Firewall, Registry WebClient, Task Scheduler) memerlukan elevasi UAC." -ForegroundColor Yellow
    Write-Host "Jika instalasi gagal, silakan buka PowerShell via 'Run as Administrator'." -ForegroundColor Yellow
    if ($InstallPath -eq "C:\GoNAS") {
        $InstallPath = Join-Path $env:USERPROFILE "GoNAS"
        Write-Host "[INFO] Mengalihkan direktori instalasi ke: $InstallPath" -ForegroundColor Cyan
    }
}

$repoOwner = "Arya-Penangsang"
$repoName = "gonas"
$storagePath = Join-Path $InstallPath "storage"

# 2. Determine Architecture
$arch = $env:PROCESSOR_ARCHITECTURE
$goArch = "amd64"
if ($arch -eq "ARM64") {
    $goArch = "arm64"
} elseif ($arch -eq "x86") {
    $goArch = "386"
}

Write-Host "[INFO] Arsitektur Sistem: $arch (Target GoNAS: $goArch)" -ForegroundColor Blue

# 3. Detect Latest Release
if (-not $ReleaseTag) {
    Write-Host "[INFO] Memeriksa versi rilis terbaru dari GitHub..." -ForegroundColor Blue
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        $apiUrl = "https://api.github.com/repos/$repoOwner/$repoName/releases"
        $releases = Invoke-RestMethod -Uri $apiUrl -Method Get -Headers @{ "User-Agent" = "GoNAS-Installer" }
        if ($releases.Count -gt 0) {
            $ReleaseTag = $releases[0].tag_name
            Write-Host "[OK] Versi rilis terbaru ditemukan: $ReleaseTag" -ForegroundColor Green
        }
    } catch {
        $ReleaseTag = "v2.5.0-beta.1"
        Write-Host "[WARN] Gagal membaca API GitHub, menggunakan versi cadangan: $ReleaseTag" -ForegroundColor Yellow
    }
}

# 4. Prepare Destination Directories
if (-not (Test-Path $InstallPath)) {
    New-Item -ItemType Directory -Path $InstallPath -Force | Out-Null
}
if (-not (Test-Path $storagePath)) {
    New-Item -ItemType Directory -Path $storagePath -Force | Out-Null
}

# 5. Download and Extract Binary
$zipName = "gonas-$ReleaseTag-windows-$goArch.zip"
$zipUrl = "https://github.com/$repoOwner/$repoName/releases/download/$ReleaseTag/$zipName"
$tempZip = Join-Path $env:TEMP $zipName
$exePath = Join-Path $InstallPath "gonas.exe"

Write-Host "[INFO] Mengunduh biner GoNAS dari: $zipUrl" -ForegroundColor Blue
try {
    Invoke-WebRequest -Uri $zipUrl -OutFile $tempZip -UseBasicParsing
    Write-Host "[OK] Berhasil mengunduh $zipName." -ForegroundColor Green
    
    # Stop running process if already present to avoid file lock
    $existingProc = Get-Process -Name "gonas" -ErrorAction SilentlyContinue
    if ($existingProc) {
        Write-Host "[INFO] Menghentikan proses gonas yang sedang berjalan..." -ForegroundColor Yellow
        Stop-Process -Name "gonas" -Force -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 1
    }

    # Extract zip archive
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::ExtractToDirectory($tempZip, $InstallPath, $true)
    Remove-Item -Path $tempZip -Force -ErrorAction SilentlyContinue
    Write-Host "[OK] Biner gonas.exe berhasil diekstrak ke: $InstallPath" -ForegroundColor Green
} catch {
    Write-Host "[WARN] Gagal mengunduh arsip zip: $_" -ForegroundColor Yellow
    Write-Host "[INFO] Mencoba mengunduh file biner exe langsung..." -ForegroundColor Cyan
    $directExeUrl = "https://github.com/$repoOwner/$repoName/releases/download/$ReleaseTag/gonas.exe"
    try {
        Invoke-WebRequest -Uri $directExeUrl -OutFile $exePath -UseBasicParsing
        Write-Host "[OK] Biner gonas.exe berhasil diunduh langsung." -ForegroundColor Green
    } catch {
        Write-Error "Gagal mengunduh biner GoNAS: $_"
        exit 1
    }
}

# 6. Seed Default users.json
$usersJsonPath = Join-Path $InstallPath "users.json"
if (-not (Test-Path $usersJsonPath)) {
    $usersContent = @'
[
  {
    "username": "admin",
    "password": "sha256$39949ccaee3a99696fad82e3085dbbf4$5cc2646eeb2f278131d3b15d92179984392c9de3d62d028fd3e009b61429dba5",
    "role": "admin",
    "display_name": "Administrator",
    "created_at": "2026-11-09T08:00:00Z"
  },
  {
    "username": "staf",
    "password": "sha256$809ea14d840bb3dfc9ea01ab0ae53276$206165a58498a2480b64727b86753269d5b6a82544f17a7704b5a88021e3af9a",
    "role": "user",
    "display_name": "Staf Pengguna",
    "created_at": "2026-11-09T08:00:00Z"
  }
]
'@
    Set-Content -Path $usersJsonPath -Value $usersContent -Encoding UTF8
    Write-Host "[OK] Konfigurasi pengguna dibuat: admin (admin123), staf (staf123)." -ForegroundColor Green
}

# 7. Configure Windows WebClient Registry (Fixes Windows Explorer BasicAuth over HTTP)
if ($isAdmin) {
    try {
        $regPath = "HKLM:\SYSTEM\CurrentControlSet\Services\WebClient\Parameters"
        if (Test-Path $regPath) {
            Set-ItemProperty -Path $regPath -Name "BasicAuthLevel" -Value 2 -Type DWord -Force
            Restart-Service -Name "WebClient" -Force -ErrorAction SilentlyContinue
            Write-Host "[OK] Registry WebClient Windows dikonfigurasi (BasicAuthLevel = 2)." -ForegroundColor Green
        }
    } catch {
        Write-Host "[WARN] Melewati konfigurasi registry WebClient: $_" -ForegroundColor Yellow
    }

    # 8. Add Windows Firewall Inbound Rule
    try {
        $existingRule = Get-NetFirewallRule -DisplayName "GoNAS Cloud Server" -ErrorAction SilentlyContinue
        if (-not $existingRule) {
            New-NetFirewallRule -DisplayName "GoNAS Cloud Server" -Direction Inbound -LocalPort $Port -Protocol TCP -Action Allow -Profile Any | Out-Null
            Write-Host "[OK] Aturan Windows Firewall port $Port dibuka untuk GoNAS." -ForegroundColor Green
        }
    } catch {
        Write-Host "[WARN] Melewati pengaturan firewall: $_" -ForegroundColor Yellow
    }

    # 9. Register Windows Scheduled Task for Auto-Start on Boot
    try {
        $taskName = "GoNAS_Server"
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction SilentlyContinue | Out-Null
        
        $action = New-ScheduledTaskAction -Execute $exePath -Argument "-port $Port -root `"$storagePath`" -title `"GoNAS Cloud Server`"" -WorkingDirectory $InstallPath
        $trigger = New-ScheduledTaskTrigger -AtStartup
        $principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
        $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit 0
        
        Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Principal $principal -Settings $settings | Out-Null
        Start-ScheduledTask -TaskName $taskName
        Write-Host "[OK] Layanan GoNAS didaftarkan ke Windows Task Scheduler ($taskName) & telah dijalankan!" -ForegroundColor Green
    } catch {
        Write-Host "[WARN] Gagal mendaftarkan task scheduler: $_. Memulai langsung di background..." -ForegroundColor Yellow
        Start-Process -FilePath $exePath -ArgumentList "-port $Port -root `"$storagePath`" -title `"GoNAS Cloud Server`"" -WorkingDirectory $InstallPath
    }
} else {
    # Non-admin startup
    Write-Host "[INFO] Menjalankan gonas.exe di latar belakang..." -ForegroundColor Cyan
    Start-Process -FilePath $exePath -ArgumentList "-port $Port -root `"$storagePath`" -title `"GoNAS Cloud Server`"" -WorkingDirectory $InstallPath
}

# 10. Summary & Output
$localIP = (Get-NetIPAddress -AddressFamily IPv4 -InterfaceAlias * -ErrorAction SilentlyContinue | Where-Object { $_.IPAddress -notlike "127.*" -and $_.IPAddress -notlike "169.254.*" } | Select-Object -ExpandProperty IPAddress -First 1)
if (-not $localIP) { $localIP = "127.0.0.1" }

Write-Host ""
Write-Host "================================================================" -ForegroundColor Green
Write-Host "  🎉 INSTALASI GONAS WINDOWS SELESAI DENGAN SUKSES!             " -ForegroundColor Green
Write-Host "================================================================" -ForegroundColor Green
Write-Host "Akses Web Dashboard GoNAS (Buka di Chrome / Edge):"
Write-Host "  👉 URL Lokal   : http://localhost:$Port" -ForegroundColor Cyan
Write-Host "  👉 URL Jaringan: http://${localIP}:$Port" -ForegroundColor Cyan
Write-Host ""
Write-Host "Koneksi WebDAV (Windows Explorer -> Map Network Drive):"
Write-Host "  👉 Folder URL  : http://${localIP}:${Port}/webdav" -ForegroundColor Yellow
Write-Host "  ⚠️  PENTING: Jangan buka URL /webdav di browser! URL tersebut" -ForegroundColor Yellow
Write-Host "     khusus untuk File Explorer / Map Network Drive." -ForegroundColor Yellow
Write-Host ""
Write-Host "Kredensial Default:"
Write-Host "  👤 Admin: admin / admin123" -ForegroundColor Green
Write-Host "  👤 Staf : staf  / staf123" -ForegroundColor Green
Write-Host ""
Write-Host "Lokasi Penyimpanan & Data:"
Write-Host "  📁 Program & Config : $InstallPath" -ForegroundColor Gray
Write-Host "  📁 File Storage     : $storagePath" -ForegroundColor Gray
Write-Host "================================================================" -ForegroundColor Green
