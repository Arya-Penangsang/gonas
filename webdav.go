package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

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
		w.Header().Set("DAV", "1, 2")
		w.Header().Set("MS-Author-Via", "DAV")
		w.Header().Set("Allow", "OPTIONS, GET, HEAD, POST, PUT, DELETE, TRACE, PROPFIND, PROPPATCH, MKCOL, COPY, MOVE, LOCK, UNLOCK")

		// Transparently normalize /webdav to /webdav/ without HTTP redirect (Windows WebClient fails on redirects!)
		if r.URL.Path == "/webdav" {
			r.URL.Path = "/webdav/"
		}

		// Windows Explorer & WebDAV clients probe with OPTIONS - MUST return 200 OK without auth!
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		ua := r.Header.Get("User-Agent")
		isWebDAVClient := strings.Contains(ua, "Microsoft-WebDAV") ||
			strings.Contains(ua, "WebDAV") ||
			strings.Contains(ua, "DavClnt") ||
			strings.Contains(ua, "gvfs") ||
			strings.Contains(ua, "Darwin")

		// Friendly browser intercept:
		// Only when opened in a standard web browser (e.g. Chrome/Firefox/Edge), not a WebDAV client!
		if r.Method == "GET" && r.URL.Path == "/webdav/" && strings.Contains(r.Header.Get("Accept"), "text/html") && !isWebDAVClient {
			renderWebDAVBrowserInfo(w, r)
			return
		}

		// WebDAV uses HTTP Basic Authentication for WebDAV clients (Explorer, Finder, Solid Explorer, etc.)
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

		davHandler.ServeHTTP(w, r)
	}

	mux.HandleFunc("/webdav", handlerFunc)
	mux.HandleFunc("/webdav/", handlerFunc)
	log.Printf("WebDAV Server aktif di: /webdav/ (Storage: %s)", rootDir)
}

// renderWebDAVBrowserInfo serves a responsive landing page when a user opens /webdav in a browser
func renderWebDAVBrowserInfo(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	mountURL := fmt.Sprintf("%s://%s/webdav", scheme, host)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="id">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>GoNAS - WebDAV Network Storage</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background: #0f172a; color: #f8fafc; margin: 0; padding: 2rem; display: flex; justify-content: center; align-items: center; min-height: 90vh; }
        .card { background: #1e293b; border: 1px solid #334155; border-radius: 16px; padding: 2.25rem; max-width: 600px; width: 100%%; box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.5); }
        .badge { display: inline-block; padding: 4px 12px; background: rgba(59, 130, 246, 0.15); border: 1px solid rgba(59, 130, 246, 0.3); border-radius: 9999px; color: #60a5fa; font-size: 0.85rem; font-weight: 600; margin-bottom: 1rem; }
        h1 { font-size: 1.6rem; margin-top: 0; margin-bottom: 0.75rem; color: #ffffff; }
        p { color: #94a3b8; font-size: 0.95rem; line-height: 1.6; margin-bottom: 1.5rem; }
        .btn { display: inline-flex; align-items: center; justify-content: center; width: 100%%; box-sizing: border-box; background: #2563eb; color: #ffffff; text-decoration: none; padding: 12px 24px; border-radius: 8px; font-weight: 600; font-size: 1rem; transition: background 0.2s; text-align: center; }
        .btn:hover { background: #1d4ed8; }
        .guide-box { background: #0f172a; border: 1px solid #334155; border-radius: 8px; padding: 1.25rem; margin-top: 1.5rem; font-size: 0.88rem; }
        .guide-title { font-weight: 600; color: #e2e8f0; margin-bottom: 0.5rem; }
        .code-block { background: #1e293b; padding: 6px 10px; border-radius: 6px; color: #38bdf8; font-family: monospace; font-size: 0.85rem; word-break: break-all; margin: 4px 0; }
        ul { margin: 0.5rem 0; padding-left: 1.25rem; color: #cbd5e1; }
        li { margin-bottom: 0.45rem; }
    </style>
</head>
<body>
    <div class="card">
        <span class="badge">🌐 WebDAV Storage Endpoint</span>
        <h1>GoNAS WebDAV Storage</h1>
        <p>Anda sedang membuka endpoint <code>/webdav/</code> melalui Web Browser. URL ini dirancang khusus untuk dipetakan sebagai <strong>Network Drive</strong> di komputer atau aplikasi File Manager, bukan untuk navigasi web.</p>
        
        <a href="/" class="btn">🚀 Buka Dashboard Web GoNAS</a>
        
        <div class="guide-box">
            <div class="guide-title">📌 Cara Menghubungkan Network Drive:</div>
            <ul>
                <li><strong>Windows Explorer:</strong> Buka <em>This PC</em> &rarr; klik <em>Map Network Drive</em> &rarr; masukkan Folder: <div class="code-block">%s</div></li>
                <li><strong>macOS Finder:</strong> <em>Go</em> &rarr; <em>Connect to Server (Cmd+K)</em> &rarr; masukkan URL di atas.</li>
                <li><strong>Android / iOS:</strong> Gunakan aplikasi Solid Explorer, Cx File Explorer, atau Owlfiles.</li>
                <li><strong>Kredensial:</strong> Gunakan username &amp; password akun GoNAS Anda (misal: <code>admin</code> / <code>admin123</code>).</li>
            </ul>
        </div>
    </div>
</body>
</html>`, mountURL)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}
