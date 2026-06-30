package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"quakemap-backend-go/internal/dmdata"
	"quakemap-backend-go/internal/model"
	"quakemap-backend-go/internal/store"
)

type debugAPI struct {
	assets      string
	processor   *dmdata.Processor
	store       *store.Store
	mu          sync.Mutex
	files       map[string][]string
	indices     map[string]int
	cycleCancel context.CancelFunc
}

var assetMap = map[string]string{
	"forecast":            "eew_forecast",
	"warning":             "eew_warning",
	"tsunami_expectation": "tsunami_expectation",
	"tsunami_watch":       "tsunami_watch",
	"file":                "raw_messages",
}

var assetTypeMap = map[string]string{
	"forecast":            dmdata.TypeEEWForecast,
	"warning":             dmdata.TypeEEWWarning,
	"tsunami_expectation": dmdata.TypeTsunami,
	"tsunami_watch":       dmdata.TypeTsunamiInfo,
}

func newDebugAPI(assets string, p *dmdata.Processor, s *store.Store) *debugAPI {
	d := &debugAPI{assets: assets, processor: p, store: s, files: map[string][]string{}, indices: map[string]int{}}
	counts := logrus.Fields{"assets": assets}
	for key, dir := range assetMap {
		d.files[key] = listFiles(filepath.Join(assets, dir))
		counts[key] = len(d.files[key])
	}
	apiLog.WithFields(counts).Debug("loaded debug fixture catalog")
	return d
}

func (d *debugAPI) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /debug/dmdata/forecast/list", d.list("forecast"))
	mux.HandleFunc("GET /debug/dmdata/warning/list", d.list("warning"))
	mux.HandleFunc("GET /debug/dmdata/tsunami_expectation/list", d.list("tsunami_expectation"))
	mux.HandleFunc("GET /debug/dmdata/tsunami_watch/list", d.list("tsunami_watch"))
	mux.HandleFunc("GET /debug/dmdata/{parse_type}/manual/{id}", d.manual)
	mux.HandleFunc("GET /debug/dmdata/forecast/cycle", d.cycle("forecast"))
	mux.HandleFunc("GET /debug/dmdata/warning/cycle", d.cycle("warning"))
	mux.HandleFunc("GET /debug/dmdata/tsunami_expectation/cycle", d.cycle("tsunami_expectation"))
	mux.HandleFunc("GET /debug/dmdata/tsunami_watch/cycle", d.cycle("tsunami_watch"))
	mux.HandleFunc("GET /debug/dmdata/file/cycle", d.cycle("file"))
	mux.HandleFunc("GET /debug/eew/clear", func(w http.ResponseWriter, _ *http.Request) {
		d.store.ClearEEW()
		writeJSON(w, http.StatusOK, model.GenericResponse{Status: 0, Data: "OK"})
	})
	mux.HandleFunc("GET /debug/p2p/{info_type}", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotImplemented, model.GenericResponse{Status: -5, Data: "P2P ingestion is disabled"})
	})
	mux.HandleFunc("GET /debug/dmdata/start_cycle", d.startCycle)
	mux.HandleFunc("GET /debug/dmdata/end_cycle", d.endCycle)
}

func (d *debugAPI) list(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, d.files[kind]) }
}

func (d *debugAPI) manual(w http.ResponseWriter, r *http.Request) {
	kind, id := r.PathValue("parse_type"), r.PathValue("id")
	if kind == "file" || !contains(d.files[kind], id) {
		apiLog.WithFields(logrus.Fields{"kind": kind, "fixture": id}).Debug("rejected unknown debug fixture")
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": -5, "data": "ID not found"})
		return
	}
	if err := d.parse(kind, id); err != nil {
		apiLog.WithFields(logrus.Fields{"kind": kind, "fixture": id}).WithError(err).Warn("failed to parse debug fixture")
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": -5, "data": err.Error()})
		return
	}
	apiLog.WithFields(logrus.Fields{"kind": kind, "fixture": id}).Debug("parsed debug fixture manually")
	writeJSON(w, http.StatusOK, map[string]any{"status": 0, "current_file": id})
}

func (d *debugAPI) cycle(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		d.mu.Lock()
		files := d.files[kind]
		if len(files) == 0 {
			d.mu.Unlock()
			writeJSON(w, http.StatusNotFound, model.GenericResponse{Status: -1, Data: "API not yet ready"})
			return
		}
		i := d.indices[kind] % len(files)
		id := files[i]
		d.indices[kind] = (i + 1) % len(files)
		d.mu.Unlock()
		d.store.ClearEEW()
		if err := d.parse(kind, id); err != nil {
			apiLog.WithFields(logrus.Fields{"kind": kind, "fixture": id}).WithError(err).Warn("failed to parse cycled debug fixture")
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": -5, "data": err.Error()})
			return
		}
		apiLog.WithFields(logrus.Fields{"kind": kind, "fixture": id}).Debug("cycled debug fixture")
		key := "current_forecast"
		if kind == "file" {
			key = "current_file"
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": 0, key: id})
	}
}

func (d *debugAPI) parse(kind, id string) error {
	dir, typ := assetMap[kind], assetTypeMap[kind]
	path := filepath.Join(d.assets, dir, id)
	apiLog.WithFields(logrus.Fields{"kind": kind, "fixture": id, "telegram": typ}).Debug("reading debug fixture")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if kind == "file" {
		typ = typeFromFilename(id)
		if typ == "" {
			return fmt.Errorf("unidentified DMData telegram: %s", id)
		}
		apiLog.WithFields(logrus.Fields{"fixture": id, "telegram": typ, "bytes": len(raw)}).Debug("parsing raw debug fixture")
		return d.processor.ParseJSONReport(typ, raw)
	}
	apiLog.WithFields(logrus.Fields{"fixture": id, "telegram": typ, "bytes": len(raw)}).Debug("parsing XML debug fixture")
	return d.processor.ParseXML(typ, raw, true)
}

func (d *debugAPI) startCycle(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("task")
	if _, ok := d.files[kind]; !ok {
		writeJSON(w, http.StatusBadRequest, model.GenericResponse{Status: -5, Data: "Bad request"})
		return
	}
	seconds := 2.0
	if v := r.URL.Query().Get("seconds"); v != "" {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil || parsed <= 0 {
			writeJSON(w, http.StatusBadRequest, model.GenericResponse{Status: -5, Data: "Bad request"})
			return
		}
		seconds = parsed
	}
	d.mu.Lock()
	if d.cycleCancel != nil {
		d.cycleCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.cycleCancel = cancel
	d.mu.Unlock()
	apiLog.WithFields(logrus.Fields{"kind": kind, "interval_seconds": seconds}).Debug("started debug fixture cycle")
	go func() {
		ticker := time.NewTicker(time.Duration(seconds * float64(time.Second)))
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.runCycle(kind)
			}
		}
	}()
	writeJSON(w, http.StatusOK, model.GenericResponse{Status: 0, Data: "OK"})
}
func (d *debugAPI) runCycle(kind string) {
	d.mu.Lock()
	files := d.files[kind]
	if len(files) == 0 {
		d.mu.Unlock()
		return
	}
	i := d.indices[kind] % len(files)
	id := files[i]
	d.indices[kind] = (i + 1) % len(files)
	d.mu.Unlock()
	d.store.ClearEEW()
	if err := d.parse(kind, id); err != nil {
		apiLog.WithFields(logrus.Fields{"kind": kind, "fixture": id}).WithError(err).Warn("debug fixture cycle failed")
		return
	}
	apiLog.WithFields(logrus.Fields{"kind": kind, "fixture": id}).Debug("debug fixture cycle advanced")
}
func (d *debugAPI) endCycle(w http.ResponseWriter, _ *http.Request) {
	d.mu.Lock()
	if d.cycleCancel != nil {
		d.cycleCancel()
		d.cycleCancel = nil
	}
	d.mu.Unlock()
	apiLog.Debug("stopped debug fixture cycle")
	writeJSON(w, http.StatusOK, model.GenericResponse{Status: 0, Data: "OK"})
}

func listFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		apiLog.WithField("directory", dir).WithError(err).Warn("failed to list debug fixtures")
		return []string{}
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}
func contains(items []string, want string) bool {
	i := sort.SearchStrings(items, want)
	return i < len(items) && items[i] == want
}
func typeFromFilename(name string) string {
	for _, t := range dmdata.SupportedTypes {
		if strings.Contains(name, t) {
			return t
		}
	}
	return ""
}
