package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	controlSessionCookie = "nrlnanny_control_session"
	controlSessionTTL    = 12 * time.Hour
)

var controlSessions = struct {
	sync.Mutex
	tokens map[string]time.Time
}{tokens: make(map[string]time.Time)}

func controlAuthenticated(r *http.Request) bool {
	if !conf.System.EnableControlPage {
		return false
	}
	cookie, err := r.Cookie(controlSessionCookie)
	if err != nil || cookie.Value == "" {
		return false
	}
	now := time.Now()
	controlSessions.Lock()
	expires, ok := controlSessions.tokens[cookie.Value]
	if ok && now.After(expires) {
		delete(controlSessions.tokens, cookie.Value)
		ok = false
	}
	controlSessions.Unlock()
	return ok
}

func controlCredentialsValid(username, password string) bool {
	expectedUser := conf.System.ControlUsername
	expectedPassword := conf.System.ControlPassword
	if expectedUser == "" || expectedPassword == "" {
		return false
	}
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(expectedUser))
	passwordOK := subtle.ConstantTimeCompare([]byte(password), []byte(expectedPassword))
	return userOK&passwordOK == 1
}

func newControlSession() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	controlSessions.Lock()
	controlSessions.tokens[token] = time.Now().Add(controlSessionTTL)
	controlSessions.Unlock()
	return token, nil
}

func setControlSessionCookie(w http.ResponseWriter, r *http.Request, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     controlSessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
		SameSite: http.SameSiteStrictMode,
	})
}

func serveLogin(w http.ResponseWriter, r *http.Request) {
	if !conf.System.EnableControlPage {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		if controlAuthenticated(r) {
			http.Redirect(w, r, "/control", http.StatusSeeOther)
			return
		}
		content, err := webAssets.ReadFile("login.html")
		if err != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(content)

	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		next := safeControlRedirect(r.FormValue("next"))
		if !controlCredentialsValid(r.FormValue("username"), r.FormValue("password")) {
			http.Redirect(w, r, "/login?error=1&next="+url.QueryEscape(next), http.StatusSeeOther)
			return
		}
		token, err := newControlSession()
		if err != nil {
			http.Error(w, "Could not create session", http.StatusInternalServerError)
			return
		}
		setControlSessionCookie(w, r, token, int(controlSessionTTL/time.Second))
		http.Redirect(w, r, next, http.StatusSeeOther)

	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func serveLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if cookie, err := r.Cookie(controlSessionCookie); err == nil {
		controlSessions.Lock()
		delete(controlSessions.tokens, cookie.Value)
		controlSessions.Unlock()
	}
	setControlSessionCookie(w, r, "", -1)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func safeControlRedirect(_ string) string {
	return "/control"
}
