package kmoni

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	"quakemap-backend-go/internal/config"
	"quakemap-backend-go/internal/model"
	"quakemap-backend-go/internal/store"
)

type Client struct {
	url      string
	interval time.Duration
	http     *http.Client
	store    *store.Store
}

var kmoniLog = logrus.WithField("prefix", "kmoni")

func NewClient(cfg config.Config, s *store.Store) *Client {
	return &Client{url: cfg.KMoniURL, interval: cfg.KMoniInterval, http: &http.Client{Timeout: cfg.HTTPTimeout}, store: s}
}

func (c *Client) Run(ctx context.Context) {
	kmoniLog.WithFields(logrus.Fields{"interval": c.interval, "url": c.url}).Debug("starting KMoni shake-level poller")
	c.refresh(ctx)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			kmoniLog.Debug("stopping KMoni shake-level poller")
			return
		case <-ticker.C:
			c.refresh(ctx)
		}
	}
}

func (c *Client) refresh(ctx context.Context) {
	if err := c.Refresh(ctx); err != nil {
		kmoniLog.WithError(err).Warn("failed to refresh KMoni shake level")
	}
}

func (c *Client) Refresh(ctx context.Context) error {
	started := time.Now()
	kmoniLog.Trace("requesting KMoni shake level")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("KMoni shake level: HTTP %s", resp.Status)
	}
	var wire struct {
		Level  int  `json:"l"`
		Green  int  `json:"g"`
		Yellow int  `json:"y"`
		Red    int  `json:"r"`
		Status *int `json:"e"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return err
	}
	c.store.SetShake(model.ShakeLevel{ShakeLevel: wire.Level, Green: wire.Green, Yellow: wire.Yellow, Red: wire.Red, Status: 0})
	kmoniLog.WithFields(logrus.Fields{
		"elapsed":     time.Since(started),
		"shake_level": wire.Level,
		"green":       wire.Green,
		"yellow":      wire.Yellow,
		"red":         wire.Red,
	}).Trace("updated KMoni shake-level state")
	return nil
}
