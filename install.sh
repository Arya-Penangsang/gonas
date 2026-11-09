#!/usr/bin/env bash
# ==============================================================================
# GoNAS Universal Automated Installer (Linux / Armbian / Raspberry Pi / macOS)
# https://github.com/Arya-Penangsang/gonas
# ==============================================================================

set -euo pipefail

APP_NAME="gonas"
REPO_OWNER="Arya-Penangsang"
REPO_NAME="gonas"
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/nas"
STORAGE_DIR="/var/nas/storage"
DEFAULT_PORT="8082"
SERVICE_NAME="gonas"

# Color outputs
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${CYAN}================================================================${NC}"
echo -e "${CYAN}  🚀 GoNAS Universal Automated Installer                         ${NC}"
echo -e "${CYAN}  Lightweight Private Cloud NAS & File Manager in Pure Go        ${NC}"
echo -e "${CYAN}================================================================${NC}"

# 1. Root / Sudo Check
if [ "$(id -u)" -ne 0 ]; then
    echo -e "${RED}[ERROR] Skrip ini membutuhkan izin root/sudo.${NC}"
    echo -e "Silakan jalankan kembali dengan: ${YELLOW}sudo bash install.sh${NC}"
    exit 1
fi

# 2. Detect OS and Architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64|amd64)
        GO_ARCH="amd64"
        ;;
    aarch64|arm64)
        GO_ARCH="arm64"
        ;;
    armv7l|armhf)
        GO_ARCH="arm-armv7"
        ;;
    *)
        echo -e "${RED}[ERROR] Arsitektur CPU '$ARCH' belum didukung otomatis.${NC}"
        exit 1
        ;;
esac

case "$OS" in
    linux)
        PLATFORM="linux"
        ;;
    darwin)
        PLATFORM="darwin"
        ;;
    *)
        echo -e "${RED}[ERROR] Sistem operasi '$OS' belum didukung oleh skrip bash ini.${NC}"
        echo -e "Untuk Windows, silakan gunakan: ${YELLOW}irm https://raw.githubusercontent.com/$REPO_OWNER/$REPO_NAME/main/install.ps1 | iex${NC}"
        exit 1
        ;;
esac

echo -e "${BLUE}[INFO] Terdeteksi Platform:${NC} ${GREEN}$PLATFORM ($GO_ARCH)${NC}"

# 3. Detect latest release from GitHub API
echo -e "${BLUE}[INFO] Mengambil informasi rilis terbaru dari GitHub...${NC}"
RELEASE_API="https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases"
LATEST_TAG=$(curl -sSL "$RELEASE_API" | grep -m1 '"tag_name":' | cut -d'"' -f4 || echo "")

if [ -z "$LATEST_TAG" ]; then
    LATEST_TAG="v2.5.0-beta.1"
    echo -e "${YELLOW}[WARN] Gagal mendapatkan tag rilis terbaru via API, menggunakan fallback: $LATEST_TAG${NC}"
else
    echo -e "${GREEN}[OK] Versi rilis ditemukan: $LATEST_TAG${NC}"
fi

TAR_NAME="gonas-${LATEST_TAG}-${PLATFORM}-${GO_ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/$REPO_OWNER/$REPO_NAME/releases/download/${LATEST_TAG}/${TAR_NAME}"

# 4. Download and Extract Binary
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo -e "${BLUE}[INFO] Mengunduh biner GoNAS (${TAR_NAME})...${NC}"
if curl -sSL -f "$DOWNLOAD_URL" -o "$TMP_DIR/$TAR_NAME"; then
    echo -e "${GREEN}[OK] Berhasil mengunduh arsip.${NC}"
    tar -xzf "$TMP_DIR/$TAR_NAME" -C "$TMP_DIR"
elif curl -sSL -f "https://github.com/$REPO_OWNER/$REPO_NAME/releases/download/${LATEST_TAG}/gonas" -o "$TMP_DIR/gonas"; then
    echo -e "${GREEN}[OK] Berhasil mengunduh biner standalone.${NC}"
else
    echo -e "${RED}[ERROR] Gagal mengunduh biner dari: $DOWNLOAD_URL${NC}"
    exit 1
fi

chmod +x "$TMP_DIR/gonas"
echo -e "${BLUE}[INFO] Memasang biner ke $INSTALL_DIR/gonas...${NC}"
install -m 755 "$TMP_DIR/gonas" "$INSTALL_DIR/gonas"

# 5. Setup Storage & Configuration Directories
echo -e "${BLUE}[INFO] Menyiapkan direktori penyimpanan di $DATA_DIR...${NC}"
mkdir -p "$STORAGE_DIR" "$DATA_DIR"

if [ ! -f "$DATA_DIR/users.json" ]; then
    cat <<EOF > "$DATA_DIR/users.json"
[
  {
    "username": "admin",
    "password": "sha256\$39949ccaee3a99696fad82e3085dbbf4\$5cc2646eeb2f278131d3b15d92179984392c9de3d62d028fd3e009b61429dba5",
    "role": "admin",
    "display_name": "Administrator",
    "created_at": "$(date -Iseconds)"
  },
  {
    "username": "staf",
    "password": "sha256\$809ea14d840bb3dfc9ea01ab0ae53276\$206165a58498a2480b64727b86753269d5b6a82544f17a7704b5a88021e3af9a",
    "role": "user",
    "display_name": "Staf Pengguna",
    "created_at": "$(date -Iseconds)"
  }
]
EOF
    echo -e "${GREEN}[OK] Kredensial default dibuat (admin: admin123, staf: staf123).${NC}"
fi

# 6. Setup Systemd Service (Linux only)
if [ "$PLATFORM" = "linux" ] && command -v systemctl >/dev/null 2>&1; then
    echo -e "${BLUE}[INFO] Mendaftarkan systemd service: /etc/systemd/system/gonas.service...${NC}"
    cat <<EOF > /etc/systemd/system/gonas.service
[Unit]
Description=GoNAS - Ultra-Lightweight Cloud NAS & Web File Manager
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=$DATA_DIR
ExecStart=$INSTALL_DIR/gonas -port $DEFAULT_PORT -root $STORAGE_DIR -title "GoNAS Cloud Server"
Restart=always
RestartSec=3s
LimitNOFILE=65535
MemoryHigh=250M
MemoryMax=500M

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable --now gonas.service
    echo -e "${GREEN}[OK] Layanan systemd gonas berhasil diaktifkan dan dijalankan!${NC}"
else
    echo -e "${YELLOW}[NOTE] Menjalankan GoNAS di latar belakang tanpa systemd...${NC}"
    nohup "$INSTALL_DIR/gonas" -port "$DEFAULT_PORT" -root "$STORAGE_DIR" -title "GoNAS Cloud Server" > "$DATA_DIR/gonas.log" 2>&1 &
fi

# 7. Summary & Information
SERVER_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "localhost")

echo -e "\n${GREEN}================================================================${NC}"
echo -e "${GREEN}  🎉 INSTALASI GONAS SELESAI DENGAN SUKSES!                     ${NC}"
echo -e "${GREEN}================================================================${NC}"
echo -e "Akses Web Dashboard GoNAS:"
echo -e "  👉 URL Lokal   : ${CYAN}http://localhost:${DEFAULT_PORT}${NC}"
echo -e "  👉 URL Jaringan: ${CYAN}http://${SERVER_IP}:${DEFAULT_PORT}${NC}"
echo -e ""
echo -e "Kredensial Login Default:"
echo -e "  👤 Admin: ${YELLOW}admin${NC} / ${YELLOW}admin123${NC}"
echo -e "  👤 Staf : ${YELLOW}staf${NC}  / ${YELLOW}staf123${NC}"
echo -e ""
echo -e "Folder Penyimpanan:"
echo -e "  📁 Root Storage: ${BLUE}${STORAGE_DIR}${NC}"
echo -e "  ⚙️ Konfigurasi : ${BLUE}${DATA_DIR}/users.json${NC}"
echo -e ""
echo -e "Perintah Pengelolaan Layanan:"
echo -e "  Cek status : ${CYAN}sudo systemctl status gonas${NC}"
echo -e "  Restart    : ${CYAN}sudo systemctl restart gonas${NC}"
echo -e "  Lihat log  : ${CYAN}sudo journalctl -u gonas -f${NC}"
echo -e "${GREEN}================================================================${NC}\n"
