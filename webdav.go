package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"golang.org/x/net/webdav"
)

// setupWebDAV registers the WebDAV endpoints with UAC authentication and role enforcement
func setupWebDAV(mux *http.ServeMux, rootDir string) {
	davHandler := &webdav.Handler{
		Prefix:     "/webdav",
		FileSystem: webdav.Dir(rootDir),
		LockSystem: webdav.NewMemLS(),
		Logger: func(req *http.Request, err error) {
			if err != nil && err != os.ErrNotExist {
				log.Printf("[WebDAV Error] %s %s: %v", req.Method, req.URL.Path, err)
			}
		},
	}

	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		// Provide WebDAV discovery capabilities for Windows Explorer & macOS Finder
		if r.Method == "OPTIONS" {
			w.Header().Set("DAV", "1, 2")
			w.Header().Set("MS-Author-Via", "DAV")
			w.Header().Set("Allow", "OPTIONS, GET, HEAD, POST, PUT, DELETE, TRACE, PROPFIND, PROPPATCH, MKCOL, COPY, MOVE, LOCK, UNLOCK")
		}

		// WebDAV uses HTTP Basic Authentication
		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="GoNAS WebDAV Storage"`)
			http.Error(w, "Unauthorized: Kredensial WebDAV diperlukan", http.StatusUnauthorized)
			return
		}

		user := userStore.Authenticate(username, password)
		if user == nil {
			w.Header().Set("WWW-Authenticate", `Basic realm="GoNAS WebDAV Storage"`)
			http.Error(w, "Unauthorized: Username atau password salah", http.StatusUnauthorized)
			logAudit("WEBDAV_AUTH_FAIL", username, clientIP(r), "Percobaan login WebDAV gagal: kredensial salah", false)
			return
		}

		// Role-based Access Control (Viewer can only read)
		if user.Role == RoleViewer {
			switch r.Method {
			case "PUT", "DELETE", "MKCOL", "MOVE", "COPY", "PROPPATCH", "LOCK", "UNLOCK":
				http.Error(w, "Forbidden: Akun Viewer hanya memiliki izin baca (Read-only)", http.StatusForbidden)
				logAudit("WEBDAV_WRITE_DENIED", username, clientIP(r), fmt.Sprintf("Aksi WebDAV %s ditolak untuk role Viewer pada %s", r.Method, r.URL.Path), false)
				return
			}
		}

		// Audit logging for write/modification operations
		switch r.Method {
		case "PUT", "DELETE", "MKCOL", "MOVE":
			logAudit("WEBDAV_"+r.Method, username, clientIP(r), fmt.Sprintf("WebDAV %s: %s", r.Method, r.URL.Path), true)
		}

		// Standardize path: redirect /webdav to /webdav/
		if r.URL.Path == "/webdav" {
			http.Redirect(w, r, "/webdav/", http.StatusPermanentRedirect)
			return
		}

		davHandler.ServeHTTP(w, r)
	}

	mux.HandleFunc("/webdav", handlerFunc)
	mux.HandleFunc("/webdav/", handlerFunc)
	log.Printf("WebDAV Server aktif di: /webdav/ (Storage: %s)", rootDir)
}
