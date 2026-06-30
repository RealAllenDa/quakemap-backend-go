package api

import (
	"encoding/json"
	"net/http"
	runtimedebug "runtime/debug"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"

	"quakemap-backend-go/internal/config"
	"quakemap-backend-go/internal/dmdata"
	"quakemap-backend-go/internal/model"
	"quakemap-backend-go/internal/store"
)

type Server struct{ handler http.Handler }

var apiLog = logrus.WithField("prefix", "api")

func New(cfg config.Config, s *store.Store, p *dmdata.Processor) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("It works!"))
	})
	mux.HandleFunc("GET /time", func(w http.ResponseWriter, r *http.Request) {
		ct, err := strconv.ParseInt(r.URL.Query().Get("ct"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, model.GenericResponse{Status: -5, Data: "Bad request"})
			return
		}
		now := time.Now().UnixMilli()
		writeJSON(w, http.StatusOK, model.TimeSync{ServerTimestamp: now, Difference: now - ct})
	})
	mux.HandleFunc("GET /heartbeat/dmdata", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, s.DMDataStatus(time.Now())) })
	mux.HandleFunc("GET /api/earthquake_info", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, s.EarthquakeInfo()) })
	mux.HandleFunc("GET /api/raw_data", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(s.RawData()))
	})
	mux.HandleFunc("GET /api/shake_level", func(w http.ResponseWriter, _ *http.Request) {
		value, ok := s.Shake()
		if !ok {
			writeJSON(w, http.StatusNotFound, model.GenericResponse{Status: -1, Data: "API not yet ready"})
			return
		}
		writeJSON(w, http.StatusOK, value)
	})
	mux.HandleFunc("GET /api/is_tsunami", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if s.IsTsunami() {
			_, _ = w.Write([]byte("1"))
		} else {
			_, _ = w.Write([]byte("0"))
		}
	})
	mux.HandleFunc("GET /api/tsunami_info", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, s.Tsunami()) })
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(cfg.StaticDir))))
	if cfg.Debug {
		newDebugAPI(cfg.DebugAssetsDir, p, s).register(mux)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotFound, model.GenericResponse{Status: -4, Data: "Not found"})
	})
	return &Server{handler: middleware(mux)}
}

func (s *Server) Handler() http.Handler { return s.handler }

func middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			apiLog.WithFields(logrus.Fields{"method": r.Method, "path": r.URL.Path, "status": http.StatusNoContent, "elapsed": time.Since(started)}).Debug("completed HTTP request")
			return
		}
		rw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if recovered := recover(); recovered != nil {
				apiLog.WithFields(logrus.Fields{
					"method": r.Method,
					"path":   r.URL.Path,
					"panic":  recovered,
					"stack":  string(runtimedebug.Stack()),
				}).Error("HTTP handler panic")
				writeJSON(rw, http.StatusInternalServerError, model.GenericResponse{Status: -2, Data: "Internal server error"})
			}
			apiLog.WithFields(logrus.Fields{
				"method":  r.Method,
				"path":    r.URL.Path,
				"status":  rw.status,
				"bytes":   rw.bytes,
				"elapsed": time.Since(started),
			}).Trace("completed HTTP request")
		}()
		next.ServeHTTP(rw, r)
		if rw.status == http.StatusNotFound && rw.bytes == 0 {
			writeJSON(w, http.StatusNotFound, model.GenericResponse{Status: -4, Data: "Not found"})
		}
	})
}

type statusWriter struct {
	http.ResponseWriter
	status, bytes int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *statusWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
