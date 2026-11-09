# 🛠️ GoNAS Troubleshooting & Error Handling Guide

This guide covers common issues, error messages, and solutions encountered when installing, running, or accessing **GoNAS**.

---

## 📑 Quick Navigation
1. [Web Browser Login Popup Loop or 405 Error on `/webdav/`](#1-web-browser-login-popup-loop-or-405-error-on-webdav)
2. [Windows Explorer Fails to Map WebDAV Network Drive](#2-windows-explorer-fails-to-map-webdav-network-drive)
3. [Port Already in Use (`bind: address already in use`)](#3-port-already-in-use-bind-address-already-in-use)
4. [Cannot Access from LAN, Mobile, or Tailscale / VPN](#4-cannot-access-from-lan-mobile-or-tailscale--vpn)
5. [Resetting Forgotten Admin Password / Locked Account](#5-resetting-forgotten-admin-password--locked-account)
6. [Large File Upload Fails or Timeouts](#6-large-file-upload-fails-or-timeouts)
7. [Service Management & Checking Logs](#7-service-management--checking-logs)

---

## 1. Web Browser Login Popup Loop or 405 Error on `/webdav/`

### 🔍 Symptoms
- When visiting `http://<IP>:8082/webdav/` in Google Chrome, Edge, or mobile browsers, a native browser pop-up modal appears:
  > *"Login: http://... requires a username and password. Your connection to this site is not private."*
- Entering credentials results in an endless pop-up loop or an `HTTP 405 Method Not Allowed` white page.

```
+-------------------------------------------------------------+
|  Login                                                      |
|  http://172.16.63.15:8082 requires a username and password. |
|  Username: [                 ]                              |
|  Password: [                 ]                              |
|                                     [ Cancel ] [ Login ]    |
+-------------------------------------------------------------+
```

### 💡 Root Cause
- The `/webdav/` endpoint is strictly an **RFC 4918 Network Storage Protocol mount point**. It is engineered for operating system file managers (such as Windows Explorer, macOS Finder, Cyberduck, or mobile file managers like Solid Explorer / Cx File Explorer).
- Browsers send `GET` requests expecting web pages, but WebDAV directory roots only respond to WebDAV commands (`PROPFIND`, `OPTIONS`, `MKCOL`).

### ✅ Solution
1. **To access the Web Dashboard UI**, open the root URL in your browser:
   👉 **`http://<IP>:8082`** (DO **NOT** append `/webdav/`).
2. Starting in **GoNAS v2.5.0+**, if you open `/webdav/` in a web browser, GoNAS will automatically detect the browser and present a friendly landing page with a direct one-click button to the Web Dashboard and instructions on how to mount the WebDAV drive.

---

## 2. Windows Explorer Fails to Map WebDAV Network Drive

### 🔍 Symptoms
- In Windows Explorer, you click **Map Network Drive**, enter `http://<IP>:8082/webdav`, and Windows displays:
  > *"The folder you entered does not appear to be valid. Please choose another."*
  or
  > *"Windows cannot access http://... Check the spelling of the name."*

### 💡 Root Cause
By default, Windows WebClient service blocks HTTP Basic Authentication unless SSL/TLS (`https://`) is used. When connecting over local LAN or Tailscale using plain HTTP (`http://`), Windows rejects the authentication request before sending it to GoNAS.

### ✅ Solution

#### Automatic Fix (PowerShell Administrator):
Run this single command in PowerShell as Administrator:
```powershell
Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\WebClient\Parameters" -Name "BasicAuthLevel" -Value 2 -Type DWord; Restart-Service -Name WebClient -Force
```

#### Manual Fix (Registry Editor):
1. Press `Win + R`, type `regedit`, and hit **Enter**.
2. Navigate to:
   ```
   HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\WebClient\Parameters
   ```
3. Locate or create a `DWORD (32-bit) Value` named **`BasicAuthLevel`**.
4. Set its value data to **`2`** (Hexadecimal/Decimal).
   - `0` = Basic Auth disabled.
   - `1` = Basic Auth enabled for SSL/HTTPS only (Windows default).
   - `2` = Basic Auth enabled for both SSL and plain HTTP (Required for LAN/Tailscale).
5. Open Command Prompt as Administrator and restart the service:
   ```cmd
   net stop WebClient
   net start WebClient
   ```
6. Return to Windows Explorer and map your drive again to `http://<IP>:8082/webdav`.

---

## 3. Port Already in Use (`bind: address already in use`)

### 🔍 Symptoms
When starting GoNAS, the terminal displays:
```text
listen tcp :8082: bind: address already in use
```

### 💡 Root Cause
Another application (or a previously running GoNAS instance) is already listening on TCP port `8082`.

### ✅ Solution

#### Option A: Change GoNAS Port
You can specify any alternative open port using the `-port` flag:
```bash
./gonas -port 8085
```

#### Option B: Terminate Conflicting Process
- **On Linux:**
  ```bash
  # Find PID using port 8082
  sudo lsof -i :8082 -sTCP:LISTEN
  # Or:
  sudo ss -tulpn | grep 8082
  # Terminate process
  sudo kill -9 <PID>
  ```
- **On Windows (PowerShell Administrator):**
  ```powershell
  # Find process using port 8082
  Get-Process -Id (Get-NetTCPConnection -LocalPort 8082).OwningProcess
  # Stop conflicting process
  Stop-Process -Id (Get-NetTCPConnection -LocalPort 8082).OwningProcess -Force
  ```

---

## 4. Cannot Access from LAN, Mobile, or Tailscale / VPN

### 🔍 Symptoms
- GoNAS works on `http://localhost:8082`, but accessing from another PC or smartphone gives:
  > *"This site can't be reached"* or *"ERR_CONNECTION_TIMED_OUT"*.

### 💡 Root Cause
The operating system firewall is blocking inbound traffic on TCP port `8082`.

### ✅ Solution

#### On Linux (Ubuntu / Debian / Armbian):
```bash
sudo ufw allow 8082/tcp
sudo ufw reload
```

#### On Linux (RHEL / CentOS / AlmaLinux):
```bash
sudo firewall-cmd --permanent --add-port=8082/tcp
sudo firewall-cmd --reload
```

#### On Windows (PowerShell Administrator):
```powershell
New-NetFirewallRule -DisplayName "GoNAS Server Port 8082" -Direction Inbound -LocalPort 8082 -Protocol TCP -Action Allow -Profile Any
```

#### Tailscale Access:
If connecting through Tailscale:
1. Ensure Tailscale is connected on both the GoNAS server and your client device (`tailscale status`).
2. Use the **Tailscale 100.x.y.z IP** or MagicDNS name:
   `http://100.x.y.z:8082`

---

## 5. Resetting Forgotten Admin Password / Locked Account

### 🔍 Symptoms
- You forgot the administrator password or were temporarily locked out due to multiple failed attempts.

### 💡 Solution
GoNAS stores user accounts in `users.json` located in its data directory.

1. Stop the running GoNAS service:
   - Linux: `sudo systemctl stop gonas`
   - Windows: `Stop-Process -Name gonas -Force`
2. Open `users.json` in a text editor (e.g., `nano /var/nas/users.json` or Notepad).
3. Replace the `password` field for the `admin` user with the default hash (`admin123`):
   ```json
   "password": "sha256$39949ccaee3a99696fad82e3085dbbf4$5cc2646eeb2f278131d3b15d92179984392c9de3d62d028fd3e009b61429dba5"
   ```
4. Save the file and restart GoNAS:
   - Linux: `sudo systemctl start gonas`
   - Windows: Start GoNAS executable or Task Scheduler task.
5. Log in with username **`admin`** and password **`admin123`**.

---

## 6. Large File Upload Fails or Timeouts

### 🔍 Symptoms
- Small files upload instantly, but files > 1 GB fail or stall halfway through.

### 💡 Root Cause & Solution
- **Direct GoNAS connection:** GoNAS has no hardcoded file size limits and supports streaming uploads.
- **Reverse Proxy (Nginx, Caddy, Apache):**
  If you are running GoNAS behind a reverse proxy, you must adjust the proxy's maximum body size limit:
  - **Nginx:**
    ```nginx
    client_max_body_size 0; # Disables file upload size restriction
    proxy_read_timeout 600s;
    proxy_send_timeout 600s;
    ```
  - **Cloudflare (Free Tier):**
    Cloudflare imposes a strict 100 MB limit on HTTP POST requests. To upload large files, access GoNAS directly via local LAN IP, Tailscale, or bypass Cloudflare proxy (DNS-only gray cloud).

---

## 7. Service Management & Checking Logs

### Linux (Systemd):
```bash
# Check real-time service status
sudo systemctl status gonas

# View live systemd logs
sudo journalctl -u gonas -f -n 100

# Restart service after config changes
sudo systemctl restart gonas
```

### Windows:
```powershell
# Check running GoNAS process
Get-Process -Name gonas

# Restart GoNAS Scheduled Task
Restart-ScheduledTask -TaskName "GoNAS_Server"
```

---

## 💬 Still Experiencing Issues?
If your problem is not covered in this guide, please open an issue with relevant logs at:
👉 [https://github.com/Arya-Penangsang/gonas/issues](https://github.com/Arya-Penangsang/gonas/issues)
