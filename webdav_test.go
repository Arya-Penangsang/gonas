package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupTestWebDAVServer(t *testing.T) (*http.ServeMux, string) {
	tempDir := t.TempDir()

	// Initialize user store in test
	userStore.Lock()
	userStore.users = map[string]*UserAccount{
		"superadmin": {
			Username:    "superadmin",
			Password:    hashPassword("admin123"),
			Role:        RoleSuperAdmin,
			DisplayName: "Super Admin",
			CreatedAt:   time.Now(),
		},
		"admin": {
			Username:    "admin",
			Password:    hashPassword("admin123"),
			Role:        RoleAdmin,
			DisplayName: "Admin",
			CreatedAt:   time.Now(),
		},
		"staf": {
			Username:    "staf",
			Password:    hashPassword("staf123"),
			Role:        RoleUser,
			DisplayName: "Staf User",
			CreatedAt:   time.Now(),
		},
		"tamu": {
			Username:    "tamu",
			Password:    hashPassword("tamu123"),
			Role:        RoleViewer,
			DisplayName: "Tamu Viewer",
			CreatedAt:   time.Now(),
		},
	}
	userStore.Unlock()

	mux := http.NewServeMux()
	setupWebDAV(mux, tempDir)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			w.Header().Set("DAV", "1, 2")
			w.Header().Set("MS-Author-Via", "DAV")
			w.Header().Set("Allow", "OPTIONS, GET, HEAD, POST, PUT, DELETE, TRACE, PROPFIND, PROPPATCH, MKCOL, COPY, MOVE, LOCK, UNLOCK")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	return mux, tempDir
}

func TestWebDAV_DiscoveryOptions(t *testing.T) {
	mux, _ := setupTestWebDAVServer(t)

	// Test 1: OPTIONS on /
	req := httptest.NewRequest("OPTIONS", "/", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 OK on OPTIONS /, got %d", rr.Code)
	}
	if !strings.Contains(rr.Header().Get("DAV"), "1") {
		t.Errorf("Missing DAV header on OPTIONS /, got %s", rr.Header().Get("DAV"))
	}
	if rr.Header().Get("MS-Author-Via") != "DAV" {
		t.Errorf("Missing MS-Author-Via on OPTIONS /, got %s", rr.Header().Get("MS-Author-Via"))
	}

	// Test 2: OPTIONS on /webdav/
	req2 := httptest.NewRequest("OPTIONS", "/webdav/", nil)
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req2)

	// Even before basic auth, OPTIONS must reveal DAV capabilities so Windows Explorer proceeds
	if !strings.Contains(rr2.Header().Get("DAV"), "1") {
		t.Errorf("Missing DAV header on OPTIONS /webdav/, got %s", rr2.Header().Get("DAV"))
	}
}

func TestWebDAV_Authentication(t *testing.T) {
	mux, _ := setupTestWebDAVServer(t)

	// Unauthenticated request
	req := httptest.NewRequest("PROPFIND", "/webdav/", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", rr.Code)
	}
	if !strings.Contains(rr.Header().Get("WWW-Authenticate"), "Basic") {
		t.Errorf("Expected Basic WWW-Authenticate header, got %s", rr.Header().Get("WWW-Authenticate"))
	}

	// Invalid password request
	reqBad := httptest.NewRequest("PROPFIND", "/webdav/", nil)
	reqBad.SetBasicAuth("admin", "wrongpassword")
	rrBad := httptest.NewRecorder()
	mux.ServeHTTP(rrBad, reqBad)

	if rrBad.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for bad credentials, got %d", rrBad.Code)
	}
}

func TestWebDAV_AdminCRUD(t *testing.T) {
	mux, tempDir := setupTestWebDAVServer(t)

	// 1. Create a subfolder with MKCOL
	reqMkcol := httptest.NewRequest("MKCOL", "/webdav/dokumen_penting", nil)
	reqMkcol.SetBasicAuth("superadmin", "admin123")
	rrMkcol := httptest.NewRecorder()
	mux.ServeHTTP(rrMkcol, reqMkcol)

	if rrMkcol.Code != http.StatusCreated {
		t.Fatalf("MKCOL failed with status %d: %s", rrMkcol.Code, rrMkcol.Body.String())
	}

	folderPath := filepath.Join(tempDir, "dokumen_penting")
	if fi, err := os.Stat(folderPath); err != nil || !fi.IsDir() {
		t.Fatalf("Folder was not created on disk: %v", err)
	}

	// 2. Upload/Create a file with PUT
	content := "Halo dari WebDAV! Berkas uji integrasi Windows Explorer."
	reqPut := httptest.NewRequest("PUT", "/webdav/dokumen_penting/catatan.txt", strings.NewReader(content))
	reqPut.SetBasicAuth("superadmin", "admin123")
	rrPut := httptest.NewRecorder()
	mux.ServeHTTP(rrPut, reqPut)

	if rrPut.Code != http.StatusCreated && rrPut.Code != http.StatusNoContent {
		t.Fatalf("PUT failed with status %d: %s", rrPut.Code, rrPut.Body.String())
	}

	filePath := filepath.Join(folderPath, "catatan.txt")
	diskData, err := os.ReadFile(filePath)
	if err != nil || string(diskData) != content {
		t.Fatalf("File on disk does not match: %s, err: %v", string(diskData), err)
	}

	// 3. Read file with GET
	reqGet := httptest.NewRequest("GET", "/webdav/dokumen_penting/catatan.txt", nil)
	reqGet.SetBasicAuth("superadmin", "admin123")
	rrGet := httptest.NewRecorder()
	mux.ServeHTTP(rrGet, reqGet)

	if rrGet.Code != http.StatusOK {
		t.Fatalf("GET failed with status %d", rrGet.Code)
	}
	if rrGet.Body.String() != content {
		t.Fatalf("GET body mismatch. Expected %q, got %q", content, rrGet.Body.String())
	}

	// 4. List directory with PROPFIND
	reqProp := httptest.NewRequest("PROPFIND", "/webdav/dokumen_penting/", nil)
	reqProp.Header.Set("Depth", "1")
	reqProp.SetBasicAuth("superadmin", "admin123")
	rrProp := httptest.NewRecorder()
	mux.ServeHTTP(rrProp, reqProp)

	if rrProp.Code != 207 { // 207 Multi-Status
		t.Fatalf("PROPFIND failed with status %d: %s", rrProp.Code, rrProp.Body.String())
	}
	if !strings.Contains(rrProp.Body.String(), "catatan.txt") {
		t.Fatalf("PROPFIND response missing catatan.txt: %s", rrProp.Body.String())
	}

	// 5. Delete file with DELETE
	reqDel := httptest.NewRequest("DELETE", "/webdav/dokumen_penting/catatan.txt", nil)
	reqDel.SetBasicAuth("superadmin", "admin123")
	rrDel := httptest.NewRecorder()
	mux.ServeHTTP(rrDel, reqDel)

	if rrDel.Code != http.StatusNoContent && rrDel.Code != http.StatusOK {
		t.Fatalf("DELETE failed with status %d", rrDel.Code)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("File should have been deleted from disk")
	}
}

func TestWebDAV_ViewerRoleRestriction(t *testing.T) {
	mux, tempDir := setupTestWebDAVServer(t)

	// Create sample file on disk
	samplePath := filepath.Join(tempDir, "laporan_publik.txt")
	os.WriteFile(samplePath, []byte("Data publik untuk tamu"), 0644)

	// 1. Viewer can GET (Read)
	reqGet := httptest.NewRequest("GET", "/webdav/laporan_publik.txt", nil)
	reqGet.SetBasicAuth("tamu", "tamu123")
	rrGet := httptest.NewRecorder()
	mux.ServeHTTP(rrGet, reqGet)

	if rrGet.Code != http.StatusOK {
		t.Fatalf("Viewer should be allowed to GET, got status %d", rrGet.Code)
	}

	// 2. Viewer cannot PUT (Write)
	reqPut := httptest.NewRequest("PUT", "/webdav/hacked.txt", bytes.NewReader([]byte("Forbidden content")))
	reqPut.SetBasicAuth("tamu", "tamu123")
	rrPut := httptest.NewRecorder()
	mux.ServeHTTP(rrPut, reqPut)

	if rrPut.Code != http.StatusForbidden {
		t.Fatalf("Viewer should be blocked from PUT with 403 Forbidden, got status %d", rrPut.Code)
	}

	// 3. Viewer cannot MKCOL (Create Folder)
	reqMkcol := httptest.NewRequest("MKCOL", "/webdav/folder_tamu", nil)
	reqMkcol.SetBasicAuth("tamu", "tamu123")
	rrMkcol := httptest.NewRecorder()
	mux.ServeHTTP(rrMkcol, reqMkcol)

	if rrMkcol.Code != http.StatusForbidden {
		t.Fatalf("Viewer should be blocked from MKCOL with 403 Forbidden, got status %d", rrMkcol.Code)
	}

	// 4. Viewer cannot DELETE
	reqDel := httptest.NewRequest("DELETE", "/webdav/laporan_publik.txt", nil)
	reqDel.SetBasicAuth("tamu", "tamu123")
	rrDel := httptest.NewRecorder()
	mux.ServeHTTP(rrDel, reqDel)

	if rrDel.Code != http.StatusForbidden {
		t.Fatalf("Viewer should be blocked from DELETE with 403 Forbidden, got status %d", rrDel.Code)
	}

	// Ensure file was not deleted
	if _, err := os.Stat(samplePath); os.IsNotExist(err) {
		t.Fatalf("Sample file was unexpectedly deleted by viewer")
	}
}
