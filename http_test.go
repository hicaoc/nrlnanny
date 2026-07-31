package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func preserveControlConfig(t *testing.T) {
	t.Helper()
	enabled := conf.System.EnableControlPage
	username := conf.System.ControlUsername
	password := conf.System.ControlPassword
	t.Cleanup(func() {
		conf.System.EnableControlPage = enabled
		conf.System.ControlUsername = username
		conf.System.ControlPassword = password
	})
}

func authenticatedRequest(t *testing.T, method, target string) *http.Request {
	t.Helper()
	token, err := newControlSession()
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, target, nil)
	request.AddCookie(&http.Cookie{Name: controlSessionCookie, Value: token})
	return request
}

func TestHomepageIsAlwaysLive(t *testing.T) {
	preserveControlConfig(t)
	for _, enabled := range []bool{false, true} {
		conf.System.EnableControlPage = enabled
		response := httptest.NewRecorder()
		serveIndex(response, httptest.NewRequest(http.MethodGet, "/", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("enabled=%v status=%d, want 200", enabled, response.Code)
		}
		if !strings.Contains(response.Body.String(), `data-page-title="live"`) {
			t.Fatalf("enabled=%v homepage is not Live", enabled)
		}
	}
}

func TestControlPageRequiresSession(t *testing.T) {
	preserveControlConfig(t)
	conf.System.EnableControlPage = true

	response := httptest.NewRecorder()
	serveControl(response, httptest.NewRequest(http.MethodGet, "/control", nil))
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login?next=/control" {
		t.Fatalf("unauthenticated response = %d %q", response.Code, response.Header().Get("Location"))
	}

	response = httptest.NewRecorder()
	serveControl(response, authenticatedRequest(t, http.MethodGet, "/control"))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `data-page-title="dashboard"`) {
		t.Fatalf("authenticated control response = %d", response.Code)
	}
}

func TestControlPageOnlyProtectsManagementAPI(t *testing.T) {
	preserveControlConfig(t)
	handler := controlPageOnly(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	conf.System.EnableControlPage = false
	response := httptest.NewRecorder()
	handler(response, httptest.NewRequest(http.MethodGet, "/api/music", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("disabled status = %d, want 404", response.Code)
	}

	conf.System.EnableControlPage = true
	response = httptest.NewRecorder()
	handler(response, httptest.NewRequest(http.MethodGet, "/api/music", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", response.Code)
	}

	response = httptest.NewRecorder()
	handler(response, authenticatedRequest(t, http.MethodGet, "/api/music"))
	if response.Code != http.StatusNoContent {
		t.Fatalf("authenticated status = %d, want 204", response.Code)
	}
}

func TestControlLoginCreatesSession(t *testing.T) {
	preserveControlConfig(t)
	conf.System.EnableControlPage = true
	conf.System.ControlUsername = "operator"
	conf.System.ControlPassword = "secret"

	form := url.Values{"username": {"operator"}, "password": {"secret"}, "next": {"/control"}}
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	serveLogin(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/control" {
		t.Fatalf("login response = %d %q", response.Code, response.Header().Get("Location"))
	}
	result := response.Result()
	var session *http.Cookie
	for _, cookie := range result.Cookies() {
		if cookie.Name == controlSessionCookie {
			session = cookie
			break
		}
	}
	if session == nil || session.Value == "" || !session.HttpOnly {
		t.Fatal("login did not create an HttpOnly session cookie")
	}
	check := httptest.NewRequest(http.MethodGet, "/control", nil)
	check.AddCookie(session)
	if !controlAuthenticated(check) {
		t.Fatal("new login session is not authenticated")
	}
}

func TestControlLoginRejectsInvalidCredentialsAndUnsafeRedirect(t *testing.T) {
	preserveControlConfig(t)
	conf.System.EnableControlPage = true
	conf.System.ControlUsername = "operator"
	conf.System.ControlPassword = "secret"

	form := url.Values{"username": {"operator"}, "password": {"wrong"}, "next": {"https://example.com/"}}
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	serveLogin(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login?error=1&next=%2Fcontrol" {
		t.Fatalf("invalid login response = %d %q", response.Code, response.Header().Get("Location"))
	}
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == controlSessionCookie && cookie.Value != "" {
			t.Fatal("invalid login created a session cookie")
		}
	}
}

func TestLogoutInvalidatesSession(t *testing.T) {
	preserveControlConfig(t)
	conf.System.EnableControlPage = true
	request := authenticatedRequest(t, http.MethodPost, "/logout")
	cookie, err := request.Cookie(controlSessionCookie)
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	serveLogout(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" {
		t.Fatalf("logout response = %d %q", response.Code, response.Header().Get("Location"))
	}

	check := httptest.NewRequest(http.MethodGet, "/control", nil)
	check.AddCookie(cookie)
	if controlAuthenticated(check) {
		t.Fatal("logged-out session is still authenticated")
	}
}

func TestRecordingsPageIsPublic(t *testing.T) {
	preserveControlConfig(t)
	for _, enabled := range []bool{false, true} {
		conf.System.EnableControlPage = enabled
		response := httptest.NewRecorder()
		servePlay(response, httptest.NewRequest(http.MethodGet, "/play", nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `data-page-title="browser"`) {
			t.Fatalf("enabled=%v recordings response = %d", enabled, response.Code)
		}
	}
}

func TestLiveMultUsesCurrentServer(t *testing.T) {
	originalServer := conf.System.Server
	t.Cleanup(func() { conf.System.Server = originalServer })

	for _, test := range []struct {
		server string
		want   string
	}{
		{server: "rooms.example.com", want: "wss://rooms.example.com/ws/calls"},
		{server: "https://rooms.example.net/ignored/path", want: "wss://rooms.example.net/ws/calls"},
	} {
		conf.System.Server = test.server
		response := httptest.NewRecorder()
		apiLiveMultConfig(response, httptest.NewRequest(http.MethodGet, "/api/live-mult-config", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("server=%q status=%d", test.server, response.Code)
		}
		var result map[string]string
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if result["ws_url"] != test.want {
			t.Fatalf("server=%q ws_url=%q, want %q", test.server, result["ws_url"], test.want)
		}
	}
}
