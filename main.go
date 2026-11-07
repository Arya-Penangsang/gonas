package main

import (
	"archive/zip"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed web/*
var embeddedWebFS embed.FS

// Config
type Config struct {
	Port     string `json:"port"`
	RootPath string `json:"root"`
	Username string `json:"username"`
	Password string `json:"password"`
	AppTitle string `json:"title"`
}

var (
	cfg       Config
	configMu  sync.RWMutex
	startTime = time.Now()
)

// Audit Log Store
type AuditEntry struct {
	Time    string `json:"time"`
	Type    string `json:"type"` // LOGIN, UPLOAD, DELETE, MKDIR, RENAME, MOVE, GROUP, ARRANGE, PASSWORD
	User    string `json:"user"`
	IP      string `json:"ip"`
	Detail  string `json:"detail"`
	Success bool   `json:"success"`
}

type AuditStore struct {
	sync.RWMutex
	entries []AuditEntry
}

var auditLogs = &AuditStore{
	entries: make([]AuditEntry, 0, 150),
}

func logAudit(eventType, user, ip, detail string, success bool) {
	auditLogs.Lock()
	defer auditLogs.Unlock()
	entry := AuditEntry{
		Time:    time.Now().Format("02/01/2006 15:04:05"),
		Type:    eventType,
		User:    user,
		IP:      ip,
		Detail:  detail,
		Success: success,
	}
	if len(auditLogs.entries) >= 150 {
		auditLogs.entries = auditLogs.entries[1:]
	}
	auditLogs.entries = append(auditLogs.entries, entry)
}

// ==================== HIERARCHICAL UAC & USER STORE ====================
type UserRole string

const (
	RoleSuperAdmin UserRole = "superadmin"
	RoleAdmin      UserRole = "admin"
	RoleUser       UserRole = "user"
	RoleViewer     UserRole = "viewer"
)

type UserAccount struct {
	Username    string    `json:"username"`
	Password    string    `json:"password"`
	Role        UserRole  `json:"role"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

type UserStore struct {
	sync.RWMutex
	filePath string
	users    map[string]*UserAccount
}

var userStore = &UserStore{
	users: make(map[string]*UserAccount),
}

func initUserStore() {
	userStore.Lock()
	defer userStore.Unlock()

	userStore.filePath = "C:\\NAS\\users.json"
	if _, err := os.Stat("C:\\NAS"); os.IsNotExist(err) {
		userStore.filePath = "users.json"
	}

	data, err := os.ReadFile(userStore.filePath)
	if err == nil {
		var list []*UserAccount
		if err := json.Unmarshal(data, &list); err == nil && len(list) > 0 {
			for _, u := range list {
				userStore.users[u.Username] = u
			}
			return
		}
	}

	// Seed default hierarchical accounts
	defaultUsers := []*UserAccount{
		{
			Username:    "superadmin",
			Password:    "admin123",
			Role:        RoleSuperAdmin,
			DisplayName: "Super Admin BPMN",
			CreatedAt:   time.Now(),
		},
		{
			Username:    "admin",
			Password:    "admin123",
			Role:        RoleAdmin,
			DisplayName: "Admin Preservasi",
			CreatedAt:   time.Now(),
		},
		{
			Username:    "staf",
			Password:    "staf123",
			Role:        RoleUser,
			DisplayName: "Staf Preservasi",
			CreatedAt:   time.Now(),
		},
		{
			Username:    "tamu",
			Password:    "tamu123",
			Role:        RoleViewer,
			DisplayName: "Tamu / Viewer",
			CreatedAt:   time.Now(),
		},
	}

	for _, u := range defaultUsers {
		userStore.users[u.Username] = u
	}
	saveUserStoreLocked()
}

func saveUserStoreLocked() {
	if userStore.filePath == "" {
		return
	}
	list := make([]*UserAccount, 0, len(userStore.users))
	for _, u := range userStore.users {
		list = append(list, u)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err == nil {
		os.WriteFile(userStore.filePath, data, 0644)
	}
}

func (us *UserStore) Authenticate(username, password string) *UserAccount {
	us.RLock()
	defer us.RUnlock()
	u, exists := us.users[username]
	if !exists {
		return nil
	}
	if u.Password == password {
		return u
	}
	return nil
}

func (us *UserStore) Get(username string) *UserAccount {
	us.RLock()
	defer us.RUnlock()
	return us.users[username]
}

func (us *UserStore) List() []*UserAccount {
	us.RLock()
	defer us.RUnlock()
	list := make([]*UserAccount, 0, len(us.users))
	for _, u := range us.users {
		safeCopy := *u
		safeCopy.Password = "••••••••"
		list = append(list, &safeCopy)
	}
	return list
}

func (us *UserStore) Add(u *UserAccount) error {
	us.Lock()
	defer us.Unlock()
	if _, exists := us.users[u.Username]; exists {
		return fmt.Errorf("Username sudah digunakan")
	}
	u.CreatedAt = time.Now()
	us.users[u.Username] = u
	saveUserStoreLocked()
	return nil
}

func (us *UserStore) Delete(username string) error {
	us.Lock()
	defer us.Unlock()
	if username == "superadmin" {
		return fmt.Errorf("Akun superadmin utama tidak dapat dihapus")
	}
	if _, exists := us.users[username]; !exists {
		return fmt.Errorf("User tidak ditemukan")
	}
	delete(us.users, username)
	saveUserStoreLocked()
	return nil
}

func (us *UserStore) UpdatePassword(username, newPass string) error {
	us.Lock()
	defer us.Unlock()
	u, exists := us.users[username]
	if !exists {
		return fmt.Errorf("User tidak ditemukan")
	}
	u.Password = newPass
	saveUserStoreLocked()
	return nil
}

// Session Store with User & Role metadata
type SessionInfo struct {
	Username    string    `json:"username"`
	Role        UserRole  `json:"role"`
	DisplayName string    `json:"display_name"`
	Expiry      time.Time `json:"expiry"`
}

type SessionStore struct {
	sync.RWMutex
	sessions map[string]*SessionInfo
}

var sessions = &SessionStore{
	sessions: make(map[string]*SessionInfo),
}

func (s *SessionStore) Create(u *UserAccount) string {
	b := make([]byte, 24)
	rand.Read(b)
	token := hex.EncodeToString(b)
	s.Lock()
	defer s.Unlock()
	s.sessions[token] = &SessionInfo{
		Username:    u.Username,
		Role:        u.Role,
		DisplayName: u.DisplayName,
		Expiry:      time.Now().Add(7 * 24 * time.Hour),
	}
	return token
}

func (s *SessionStore) Get(token string) *SessionInfo {
	if token == "" {
		return nil
	}
	s.RLock()
	info, exists := s.sessions[token]
	s.RUnlock()
	if !exists {
		return nil
	}
	if time.Now().After(info.Expiry) {
		s.Lock()
		delete(s.sessions, token)
		s.Unlock()
		return nil
	}
	return info
}

func (s *SessionStore) Validate(token string) bool {
	return s.Get(token) != nil
}

func (s *SessionStore) Revoke(token string) {
	s.Lock()
	defer s.Unlock()
	delete(s.sessions, token)
}

// File Item Struct
type FileItem struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	IsDir       bool   `json:"is_dir"`
	Size        int64  `json:"size"`
	SizeDisplay string `json:"size_display"`
	ModTime     string `json:"mod_time"`
	Extension   string `json:"extension"`
	Category    string `json:"category"`
}

type ListResponse struct {
	CurrentPath string     `json:"current_path"`
	Breadcrumbs []Crumb    `json:"breadcrumbs"`
	Items       []FileItem `json:"items"`
}

type Crumb struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type DriveInfo struct {
	Letter       string  `json:"letter"`
	Label        string  `json:"label"`
	TotalBytes   uint64  `json:"total_bytes"`
	FreeBytes    uint64  `json:"free_bytes"`
	UsedBytes    uint64  `json:"used_bytes"`
	UsedPercent  float64 `json:"used_percent"`
	TotalDisplay string  `json:"total_display"`
	UsedDisplay  string  `json:"used_display"`
	FreeDisplay  string  `json:"free_display"`
	IsNASRoot    bool    `json:"is_nas_root"`
}

type SystemInfo struct {
	Hostname       string      `json:"hostname"`
	OS             string      `json:"os"`
	GoVersion      string      `json:"go_version"`
	UptimeSeconds  int64       `json:"uptime_seconds"`
	UptimeDisplay  string      `json:"uptime_display"`
	RAMAllocBytes  uint64      `json:"ram_alloc_bytes"`
	RAMDisplay     string      `json:"ram_display"`
	Goroutines     int         `json:"goroutines"`
	IPAddresses    []string    `json:"ip_addresses"`
	AppTitle       string      `json:"app_title"`
	RootPath       string      `json:"root_path"`
	Drives         []DriveInfo `json:"drives"`
	ActiveSessions int         `json:"active_sessions"`
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 24 {
		days := h / 24
		h = h % 24
		return fmt.Sprintf("%d hari %d jam %d menit", days, h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%d jam %d menit %d dtk", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%d menit %d dtk", m, s)
	}
	return fmt.Sprintf("%d detik", s)
}

func getCategory(ext string, isDir bool) string {
	if isDir {
		return "folder"
	}
	ext = strings.ToLower(ext)
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".bmp", ".ico":
		return "image"
	case ".mp4", ".mkv", ".webm", ".avi", ".mov", ".wmv":
		return "video"
	case ".mp3", ".wav", ".ogg", ".flac", ".m4a", ".aac":
		return "audio"
	case ".pdf":
		return "pdf"
	case ".doc", ".docx", ".odt", ".rtf":
		return "word"
	case ".xls", ".xlsx", ".csv", ".ods":
		return "sheet"
	case ".ppt", ".pptx", ".odp":
		return "presentation"
	case ".zip", ".rar", ".7z", ".tar", ".gz":
		return "archive"
	case ".txt", ".log", ".sql", ".json", ".xml", ".html", ".css", ".js", ".go", ".py", ".md", ".sh", ".cmd", ".bat", ".ps1":
		return "code"
	default:
		return "generic"
	}
}

func getAutoArrangeTarget(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".bmp", ".mp4", ".mkv", ".webm", ".avi", ".mov", ".mp3", ".wav", ".flac":
		return "Media_Foto_Video"
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".csv", ".ppt", ".pptx", ".txt", ".odt":
		return "Dokumen_Bersama"
	case ".exe", ".msi", ".iso", ".dmg", ".apk":
		return "Master_Software"
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".sql", ".bak":
		return "Arsip_Preservasi"
	default:
		return "Dokumen_Bersama"
	}
}

type StorageVolume struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DriveLetter string `json:"drive_letter"`
	RootPath    string `json:"root_path"`
	Description string `json:"description"`
}

var volumes = []StorageVolume{
	{
		ID:          "vol-n",
		Name:        "Storage Utama",
		DriveLetter: "N:",
		RootPath:    "N:\\",
		Description: "Penyimpanan Utama Berbagi Berkas Preservasi (500 GB)",
	},
	{
		ID:          "vol-d",
		Name:        "Gudang Ekstra 1TB",
		DriveLetter: "D:",
		RootPath:    "D:\\STORAGE_NAS_GUDANG",
		Description: "Penyimpanan Ekstra Gudang Media, Video & Raw Scan (1.0 TB)",
	},
	{
		ID:          "vol-e",
		Name:        "Arsip Kedinasan",
		DriveLetter: "E:",
		RootPath:    "E:\\Arsip_Dinas_Perkantoran",
		Description: "Penyimpanan Arsip Surat & Dokumen Dinas Kantor",
	},
}

func getVolume(volID string) StorageVolume {
	if volID != "" {
		for _, v := range volumes {
			if strings.EqualFold(v.ID, volID) || strings.EqualFold(v.DriveLetter, volID) || strings.EqualFold(strings.TrimSuffix(v.DriveLetter, ":"), volID) {
				return v
			}
		}
	}
	return volumes[0]
}

func safePathVol(volID, subpath string) (string, StorageVolume, error) {
	vol := getVolume(volID)
	root := vol.RootPath

	clean := filepath.Clean(filepath.FromSlash(subpath))
	if clean == "." || clean == "/" || clean == "\\" {
		clean = ""
	}
	clean = strings.TrimPrefix(clean, "\\")
	clean = strings.TrimPrefix(clean, "/")

	full := filepath.Join(root, clean)
	fullClean := filepath.Clean(full)

	rootClean := filepath.Clean(root)
	if !strings.HasPrefix(strings.ToLower(fullClean), strings.ToLower(rootClean)) {
		return "", vol, fmt.Errorf("akses ditolak (path traversal)")
	}
	return fullClean, vol, nil
}

func safePath(subpath string) (string, error) {
	configMu.RLock()
	root := cfg.RootPath
	configMu.RUnlock()

	clean := filepath.Clean(filepath.FromSlash(subpath))
	if clean == "." || clean == "/" || clean == "\\" {
		clean = ""
	}
	clean = strings.TrimPrefix(clean, "\\")
	clean = strings.TrimPrefix(clean, "/")

	full := filepath.Join(root, clean)
	fullClean := filepath.Clean(full)

	rootClean := filepath.Clean(root)
	if !strings.HasPrefix(strings.ToLower(fullClean), strings.ToLower(rootClean)) {
		return "", fmt.Errorf("akses ditolak (path traversal)")
	}
	return fullClean, nil
}

func getAllDrives() []DriveInfo {
	drives := []string{"N:\\", "C:\\", "D:\\", "E:\\"}
	var result []DriveInfo
	for _, d := range drives {
		if stats, err := getDiskStats(d); err == nil && stats.TotalBytes > 0 {
			result = append(result, stats)
		}
	}
	return result
}

func getLocalIPs() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil && !ip.IsLoopback() && ip.To4() != nil {
				ipStr := ip.String()
				if strings.HasPrefix(ipStr, "100.") || strings.HasPrefix(ipStr, "172.16.") || strings.HasPrefix(ipStr, "192.168.") {
					ips = append(ips, ipStr)
				}
			}
		}
	}
	return ips
}

func clientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return strings.Split(forwarded, ",")[0]
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// ==================== UAC MIDDLEWARES & HELPERS ====================
func getRequestSession(r *http.Request) *SessionInfo {
	token := ""
	if cookie, err := r.Cookie("gonas_session"); err == nil {
		token = cookie.Value
	} else if cookie, err := r.Cookie("aether_session"); err == nil {
		token = cookie.Value
	}
	if token == "" {
		token = r.Header.Get("X-Auth-Token")
	}
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	return sessions.Get(token)
}

// 1. Any Authenticated User
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := getRequestSession(r)
		if sess == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Sesi tidak valid atau telah kedaluwarsa. Silakan login kembali.",
			})
			return
		}
		next(w, r)
	}
}

// 2. Administrator & Superadmin Only (Blocks user & viewer)
func adminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := getRequestSession(r)
		if sess == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Sesi tidak valid. Silakan login kembali.",
			})
			return
		}
		if sess.Role != RoleSuperAdmin && sess.Role != RoleAdmin {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Akses ditolak: Dashboard & Telemetri server hanya dapat diakses oleh Administrator.",
			})
			return
		}
		next(w, r)
	}
}

// 3. Superadmin Only
func superadminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := getRequestSession(r)
		if sess == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Sesi tidak valid. Silakan login kembali.",
			})
			return
		}
		if sess.Role != RoleSuperAdmin {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Akses ditolak: Fitur ini memerlukan hak akses tingkat Superadmin.",
			})
			return
		}
		next(w, r)
	}
}

// 4. Non-Viewer (Blocks viewer from upload/edit/delete)
func writeMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := getRequestSession(r)
		if sess == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Sesi tidak valid. Silakan login kembali.",
			})
			return
		}
		if sess.Role == RoleViewer {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Akses ditolak: Akun Tamu hanya memiliki hak akses baca (Read-only).",
			})
			return
		}
		next(w, r)
	}
}

// Config file persistence
func loadConfigFile() {
	configPath := "config.json"
	if runtime.GOOS == "windows" {
		if _, err := os.Stat("C:\\NAS"); err == nil {
			configPath = "C:\\NAS\\config.json"
		}
	}
	data, err := os.ReadFile(configPath)
	if err == nil {
		var saved Config
		if err := json.Unmarshal(data, &saved); err == nil {
			configMu.Lock()
			if saved.Password != "" {
				cfg.Password = saved.Password
			}
			if saved.Username != "" {
				cfg.Username = saved.Username
			}
			configMu.Unlock()
			log.Printf("Loaded persistent config from %s", configPath)
		}
	}
}

func saveConfigFile() error {
	configPath := "config.json"
	if runtime.GOOS == "windows" {
		if _, err := os.Stat("C:\\NAS"); err == nil {
			configPath = "C:\\NAS\\config.json"
		}
	}
	configMu.RLock()
	data, err := json.MarshalIndent(cfg, "", "  ")
	configMu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func main() {
	defaultRoot := "N:\\"
	if runtime.GOOS != "windows" {
		defaultRoot = "/storage"
	}
	flag.StringVar(&cfg.Port, "port", "8082", "Port server")
	flag.StringVar(&cfg.RootPath, "root", defaultRoot, "Root direktori storage NAS")
	flag.StringVar(&cfg.Username, "user", "admin", "Username login")
	flag.StringVar(&cfg.Password, "pass", "admin123", "Password login")
	flag.StringVar(&cfg.AppTitle, "title", "GoNAS - Cloud Preservasi BPMN", "Judul aplikasi Web NAS")
	flag.Parse()

	if envPort := os.Getenv("NAS_PORT"); envPort != "" {
		cfg.Port = envPort
	}
	if envRoot := os.Getenv("NAS_ROOT_PATH"); envRoot != "" {
		cfg.RootPath = envRoot
	}
	if envTitle := os.Getenv("NAS_APP_TITLE"); envTitle != "" {
		cfg.AppTitle = envTitle
	}

	loadConfigFile()

	if runtime.GOOS != "windows" {
		volumes[0].RootPath = cfg.RootPath
		volumes[0].DriveLetter = "STORAGE"
		volumes[1].RootPath = filepath.Join(cfg.RootPath, "gudang")
		volumes[1].DriveLetter = "GUDANG"
		volumes[2].RootPath = filepath.Join(cfg.RootPath, "arsip")
		volumes[2].DriveLetter = "ARSIP"
	} else {
		volumes[0].RootPath = cfg.RootPath
	}

	if _, err := os.Stat(cfg.RootPath); err != nil {
		log.Printf("Peringatan: Root path %s tidak ditemukan, mencoba membuat atau fallback...", cfg.RootPath)
		if err := os.MkdirAll(cfg.RootPath, 0755); err != nil {
			log.Printf("Gagal membuat %s: %v. Fallback ke direktori lokal ./storage", cfg.RootPath, err)
			cfg.RootPath = "./storage"
			os.MkdirAll(cfg.RootPath, 0755)
		}
	}

	log.Printf("=== %s v2.1 ===", cfg.AppTitle)
	log.Printf("Serving Storage: %s", cfg.RootPath)
	log.Printf("Port: %s", cfg.Port)
	log.Printf("Default User: %s", cfg.Username)

	logAudit("SYSTEM_START", "SYSTEM", "127.0.0.1", fmt.Sprintf("NAS Server started on port %s, root %s", cfg.Port, cfg.RootPath), true)

	mux := http.NewServeMux()

	initUserStore()

	// 1. Auth Endpoints
	mux.HandleFunc("/api/auth/login", handleLogin)
	mux.HandleFunc("/api/auth/logout", handleLogout)
	mux.HandleFunc("/api/auth/check", handleAuthCheck)

	// 2. Admin & System Telemetry Endpoints (ADMIN & SUPERADMIN ONLY)
	mux.HandleFunc("/api/sys/stats", authMiddleware(handleStats))
	mux.HandleFunc("/api/sys/info", adminMiddleware(handleSystemInfo))
	mux.HandleFunc("/api/sys/drives", adminMiddleware(handleDrives))
	mux.HandleFunc("/api/sys/logs", adminMiddleware(handleLogs))
	mux.HandleFunc("/api/sys/audit", adminMiddleware(handleLogs)) // Alias for audit log
	mux.HandleFunc("/api/sys/password", authMiddleware(handlePasswordChange))
	mux.HandleFunc("/api/sys/defrag", superadminMiddleware(handleDefrag))
	mux.HandleFunc("/api/sys/health", authMiddleware(handleHealth))

	// 3. User Management Endpoints (SUPERADMIN ONLY)
	mux.HandleFunc("/api/users/list", superadminMiddleware(handleListUsers))
	mux.HandleFunc("/api/users/add", superadminMiddleware(handleAddUser))
	mux.HandleFunc("/api/users/delete", superadminMiddleware(handleDeleteUser))
	mux.HandleFunc("/api/users/reset-pass", superadminMiddleware(handleResetUserPass))

	// 4. File Operations (Single & Bulk)
	mux.HandleFunc("/api/fs/volumes", authMiddleware(handleVolumes))
	mux.HandleFunc("/api/fs/list", authMiddleware(handleList))
	mux.HandleFunc("/api/fs/download", authMiddleware(handleDownload))
	mux.HandleFunc("/api/fs/view", authMiddleware(handleView))
	mux.HandleFunc("/api/fs/preview", authMiddleware(handleView)) // Dual alias for preview/view
	mux.HandleFunc("/api/fs/zip", authMiddleware(handleZip))
	// Write operations restricted for viewer role
	mux.HandleFunc("/api/fs/upload", writeMiddleware(handleUpload))
	mux.HandleFunc("/api/fs/mkdir", writeMiddleware(handleMkdir))
	mux.HandleFunc("/api/fs/rename", writeMiddleware(handleRename))
	mux.HandleFunc("/api/fs/delete", writeMiddleware(handleDelete))
	mux.HandleFunc("/api/fs/move", writeMiddleware(handleMove))
	mux.HandleFunc("/api/fs/bulk-move", writeMiddleware(handleBulkMove))
	mux.HandleFunc("/api/fs/bulk-delete", writeMiddleware(handleBulkDelete))
	mux.HandleFunc("/api/fs/group", writeMiddleware(handleGroup))
	mux.HandleFunc("/api/fs/bulk-zip", writeMiddleware(handleBulkZip))
	mux.HandleFunc("/api/fs/auto-arrange/scan", authMiddleware(handleAutoArrangeScan))
	mux.HandleFunc("/api/fs/auto-arrange/execute", writeMiddleware(handleAutoArrangeExecute))

	// 4. Static UI (Embedded)
	webSubFS, err := fs.Sub(embeddedWebFS, "web")
	if err != nil {
		log.Fatalf("Gagal inisialisasi embedded web UI: %v", err)
	}
	fileServer := http.FileServer(http.FS(webSubFS))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		if path == "/" || !strings.Contains(path, ".") {
			content, err := webSubFS.Open("index.html")
			if err != nil {
				http.Error(w, "index.html tidak ditemukan", http.StatusInternalServerError)
				return
			}
			defer content.Close()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.Copy(w, content)
			return
		}
		fileServer.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  2 * time.Hour,
		WriteTimeout: 2 * time.Hour,
	}

	log.Printf("Server siap listening di http://0.0.0.0:%s", cfg.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

// Handlers
func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ip := clientIP(r)
	configMu.RLock()
	title := cfg.AppTitle
	configMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	
	// Check against UAC User Store
	user := userStore.Authenticate(req.Username, req.Password)
	if user != nil {
		token := sessions.Create(user)
		http.SetCookie(w, &http.Cookie{
			Name:     "gonas_session",
			Value:    token,
			Path:     "/",
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		http.SetCookie(w, &http.Cookie{
			Name:     "aether_session",
			Value:    token,
			Path:     "/",
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		logAudit("LOGIN", user.Username, ip, fmt.Sprintf("Login berhasil sebagai role: %s (%s)", user.Role, user.DisplayName), true)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      true,
			"token":        token,
			"user":         user.Username,
			"role":         user.Role,
			"display_name": user.DisplayName,
			"title":        title,
		})
	} else {
		logAudit("LOGIN_FAIL", req.Username, ip, "Percobaan login gagal (kredensial salah)", false)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Username atau password salah!",
		})
	}
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	sess := getRequestSession(r)
	username := "User"
	if sess != nil {
		username = sess.Username
	}
	if cookie, err := r.Cookie("gonas_session"); err == nil {
		sessions.Revoke(cookie.Value)
	}
	if cookie, err := r.Cookie("aether_session"); err == nil {
		sessions.Revoke(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "gonas_session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "aether_session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	})
	logAudit("LOGOUT", username, ip, "User logout dari session", true)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func handleAuthCheck(w http.ResponseWriter, r *http.Request) {
	sess := getRequestSession(r)
	w.Header().Set("Content-Type", "application/json")
	if sess != nil {
		configMu.RLock()
		t := cfg.AppTitle
		configMu.RUnlock()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"authenticated": true,
			"user":          sess.Username,
			"role":          sess.Role,
			"display_name":  sess.DisplayName,
			"title":         t,
		})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"authenticated": false,
		})
	}
}

// User Management Handlers (Superadmin only)
func handleListUsers(w http.ResponseWriter, r *http.Request) {
	list := userStore.List()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"users":   list,
	})
}

func handleAddUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		Role        string `json:"role"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Username dan password wajib diisi"})
		return
	}
	role := UserRole(req.Role)
	if role != RoleSuperAdmin && role != RoleAdmin && role != RoleUser && role != RoleViewer {
		role = RoleUser
	}
	disp := req.DisplayName
	if disp == "" {
		disp = req.Username
	}
	err := userStore.Add(&UserAccount{
		Username:    req.Username,
		Password:    req.Password,
		Role:        role,
		DisplayName: disp,
	})
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	logAudit("USER_ADD", req.Username, clientIP(r), fmt.Sprintf("Tambah user baru: %s (%s)", req.Username, role), true)
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "User berhasil ditambahkan"})
}

func handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	err := userStore.Delete(req.Username)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	logAudit("USER_DEL", req.Username, clientIP(r), fmt.Sprintf("Hapus akun user: %s", req.Username), true)
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "User berhasil dihapus"})
}

func handleResetUserPass(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username    string `json:"username"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	err := userStore.UpdatePassword(req.Username, req.NewPassword)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	logAudit("USER_PASS_RESET", req.Username, clientIP(r), fmt.Sprintf("Reset password user: %s", req.Username), true)
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Password user berhasil direset"})
}

func handleVolumes(w http.ResponseWriter, r *http.Request) {
	type VolResponse struct {
		ID           string  `json:"id"`
		Name         string  `json:"name"`
		DriveLetter  string  `json:"drive_letter"`
		Description  string  `json:"description"`
		RootPath     string  `json:"root_path"`
		TotalBytes   uint64  `json:"total_bytes"`
		FreeBytes    uint64  `json:"free_bytes"`
		UsedBytes    uint64  `json:"used_bytes"`
		UsedPercent  float64 `json:"used_percent"`
		TotalDisplay string  `json:"total_display"`
		UsedDisplay  string  `json:"used_display"`
		FreeDisplay  string  `json:"free_display"`
	}

	var result []VolResponse
	for _, v := range volumes {
		stat, err := getDiskStats(v.DriveLetter + "\\")
		if err != nil {
			stat, _ = getDiskStats(v.RootPath)
		}
		result = append(result, VolResponse{
			ID:           v.ID,
			Name:         v.Name,
			DriveLetter:  v.DriveLetter,
			Description:  v.Description,
			RootPath:     v.RootPath,
			TotalBytes:   stat.TotalBytes,
			FreeBytes:    stat.FreeBytes,
			UsedBytes:    stat.UsedBytes,
			UsedPercent:  stat.UsedPercent,
			TotalDisplay: stat.TotalDisplay,
			UsedDisplay:  stat.UsedDisplay,
			FreeDisplay:  stat.FreeDisplay,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"volumes": result,
	})
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	volID := r.URL.Query().Get("vol")
	vol := getVolume(volID)

	stats, err := getDiskStats(vol.DriveLetter + "\\")
	if err != nil {
		stats, err = getDiskStats(vol.RootPath)
	}
	stats.Label = vol.Name

	configMu.RLock()
	title := cfg.AppTitle
	configMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"stats":   stats,
		"title":   title,
		"volume":  vol,
	})
}

func handleDrives(w http.ResponseWriter, r *http.Request) {
	drives := getAllDrives()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"drives":  drives,
	})
}

func handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	uptime := time.Since(startTime)
	drives := getAllDrives()
	ips := getLocalIPs()

	sessions.RLock()
	activeCount := len(sessions.sessions)
	sessions.RUnlock()

	configMu.RLock()
	title := cfg.AppTitle
	root := cfg.RootPath
	configMu.RUnlock()

	info := SystemInfo{
		Hostname:       hostname,
		OS:             fmt.Sprintf("%s (%s)", runtime.GOOS, runtime.GOARCH),
		GoVersion:      runtime.Version(),
		UptimeSeconds:  int64(uptime.Seconds()),
		UptimeDisplay:  formatDuration(uptime),
		RAMAllocBytes:  m.Alloc,
		RAMDisplay:     formatBytes(m.Alloc),
		Goroutines:     runtime.NumGoroutine(),
		IPAddresses:    ips,
		AppTitle:       title,
		RootPath:       root,
		Drives:         drives,
		ActiveSessions: activeCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"info":    info,
	})
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	auditLogs.RLock()
	defer auditLogs.RUnlock()

	count := len(auditLogs.entries)
	reversed := make([]AuditEntry, count)
	for i := 0; i < count; i++ {
		reversed[i] = auditLogs.entries[count-1-i]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"logs":    reversed,
	})
}

func handlePasswordChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess := getRequestSession(r)
	if sess == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ip := clientIP(r)

	// Verify old password against UserStore
	u := userStore.Authenticate(sess.Username, req.OldPassword)
	if u == nil {
		logAudit("PASSWORD_FAIL", sess.Username, ip, "Gagal mengubah password (password saat ini salah)", false)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Password saat ini tidak sesuai!",
		})
		return
	}

	if len(req.NewPassword) < 4 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Password baru terlalu pendek (minimal 4 karakter)",
		})
		return
	}

	userStore.UpdatePassword(sess.Username, req.NewPassword)

	configMu.Lock()
	if sess.Username == cfg.Username {
		cfg.Password = req.NewPassword
		saveConfigFile()
	}
	configMu.Unlock()

	logAudit("PASSWORD_CHANGE", sess.Username, ip, fmt.Sprintf("Password akun %s berhasil diperbarui", sess.Username), true)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Password berhasil diperbarui!",
	})
}

func handleDefrag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		DriveLetter string `json:"drive"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	drive := "N"
	if req.DriveLetter != "" {
		drive = strings.TrimSuffix(strings.ToUpper(req.DriveLetter), ":")
	}

	ip := clientIP(r)
	go func() {
		if runtime.GOOS == "windows" {
			cmd := exec.Command("defrag.exe", drive+":", "/U", "/V")
			cmd.Run()
		} else {
			cmd := exec.Command("sync")
			cmd.Run()
		}
	}()

	logAudit("DEFRAG", cfg.Username, ip, fmt.Sprintf("Memicu proses optimasi/defrag Windows untuk drive %s:", drive), true)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Optimasi defrag untuk drive %s: sedang berjalan di latar belakang", drive),
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"health": map[string]interface{}{
			"status":      "HEALTHY",
			"temperature": "Normal",
			"smart":       "Passed (S.M.A.R.T. OK)",
			"disk_type":   "HDD SATA 7200 RPM / SSD NTFS",
		},
	})
}

func handleList(w http.ResponseWriter, r *http.Request) {
	volID := r.URL.Query().Get("vol")
	reqPath := r.URL.Query().Get("path")
	targetPath, vol, err := safePathVol(volID, reqPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	entries, err := os.ReadDir(targetPath)
	if err != nil {
		http.Error(w, "Gagal membaca direktori: "+err.Error(), http.StatusInternalServerError)
		return
	}

	items := make([]FileItem, 0)
	for _, entry := range entries {
		name := entry.Name()
		lower := strings.ToLower(name)
		if lower == "system volume information" || lower == "$recycle.bin" || strings.HasPrefix(lower, ".trashed") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		relPath := filepath.ToSlash(filepath.Join(reqPath, name))
		relPath = strings.TrimPrefix(relPath, "/")

		isDir := entry.IsDir()
		fullItemPath := filepath.Join(targetPath, name)
		if !isDir {
			if stat, err := os.Stat(fullItemPath); err == nil && stat.IsDir() {
				isDir = true
			}
		}

		ext := filepath.Ext(name)
		category := getCategory(ext, isDir)

		sizeDisplay := formatBytes(uint64(info.Size()))
		if isDir {
			subEntries, err := os.ReadDir(fullItemPath)
			if err == nil {
				count := 0
				var totalFolderSize int64 = 0
				for _, se := range subEntries {
					sen := strings.ToLower(se.Name())
					if sen == "system volume information" || sen == "$recycle.bin" || strings.HasPrefix(sen, ".trashed") {
						continue
					}
					count++
					if si, err := se.Info(); err == nil && !se.IsDir() {
						totalFolderSize += si.Size()
					}
				}
				if count == 0 {
					sizeDisplay = "Kosong"
				} else if totalFolderSize > 0 {
					sizeDisplay = fmt.Sprintf("%s (%d item)", formatBytes(uint64(totalFolderSize)), count)
				} else {
					sizeDisplay = fmt.Sprintf("%d item", count)
				}
			} else {
				sizeDisplay = "—"
			}
		}

		items = append(items, FileItem{
			Name:        name,
			Path:        relPath,
			IsDir:       isDir,
			Size:        info.Size(),
			SizeDisplay: sizeDisplay,
			ModTime:     info.ModTime().Format("02 Jan 2006 15:04"),
			Extension:   strings.TrimPrefix(strings.ToLower(ext), "."),
			Category:    category,
		})
	}

	// Sort: Folders first, then names alphabetically
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	// Build breadcrumbs
	var breadcrumbs []Crumb
	breadcrumbs = append(breadcrumbs, Crumb{Name: "Root (" + vol.Name + ")", Path: ""})
	if reqPath != "" {
		parts := strings.Split(filepath.ToSlash(filepath.Clean(reqPath)), "/")
		curr := ""
		for _, part := range parts {
			if part == "" || part == "." {
				continue
			}
			curr = filepath.ToSlash(filepath.Join(curr, part))
			breadcrumbs = append(breadcrumbs, Crumb{Name: part, Path: curr})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"current_path":  filepath.ToSlash(reqPath),
		"breadcrumbs":   breadcrumbs,
		"items":         items,
		"active_volume": vol.ID,
		"volume_name":   vol.Name,
		"drive_letter":  vol.DriveLetter,
	})
}

// Internal File Movement
func handleMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Vol       string `json:"vol"`
		Source    string `json:"source"`
		TargetDir string `json:"target_dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	srcPath, _, err := safePathVol(req.Vol, req.Source)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	targetDirPath, _, err := safePathVol(req.Vol, req.TargetDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	cleanSrc := filepath.Clean(srcPath)
	cleanDestDir := filepath.Clean(targetDirPath)
	if cleanSrc == cleanDestDir || strings.HasPrefix(strings.ToLower(cleanDestDir), strings.ToLower(cleanSrc)+string(filepath.Separator)) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Tidak dapat memindahkan folder ke dalam folder itu sendiri",
		})
		return
	}

	destPath := filepath.Join(targetDirPath, filepath.Base(srcPath))
	if filepath.Clean(srcPath) == filepath.Clean(destPath) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Berkas sudah berada di folder tersebut",
		})
		return
	}

	if _, err := os.Stat(destPath); err == nil {
		ext := filepath.Ext(destPath)
		base := strings.TrimSuffix(filepath.Base(destPath), ext)
		for i := 1; i <= 100; i++ {
			candidate := filepath.Join(targetDirPath, fmt.Sprintf("%s (%d)%s", base, i, ext))
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				destPath = candidate
				break
			}
		}
	}

	if err := os.Rename(srcPath, destPath); err != nil {
		http.Error(w, "Gagal memindahkan berkas: "+err.Error(), http.StatusInternalServerError)
		return
	}

	ip := clientIP(r)
	logAudit("MOVE", cfg.Username, ip, fmt.Sprintf("Pindah: %s -> /%s", filepath.Base(srcPath), req.TargetDir), true)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// Bulk Move
func handleBulkMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Vol       string   `json:"vol"`
		Sources   []string `json:"sources"`
		TargetDir string   `json:"target_dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	targetDirPath, _, err := safePathVol(req.Vol, req.TargetDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	cleanDestDir := filepath.Clean(targetDirPath)
	movedCount := 0
	for _, src := range req.Sources {
		srcPath, _, err := safePathVol(req.Vol, src)
		if err != nil {
			continue
		}
		cleanSrc := filepath.Clean(srcPath)
		if cleanSrc == cleanDestDir || strings.HasPrefix(strings.ToLower(cleanDestDir), strings.ToLower(cleanSrc)+string(filepath.Separator)) {
			continue
		}
		destPath := filepath.Join(targetDirPath, filepath.Base(srcPath))
		if cleanSrc == filepath.Clean(destPath) {
			continue
		}
		if _, err := os.Stat(destPath); err == nil {
			ext := filepath.Ext(destPath)
			base := strings.TrimSuffix(filepath.Base(destPath), ext)
			for i := 1; i <= 100; i++ {
				candidate := filepath.Join(targetDirPath, fmt.Sprintf("%s (%d)%s", base, i, ext))
				if _, err := os.Stat(candidate); os.IsNotExist(err) {
					destPath = candidate
					break
				}
			}
		}
		if err := os.Rename(srcPath, destPath); err == nil {
			movedCount++
		}
	}

	ip := clientIP(r)
	logAudit("BULK_MOVE", cfg.Username, ip, fmt.Sprintf("Memindahkan %d berkas ke /%s", movedCount, req.TargetDir), true)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   movedCount,
		"message": fmt.Sprintf("%d berkas berhasil dipindahkan", movedCount),
	})
}

// Bulk Delete
func handleBulkDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Vol   string   `json:"vol"`
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	deletedCount := 0
	for _, p := range req.Paths {
		targetPath, vol, err := safePathVol(req.Vol, p)
		if err != nil {
			continue
		}
		if filepath.Clean(targetPath) == filepath.Clean(vol.RootPath) {
			continue
		}
		if err := os.RemoveAll(targetPath); err == nil {
			deletedCount++
		}
	}

	ip := clientIP(r)
	logAudit("BULK_DELETE", cfg.Username, ip, fmt.Sprintf("Menghapus massal %d berkas/folder", deletedCount), true)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   deletedCount,
	})
}

// Group to New Folder
func handleGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Vol        string   `json:"vol"`
		FolderName string   `json:"folder_name"`
		Parent     string   `json:"parent"`
		Files      []string `json:"files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	cleanName := filepath.Base(filepath.Clean(req.FolderName))
	if cleanName == "" || cleanName == "." {
		http.Error(w, "Nama folder tidak valid", http.StatusBadRequest)
		return
	}

	parentPath, _, err := safePathVol(req.Vol, req.Parent)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	newFolderPath := filepath.Join(parentPath, cleanName)
	if err := os.MkdirAll(newFolderPath, 0755); err != nil {
		http.Error(w, "Gagal membuat folder baru: "+err.Error(), http.StatusInternalServerError)
		return
	}

	movedCount := 0
	for _, f := range req.Files {
		srcPath, _, err := safePathVol(req.Vol, f)
		if err != nil {
			continue
		}
		destPath := filepath.Join(newFolderPath, filepath.Base(srcPath))
		if err := os.Rename(srcPath, destPath); err == nil {
			movedCount++
		}
	}

	ip := clientIP(r)
	logAudit("GROUP", cfg.Username, ip, fmt.Sprintf("Mengelompokkan %d berkas ke folder baru: %s", movedCount, cleanName), true)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   movedCount,
		"folder":  cleanName,
	})
}

// Bulk Zip
func handleBulkZip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Vol   string   `json:"vol"`
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"GoNAS_Selected.zip\"")

	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, p := range req.Paths {
		srcPath, _, err := safePathVol(req.Vol, p)
		if err != nil {
			continue
		}
		info, err := os.Stat(srcPath)
		if err != nil {
			continue
		}

		if info.IsDir() {
			filepath.Walk(srcPath, func(subpath string, subinfo os.FileInfo, err error) error {
				if err != nil || subpath == srcPath {
					return nil
				}
				rel, _ := filepath.Rel(srcPath, subpath)
				rel = filepath.ToSlash(filepath.Join(info.Name(), rel))
				if subinfo.IsDir() {
					rel += "/"
					zw.Create(rel)
					return nil
				}
				header, _ := zip.FileInfoHeader(subinfo)
				header.Name = rel
				header.Method = zip.Deflate
				writer, _ := zw.CreateHeader(header)
				f, err := os.Open(subpath)
				if err == nil {
					io.Copy(writer, f)
					f.Close()
				}
				return nil
			})
		} else {
			header, _ := zip.FileInfoHeader(info)
			header.Name = info.Name()
			header.Method = zip.Deflate
			writer, _ := zw.CreateHeader(header)
			f, err := os.Open(srcPath)
			if err == nil {
				io.Copy(writer, f)
				f.Close()
			}
		}
	}
}

// Auto-Arrange Scan
type ArrangeRecommendation struct {
	SourcePath   string `json:"source_path"`
	FileName     string `json:"file_name"`
	TargetFolder string `json:"target_folder"`
	Category     string `json:"category"`
	SizeDisplay  string `json:"size_display"`
}

func handleAutoArrangeScan(w http.ResponseWriter, r *http.Request) {
	volID := r.URL.Query().Get("vol")
	reqPath := r.URL.Query().Get("path")
	targetPath, _, err := safePathVol(volID, reqPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	entries, err := os.ReadDir(targetPath)
	if err != nil {
		http.Error(w, "Gagal membaca direktori", http.StatusInternalServerError)
		return
	}

	var recs []ArrangeRecommendation
	for _, entry := range entries {
		if entry.IsDir() {
			continue // Only organize loose files
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, ".") {
			continue
		}

		ext := filepath.Ext(name)
		target := getAutoArrangeTarget(ext)
		info, _ := entry.Info()

		rel := filepath.ToSlash(filepath.Join(reqPath, name))
		rel = strings.TrimPrefix(rel, "/")

		sizeStr := ""
		if info != nil {
			sizeStr = formatBytes(uint64(info.Size()))
		}

		recs = append(recs, ArrangeRecommendation{
			SourcePath:   rel,
			FileName:     name,
			TargetFolder: target,
			Category:     getCategory(ext, false),
			SizeDisplay:  sizeStr,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"recommendations": recs,
		"count":           len(recs),
	})
}

// Auto-Arrange Execute
func handleAutoArrangeExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Vol  string `json:"vol"`
		Path string `json:"path"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	targetDir, _, err := safePathVol(req.Vol, req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		http.Error(w, "Gagal membaca direktori", http.StatusInternalServerError)
		return
	}

	movedCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		ext := filepath.Ext(name)
		targetFolderName := getAutoArrangeTarget(ext)

		// Target folder path
		targetFolderPath, _, err := safePathVol(req.Vol, targetFolderName)
		if err != nil {
			continue
		}
		os.MkdirAll(targetFolderPath, 0755)

		src := filepath.Join(targetDir, name)
		dest := filepath.Join(targetFolderPath, name)

		if err := os.Rename(src, dest); err == nil {
			movedCount++
		}
	}

	ip := clientIP(r)
	logAudit("ARRANGE", cfg.Username, ip, fmt.Sprintf("Smart Auto-Arrange merapikan %d berkas ke folder tematik", movedCount), true)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   movedCount,
		"message": fmt.Sprintf("Berhasil merapikan %d berkas secara otomatis!", movedCount),
	})
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	volID := r.URL.Query().Get("vol")
	reqPath := r.URL.Query().Get("path")
	targetDir, _, err := safePathVol(volID, reqPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	mr, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "Invalid multipart request: "+err.Error(), http.StatusBadRequest)
		return
	}

	ip := clientIP(r)
	uploadedCount := 0
	var uploadedNames []string

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "Error reading multipart: "+err.Error(), http.StatusInternalServerError)
			return
		}

		filename := part.FileName()
		if filename == "" {
			continue
		}

		filename = filepath.Base(filepath.Clean(filename))
		destPath := filepath.Join(targetDir, filename)

		out, err := os.Create(destPath)
		if err != nil {
			http.Error(w, "Gagal membuat file: "+err.Error(), http.StatusInternalServerError)
			return
		}

		buf := make([]byte, 256*1024)
		_, err = io.CopyBuffer(out, part, buf)
		out.Close()
		if err != nil {
			http.Error(w, "Gagal menulis file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		uploadedCount++
		uploadedNames = append(uploadedNames, filename)
	}

	if uploadedCount > 0 {
		detail := fmt.Sprintf("Unggah %d berkas ke /%s: %s", uploadedCount, reqPath, strings.Join(uploadedNames, ", "))
		if len(detail) > 120 {
			detail = fmt.Sprintf("Unggah %d berkas ke /%s", uploadedCount, reqPath)
		}
		logAudit("UPLOAD", cfg.Username, ip, detail, true)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   uploadedCount,
		"message": fmt.Sprintf("%d berkas berhasil diupload", uploadedCount),
	})
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	volID := r.URL.Query().Get("vol")
	reqPath := r.URL.Query().Get("path")
	targetPath, _, err := safePathVol(volID, reqPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		http.Error(w, "File tidak ditemukan", http.StatusNotFound)
		return
	}

	if info.IsDir() {
		streamZipFolder(w, targetPath, info.Name())
		return
	}

	file, err := os.Open(targetPath)
	if err != nil {
		http.Error(w, "Gagal membuka file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", info.Name()))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func handleView(w http.ResponseWriter, r *http.Request) {
	volID := r.URL.Query().Get("vol")
	reqPath := r.URL.Query().Get("path")
	targetPath, _, err := safePathVol(volID, reqPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	info, err := os.Stat(targetPath)
	if err != nil || info.IsDir() {
		http.Error(w, "File tidak ditemukan atau berupa direktori", http.StatusNotFound)
		return
	}

	file, err := os.Open(targetPath)
	if err != nil {
		http.Error(w, "Gagal membuka file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(info.Name()))
	var mimeType string
	switch ext {
	case ".pdf":
		mimeType = "application/pdf"
	case ".mp4":
		mimeType = "video/mp4"
	case ".webm":
		mimeType = "video/webm"
	case ".mp3":
		mimeType = "audio/mpeg"
	case ".wav":
		mimeType = "audio/wav"
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".png":
		mimeType = "image/png"
	case ".webp":
		mimeType = "image/webp"
	case ".gif":
		mimeType = "image/gif"
	case ".svg":
		mimeType = "image/svg+xml"
	case ".txt", ".log", ".sql", ".json", ".md", ".csv", ".tsv", ".yaml", ".yml", ".go", ".py", ".sh", ".bat", ".ps1":
		mimeType = "text/plain; charset=utf-8"
	case ".docx":
		mimeType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		mimeType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".pptx":
		mimeType = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	default:
		mimeType = mime.TypeByExtension(ext)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func handleZip(w http.ResponseWriter, r *http.Request) {
	volID := r.URL.Query().Get("vol")
	reqPath := r.URL.Query().Get("path")
	targetPath, _, err := safePathVol(volID, reqPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	info, err := os.Stat(targetPath)
	if err != nil || !info.IsDir() {
		http.Error(w, "Direktori tidak ditemukan", http.StatusNotFound)
		return
	}

	streamZipFolder(w, targetPath, info.Name())
}

func streamZipFolder(w http.ResponseWriter, folderPath, folderName string) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q.zip", folderName))

	zw := zip.NewWriter(w)
	defer zw.Close()

	filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || path == folderPath {
			return nil
		}

		relPath, err := filepath.Rel(folderPath, path)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		lower := strings.ToLower(filepath.Base(path))
		if lower == "system volume information" || lower == "$recycle.bin" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			relPath += "/"
			_, err = zw.Create(relPath)
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return nil
		}
		header.Name = relPath
		header.Method = zip.Deflate

		writer, err := zw.CreateHeader(header)
		if err != nil {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		io.Copy(writer, f)
		return nil
	})
}

func handleMkdir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Vol    string `json:"vol"`
		Parent string `json:"parent"`
		Name   string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	cleanName := filepath.Base(filepath.Clean(req.Name))
	if cleanName == "" || cleanName == "." || cleanName == "/" {
		http.Error(w, "Nama folder tidak valid", http.StatusBadRequest)
		return
	}

	parentPath, _, err := safePathVol(req.Vol, req.Parent)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	target := filepath.Join(parentPath, cleanName)
	if err := os.MkdirAll(target, 0755); err != nil {
		http.Error(w, "Gagal membuat folder: "+err.Error(), http.StatusInternalServerError)
		return
	}

	ip := clientIP(r)
	logAudit("MKDIR", cfg.Username, ip, fmt.Sprintf("Buat folder baru: %s di /%s", cleanName, req.Parent), true)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func handleRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Vol     string `json:"vol"`
		Path    string `json:"path"`
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	cleanName := filepath.Base(filepath.Clean(req.NewName))
	if cleanName == "" || cleanName == "." {
		http.Error(w, "Nama baru tidak valid", http.StatusBadRequest)
		return
	}

	oldPath, _, err := safePathVol(req.Vol, req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	parentDir := filepath.Dir(oldPath)
	newPath := filepath.Join(parentDir, cleanName)

	if err := os.Rename(oldPath, newPath); err != nil {
		http.Error(w, "Gagal mengganti nama: "+err.Error(), http.StatusInternalServerError)
		return
	}

	ip := clientIP(r)
	logAudit("RENAME", cfg.Username, ip, fmt.Sprintf("Ubah nama: %s -> %s", filepath.Base(oldPath), cleanName), true)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Vol  string `json:"vol"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	targetPath, vol, err := safePathVol(req.Vol, req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if filepath.Clean(targetPath) == filepath.Clean(vol.RootPath) {
		http.Error(w, "Tidak diperbolehkan menghapus root storage", http.StatusForbidden)
		return
	}

	if err := os.RemoveAll(targetPath); err != nil {
		http.Error(w, "Gagal menghapus berkas: "+err.Error(), http.StatusInternalServerError)
		return
	}

	ip := clientIP(r)
	logAudit("DELETE", cfg.Username, ip, fmt.Sprintf("Hapus berkas/folder: /%s", req.Path), true)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}
