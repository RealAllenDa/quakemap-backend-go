package main

import (
	"context"
	"errors"
	stdlog "log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	"quakemap-backend-go/internal/api"
	"quakemap-backend-go/internal/config"
	"quakemap-backend-go/internal/dmdata"
	"quakemap-backend-go/internal/kmoni"
	"quakemap-backend-go/internal/logging"
	"quakemap-backend-go/internal/persist"
	"quakemap-backend-go/internal/store"
)

func main() {
	_ = godotenv.Load()

	logging.Configure()
	cfg := config.FromEnv()
	appLog := logrus.WithField("prefix", "app")
	appLog.WithFields(logrus.Fields{
		"address":          cfg.Address,
		"debug":            cfg.Debug,
		"dmdata_enabled":   cfg.DMDataEnabled,
		"kmoni_interval":   cfg.KMoniInterval,
		"static_dir":       cfg.StaticDir,
		"centroid_dir":     cfg.CentroidDir,
		"debug_assets_dir": cfg.DebugAssetsDir,
		"persist_dir":      cfg.PersistDir,
	}).Debug("loaded application configuration")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	state := store.New()
	stateRepository, err := persist.New(cfg.PersistDir)
	if err != nil {
		appLog.WithError(err).Error("API state persistence is unavailable")
	} else {
		saved, exists, loadErr := stateRepository.Load()
		switch {
		case loadErr != nil:
			appLog.WithField("path", stateRepository.Path()).WithError(loadErr).Warn("failed to restore persisted API state")
		case exists:
			state.RestoreAPIState(saved.EarthquakeInfo, saved.TsunamiInfo)
			appLog.WithFields(logrus.Fields{
				"path":        stateRepository.Path(),
				"saved_at":    saved.SavedAt,
				"earthquakes": len(saved.EarthquakeInfo.Info),
			}).Info("restored persisted API state")
		default:
			appLog.WithField("path", stateRepository.Path()).Debug("no persisted API state found")
		}
	}
	processor := dmdata.NewProcessor(state, cfg.CentroidDir)
	var background sync.WaitGroup
	background.Add(2)
	go func() {
		defer background.Done()
		dmdata.NewClient(cfg, state, processor).Run(ctx)
	}()
	go func() {
		defer background.Done()
		kmoni.NewClient(cfg, state).Run(ctx)
	}()

	logWriter := logrus.StandardLogger().Writer()
	defer logWriter.Close()
	server := &http.Server{Addr: cfg.Address, Handler: api.New(cfg, state, processor).Handler(), ErrorLog: stdlog.New(logWriter, "", 0), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 2 * time.Minute}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		appLog.Debug("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			appLog.WithError(err).Warn("HTTP server shutdown did not complete cleanly")
			return
		}
		appLog.Debug("HTTP server shutdown completed")
	}()
	appLog.WithField("address", cfg.Address).Info("quakemap backend listening")
	serveErr := server.ListenAndServe()
	stop()
	<-shutdownDone
	background.Wait()
	if stateRepository != nil {
		if err := stateRepository.Save(state.EarthquakeInfo(), state.Tsunami()); err != nil {
			appLog.WithField("path", stateRepository.Path()).WithError(err).Error("failed to persist API state")
		} else {
			appLog.WithField("path", stateRepository.Path()).Info("persisted API state")
		}
	}
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		appLog.WithError(serveErr).Error("HTTP server failed")
		os.Exit(1)
	}
}
