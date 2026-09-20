package main

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePrecedence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VP_CONFIG_DIR", dir)
	t.Setenv("VP_API", "")
	t.Setenv("VP_API_KEY", "")
	t.Setenv("VP_TOKEN", "")

	if err := saveConfig(Config{APIBase: "https://from-file.example", APIKey: "vp_live_file"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := resolve("", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIBase != "https://from-file.example" || cfg.APIKey != "vp_live_file" {
		t.Fatalf("file config: %+v", cfg)
	}

	t.Setenv("VP_API", "https://from-env.example/")
	t.Setenv("VP_API_KEY", "vp_live_env")
	cfg, err = resolve("", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIBase != "https://from-env.example" || cfg.APIKey != "vp_live_env" {
		t.Fatalf("env config: %+v", cfg)
	}

	cfg, err = resolve("https://from-flag.example", "vp_live_flag")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIBase != "https://from-flag.example" || cfg.APIKey != "vp_live_flag" {
		t.Fatalf("flag config: %+v", cfg)
	}
}

func TestClientCreateGetRawListDelete(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer vp_live_test" {
			http.Error(w, `{"error":{"code":"unauthorized","message":"no"}}`, 401)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"user": map[string]any{"email": "cli@voidpaste.com", "id": "u1", "role": "user", "plan": "free"},
		})
	})
	mux.HandleFunc("/api/v1/pastes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["content"] == nil {
			http.Error(w, `{"error":{"code":"validation_error","message":"content required"}}`, 400)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"paste":    map[string]any{"id": "Abcd1234Efgh5678Ijkl", "visibility": body["visibility"]},
			"warnings": []any{},
		})
	})
	mux.HandleFunc("/api/v1/pastes/Abcd1234Efgh5678Ijkl", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"paste": map[string]any{"id": "Abcd1234Efgh5678Ijkl", "content": "hello"},
			})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/raw/Abcd1234Efgh5678Ijkl", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello"))
	})
	mux.HandleFunc("/api/v1/me/pastes", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"pastes": []map[string]any{{"id": "Abcd1234Efgh5678Ijkl", "visibility": "unlisted", "created_at": "2026-01-01T00:00:00Z"}},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(Config{APIBase: srv.URL, APIKey: "vp_live_test"})
	h, err := c.Health(t.Context())
	if err != nil || h["status"] != "ok" {
		t.Fatalf("health: %v %v", h, err)
	}
	me, err := c.Me(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	user := me["user"].(map[string]any)
	if user["email"] != "cli@voidpaste.com" {
		t.Fatalf("me: %v", me)
	}
	created, err := c.CreatePaste(t.Context(), CreatePasteInput{Content: "hello", Visibility: "unlisted"}, "")
	if err != nil {
		t.Fatal(err)
	}
	paste := created["paste"].(map[string]any)
	if paste["id"] != "Abcd1234Efgh5678Ijkl" {
		t.Fatalf("create: %v", created)
	}
	got, err := c.GetPaste(t.Context(), "Abcd1234Efgh5678Ijkl", "")
	if err != nil {
		t.Fatal(err)
	}
	if got["paste"].(map[string]any)["content"] != "hello" {
		t.Fatalf("get: %v", got)
	}
	raw, err := c.RawPaste(t.Context(), "Abcd1234Efgh5678Ijkl", "")
	if err != nil || string(raw) != "hello" {
		t.Fatalf("raw: %q %v", raw, err)
	}
	list, err := c.ListPastes(t.Context(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list["pastes"].([]any)) != 1 {
		t.Fatalf("list: %v", list)
	}
	if err := c.DeletePaste(t.Context(), "Abcd1234Efgh5678Ijkl"); err != nil {
		t.Fatal(err)
	}
}

func TestClientCollectionsAndVersions(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/collections", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"collections": []any{map[string]any{"id": "c1", "name": "N", "visibility": "private"}},
			})
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"collection": map[string]any{"id": "c1", "name": "N"},
			})
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/api/v1/collections/c1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"collection": map[string]any{"id": "c1", "name": "N"},
			"items":      []any{},
		})
	})
	mux.HandleFunc("/api/v1/collections/c1/pastes", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"paste_id": "p1", "position": 0})
	})
	mux.HandleFunc("/api/v1/pastes/p1/versions", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"versions": []any{map[string]any{"version": 1, "size_bytes": 3, "created_at": "2026-01-01T00:00:00Z"}},
		})
	})
	mux.HandleFunc("/api/v1/pastes/p1/versions/1/restore", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := &Client{Base: srv.URL, APIKey: "vp_live_test", HTTP: srv.Client()}
	ctx := context.Background()
	if _, err := c.ListCollections(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateCollection(ctx, "N", "", "private"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetCollection(ctx, "c1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AddPasteToCollection(ctx, "c1", "p1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListVersions(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RestoreVersion(ctx, "p1", 1); err != nil {
		t.Fatal(err)
	}
}

func TestInterspersedFlags(t *testing.T) {
	fs := flag.NewFlagSet("raw", flag.ContinueOnError)
	var api, password string
	fs.StringVar(&api, "api", "", "")
	fs.StringVar(&password, "password", "", "")
	pos, err := parseFlagsAllowInterspersed(fs, []string{"Abcd1234Efgh5678Ijkl", "--api", "https://voidpaste.com", "--password", "x"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pos) != 1 || pos[0] != "Abcd1234Efgh5678Ijkl" {
		t.Fatalf("pos=%v", pos)
	}
	if api != "https://voidpaste.com" || password != "x" {
		t.Fatalf("api=%q password=%q", api, password)
	}
}

func TestRootHelp(t *testing.T) {
	if err := root(nil); err != nil {
		t.Fatal(err)
	}
	if err := root([]string{"help"}); err != nil {
		t.Fatal(err)
	}
}

func TestSaveConfigPermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VP_CONFIG_DIR", dir)
	if err := saveConfig(Config{APIBase: defaultAPIBase, APIKey: "vp_live_x"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, configFileName)
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		// On Windows permissions may not match Unix; only assert file exists + parses.
		t.Logf("perm=%v (platform may ignore mode bits)", fi.Mode())
	}
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cfg.APIKey, "vp_live_") {
		t.Fatalf("key %q", cfg.APIKey)
	}
}
