package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"yunjian/internal/app"
	"yunjian/internal/platform/config"
	"yunjian/internal/platform/logging"
)

func TestHTTPContract(t *testing.T) {
	engine := newEngine(t)
	register := postJSON(t, engine, http.MethodPost, "/api/account/register", map[string]any{
		"username": "linan", "password": "secret-pass", "display_name": "李楠",
	}, "")
	if register.Code != 200 || register.Body["code"] != float64(200) {
		t.Fatalf("register status=%d body=%v", register.Code, register.Body)
	}
	if _, ok := register.Body["data"].(map[string]any)["token"]; ok {
		t.Fatal("token must not be returned to the browser")
	}
	if register.Cookie == "" {
		t.Fatal("missing session cookie")
	}
	me := postJSON(t, engine, http.MethodPost, "/api/account/me", map[string]any{}, register.Cookie)
	if me.Body["code"] != float64(200) {
		t.Fatalf("me = %v", me.Body)
	}
	created := postJSON(t, engine, http.MethodPost, "/api/document/create", map[string]any{
		"kind": "ppt", "title": "发布会",
	}, register.Cookie)
	data := created.Body["data"].(map[string]any)
	if created.Body["code"] != float64(200) || data["kind"] != "ppt" {
		t.Fatalf("create = %v", created.Body)
	}
	searched := postJSON(t, engine, http.MethodPost, "/api/document/search", map[string]any{"kind": "ppt"}, register.Cookie)
	if _, ok := searched.Body["code"]; ok {
		t.Fatalf("page response should not use code envelope: %v", searched.Body)
	}
	if searched.Body["total"] != float64(1) {
		t.Fatalf("search = %v", searched.Body)
	}
	missing := postJSON(t, engine, http.MethodPost, "/api/missing", map[string]any{}, register.Cookie)
	if missing.Code != 200 || missing.Body["code"] != float64(404) {
		t.Fatalf("unknown api = %d %v", missing.Code, missing.Body)
	}
	unauth := postJSON(t, engine, http.MethodPost, "/api/document/search", map[string]any{}, "")
	if unauth.Body["code"] != float64(401) {
		t.Fatalf("unauth = %v", unauth.Body)
	}
	live := getRaw(engine, "/health/live")
	if live.Code != 200 {
		t.Fatalf("live = %d", live.Code)
	}
	ready := getRaw(engine, "/health/ready")
	if ready.Code != 200 {
		t.Fatalf("ready = %d %s", ready.Code, ready.Raw)
	}
	page := getRaw(engine, "/edit/1")
	if page.Code != 200 || !strings.Contains(page.Raw, "云笺") {
		t.Fatalf("spa = %d %s", page.Code, page.Raw)
	}
	asset := getRaw(engine, "/missing-file.js")
	if asset.Code != 404 {
		t.Fatalf("missing asset = %d", asset.Code)
	}
}

type recorded struct {
	Code   int
	Body   map[string]any
	Cookie string
	Raw    string
}

func postJSON(t *testing.T, engine *gin.Engine, method, path string, payload any, cookie string) recorded {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	result := recorded{Code: recorder.Code, Raw: recorder.Body.String(), Body: map[string]any{}}
	if recorder.Body.Len() > 0 && recorder.Body.Bytes()[0] == '{' {
		if err := json.Unmarshal(recorder.Body.Bytes(), &result.Body); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range recorder.Result().Cookies() {
		if item.Name == "yunjian_session" && item.Value != "" {
			result.Cookie = item.Name + "=" + item.Value
		}
	}
	return result
}

func getRaw(engine *gin.Engine, path string) recorded {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	return recorded{Code: recorder.Code, Raw: recorder.Body.String()}
}

func newEngine(t *testing.T) *gin.Engine {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller")
	}
	root := filepath.Dir(filepath.Dir(file))
	cfg, err := config.Load([]string{"--base-dir", root, "--data-dir", t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	logger := logging.NewDiscard()
	var engine *gin.Engine
	application := fx.New(
		fx.Supply(cfg, logger),
		app.Module,
		fx.Populate(&engine),
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
	)
	if err := application.Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = application.Stop(ctx)
	})
	return engine
}
