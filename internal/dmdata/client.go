package dmdata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"quakemap-backend-go/internal/config"
	"quakemap-backend-go/internal/store"
)

const (
	tokenEndpoint  = "https://manager.dmdata.jp/account/oauth2/v1/token"
	socketEndpoint = "https://api.dmdata.jp/v2/socket"
)

type Client struct {
	cfg       config.Config
	store     *store.Store
	processor *Processor
	http      *http.Client
}

var dmdataLog = logrus.WithField("prefix", "dmdata")

func NewClient(cfg config.Config, s *store.Store, p *Processor) *Client {
	return &Client{cfg: cfg, store: s, processor: p, http: &http.Client{Timeout: cfg.HTTPTimeout}}
}

func (c *Client) Run(ctx context.Context) {
	if !c.cfg.DMDataEnabled {
		dmdataLog.Debug("DMData client disabled by configuration")
		return
	}
	if c.cfg.DMDataRefreshToken == "" {
		dmdataLog.Warn("DMData disabled: DMDATA_REFRESH_TOKEN/REFRESH_TOKEN is not set")
		return
	}
	dmdataLog.WithFields(logrus.Fields{
		"app_name": c.cfg.DMDataAppName,
		"types":    SupportedTypes,
	}).Debug("starting DMData client")
	backoff := time.Second
	for ctx.Err() == nil {
		if err := c.connect(ctx); err != nil && !errors.Is(err, context.Canceled) {
			dmdataLog.WithError(err).Error("DMData connection ended")
		}
		c.store.SocketError()
		dmdataLog.WithField("backoff", backoff).Debug("waiting before DMData reconnect")
		select {
		case <-ctx.Done():
			dmdataLog.Debug("stopping DMData client")
			return
		case <-time.After(backoff):
		}
		if backoff < time.Minute {
			backoff *= 2
		}
	}
}

func (c *Client) connect(ctx context.Context) error {
	started := time.Now()
	dmdataLog.Debug("refreshing DMData access token")
	token, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	socket, err := c.createSocket(ctx, token)
	if err != nil {
		return err
	}
	id := strconv.FormatInt(socket.WebSocket.ID, 10)
	defer c.closeSocket(token, id)
	dmdataLog.WithFields(logrus.Fields{
		"socket_id":  id,
		"protocol":   socket.WebSocket.Protocol,
		"expiration": socket.WebSocket.Expiration,
	}).Debug("dialing allocated DMData websocket")
	ws, response, err := websocket.DefaultDialer.DialContext(ctx, socket.WebSocket.URL, nil)
	if response != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		return err
	}
	ws.SetReadLimit(16 << 20)
	defer ws.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = ws.Close()
		case <-done:
		}
	}()
	c.store.SocketConnected(id)
	dmdataLog.WithFields(logrus.Fields{"socket_id": id, "elapsed": time.Since(started)}).Info("connected to DMData websocket")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		messageType, payload, err := ws.ReadMessage()
		if err != nil {
			return err
		}
		if messageType != websocket.TextMessage {
			dmdataLog.WithField("message_type", messageType).Debug("ignored non-text websocket message")
			continue
		}
		var envelope Envelope
		if err := json.Unmarshal(payload, &envelope); err != nil {
			dmdataLog.WithError(err).Warn("invalid DMData websocket JSON")
			continue
		}
		switch envelope.Type {
		case "ping":
			var ping SocketPing
			if err := json.Unmarshal(payload, &ping); err != nil {
				dmdataLog.WithError(err).Warn("invalid DMData ping message")
				continue
			}
			pong, _ := json.Marshal(SocketPong{Type: "pong", PingID: ping.PingID})
			if err := ws.WriteMessage(websocket.TextMessage, pong); err != nil {
				return err
			}
			c.store.Pong()
			dmdataLog.WithField("ping_id", ping.PingID).Trace("answered DMData application ping")
		case "start":
			var start SocketStart
			if err := json.Unmarshal(payload, &start); err != nil {
				dmdataLog.WithError(err).Warn("invalid DMData start message")
				continue
			}
			c.store.SocketConnected(strconv.FormatInt(start.SocketID, 10))
			dmdataLog.WithFields(logrus.Fields{
				"socket_id":       start.SocketID,
				"classifications": start.Classifications,
				"types":           start.Types,
				"formats":         start.Formats,
			}).Debug("received DMData websocket start message")
		case "data":
			var data SocketData
			if err := json.Unmarshal(payload, &data); err != nil {
				dmdataLog.WithError(err).Error("invalid DMData data message")
				continue
			}
			messageStarted := time.Now()
			dmdataLog.WithFields(logrus.Fields{
				"id":             data.ID,
				"telegram":       data.Head.Type,
				"classification": data.Classification,
				"payload_bytes":  len(payload),
			}).Debug("received DMData telegram")
			if err := c.processor.ParseSocketData(data); err != nil {
				dmdataLog.WithFields(logrus.Fields{"id": data.ID, "telegram": data.Head.Type}).WithError(err).Error("failed to process DMData message")
			} else {
				dmdataLog.WithFields(logrus.Fields{"id": data.ID, "telegram": data.Head.Type, "elapsed": time.Since(messageStarted)}).Debug("processed DMData telegram")
			}
		case "error":
			var socketError SocketError
			if err := json.Unmarshal(payload, &socketError); err != nil {
				continue
			}
			if socketError.Close {
				return fmt.Errorf("DMData socket error %d: %s", socketError.Code, socketError.Error)
			}
			dmdataLog.WithFields(logrus.Fields{"code": socketError.Code, "error": socketError.Error}).Error("DMData socket error")
		default:
			dmdataLog.WithField("type", envelope.Type).Debug("ignored unknown DMData websocket message")
		}
	}
}

func (c *Client) closeSocket(token, id string) {
	dmdataLog.WithField("socket_id", id).Debug("closing DMData socket allocation")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, socketEndpoint+"/"+url.PathEscape(id), nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		dmdataLog.WithField("socket_id", id).WithError(err).Warn("failed to close DMData socket allocation")
		return
	}
	_ = resp.Body.Close()
	dmdataLog.WithFields(logrus.Fields{"socket_id": id, "status": resp.StatusCode}).Debug("closed DMData socket allocation")
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	started := time.Now()
	input := TokenRequest{ClientID: c.cfg.DMDataClientID, ClientSecret: c.cfg.DMDataClientSecret, GrantType: "refresh_token", RefreshToken: c.cfg.DMDataRefreshToken}
	form := url.Values{"grant_type": {input.GrantType}, "client_id": {input.ClientID}, "client_secret": {input.ClientSecret}, "refresh_token": {input.RefreshToken}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if resp.StatusCode/100 != 2 || out.AccessToken == "" {
		return "", fmt.Errorf("DMData OAuth: %s: %s", out.Error, out.ErrorDescription)
	}
	dmdataLog.WithFields(logrus.Fields{
		"elapsed":    time.Since(started),
		"expires_in": out.ExpiresIn,
		"scope":      out.Scope,
		"status":     resp.StatusCode,
	}).Debug("refreshed DMData access token")
	return out.AccessToken, nil
}

func (c *Client) createSocket(ctx context.Context, token string) (SocketResponse, error) {
	body := SocketRequest{Classifications: []string{"application.jquake", "telegram.earthquake", "eew.forecast"}, Types: SupportedTypes, AppName: c.cfg.DMDataAppName}
	dmdataLog.WithFields(logrus.Fields{"classifications": body.Classifications, "types": body.Types}).Debug("requesting DMData socket allocation")
	started := time.Now()
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, socketEndpoint, bytes.NewReader(raw))
	if err != nil {
		return SocketResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return SocketResponse{}, err
	}
	defer resp.Body.Close()
	var out SocketResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	if resp.StatusCode/100 != 2 || out.WebSocket.URL == "" {
		if out.Error != nil {
			return out, fmt.Errorf("DMData socket %d: %s", out.Error.Code, out.Error.Message)
		}
		return out, fmt.Errorf("DMData socket: HTTP %s", resp.Status)
	}
	dmdataLog.WithFields(logrus.Fields{
		"elapsed":   time.Since(started),
		"socket_id": out.WebSocket.ID,
		"status":    resp.StatusCode,
	}).Debug("allocated DMData socket")
	return out, nil
}
