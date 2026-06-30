package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"quakemap-backend-go/internal/config"
	"quakemap-backend-go/internal/dmdata"
	"quakemap-backend-go/internal/model"
	"quakemap-backend-go/internal/store"
)

func testServer(t *testing.T, debug bool) (*httptest.Server, *store.Store) {
	t.Helper()
	s := store.New()
	cfg := config.Config{Debug: debug, StaticDir: filepath.Join("..", "..", "static"), DebugAssetsDir: filepath.Join("..", "..", "test", "assets"), CentroidDir: filepath.Join("..", "..", "assets", "centroid")}
	p := dmdata.NewProcessor(s, cfg.CentroidDir)
	server := httptest.NewServer(New(cfg, s, p).Handler())
	t.Cleanup(server.Close)
	return server, s
}
func get(t *testing.T, url string) (*http.Response, []byte) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	err = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	return resp, body
}

func TestIndexAndTime(t *testing.T) {
	srv, _ := testServer(t, false)
	resp, body := get(t, srv.URL+"/")
	if resp.StatusCode != 200 || string(body) != "It works!" {
		t.Fatalf("status=%d body=%q", resp.StatusCode, body)
	}
	ct := time.Now().UnixMilli()
	resp, body = get(t, srv.URL+"/time?ct="+jsonNumber(ct))
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	var sync model.TimeSync
	if err := json.Unmarshal(body, &sync); err != nil {
		t.Fatal(err)
	}
	if sync.ServerTimestamp-ct != sync.Difference {
		t.Fatalf("bad difference: %+v", sync)
	}
}
func TestDefaultAPIModels(t *testing.T) {
	srv, _ := testServer(t, false)
	for _, path := range []string{"/heartbeat/dmdata", "/api/earthquake_info", "/api/is_tsunami", "/api/tsunami_info", "/api/raw_data"} {
		resp, _ := get(t, srv.URL+path)
		if resp.StatusCode != 200 {
			t.Errorf("%s status %d", path, resp.StatusCode)
		}
	}
	resp, body := get(t, srv.URL+"/api/shake_level")
	if resp.StatusCode != 404 || !strings.Contains(string(body), "API not yet ready") {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
}
func TestShakeAndStatic(t *testing.T) {
	srv, s := testServer(t, false)
	s.SetShake(model.ShakeLevel{ShakeLevel: 50, Green: 1, Yellow: 2, Red: 3, Status: 0})
	resp, body := get(t, srv.URL+"/api/shake_level")
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"shake_level":50`) {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	resp, body = get(t, srv.URL+"/static/geojson/japan.json")
	if resp.StatusCode != 200 || len(body) < 100 {
		t.Fatalf("static status=%d bytes=%d", resp.StatusCode, len(body))
	}
}
func TestDebugEndpoints(t *testing.T) {
	srv, _ := testServer(t, true)
	lists := []string{"forecast", "warning", "tsunami_expectation", "tsunami_watch"}
	var forecastFiles []string
	for _, kind := range lists {
		resp, body := get(t, srv.URL+"/debug/dmdata/"+kind+"/list")
		if resp.StatusCode != 200 {
			t.Fatalf("%s: status=%d body=%s", kind, resp.StatusCode, body)
		}
		var files []string
		if err := json.Unmarshal(body, &files); err != nil {
			t.Fatal(err)
		}
		if kind == "forecast" {
			forecastFiles = files
		}
	}
	resp, body := get(t, srv.URL+"/debug/dmdata/forecast/manual/"+forecastFiles[0])
	if resp.StatusCode != 200 {
		t.Fatalf("manual: status=%d body=%s", resp.StatusCode, body)
	}
	for _, path := range []string{"/debug/dmdata/forecast/cycle", "/debug/dmdata/warning/cycle", "/debug/dmdata/tsunami_expectation/cycle", "/debug/dmdata/tsunami_watch/cycle", "/debug/dmdata/file/cycle", "/debug/eew/clear", "/debug/dmdata/start_cycle?task=forecast&seconds=60", "/debug/dmdata/end_cycle"} {
		resp, body = get(t, srv.URL+path)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: status=%d body=%s", path, resp.StatusCode, body)
		}
	}
	resp, _ = get(t, srv.URL+"/debug/p2p/ScalePrompt")
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected disabled P2P endpoint, got %d", resp.StatusCode)
	}
}
func TestDebugDisabled(t *testing.T) {
	srv, _ := testServer(t, false)
	resp, body := get(t, srv.URL+"/debug/dmdata/forecast/list")
	if resp.StatusCode != 404 {
		t.Fatalf("got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), `"status":-4`) {
		t.Fatalf("unexpected not-found body: %s", body)
	}
}
func jsonNumber(v int64) string {
	return json.Number(strings.TrimSpace(strings.TrimSpace(formatInt(v)))).String()
}
func formatInt(v int64) string {
	if v == 0 {
		return "0"
	}
	negative := v < 0
	if negative {
		v = -v
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if negative {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
