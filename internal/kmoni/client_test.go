package kmoni

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"quakemap-backend-go/internal/config"
	"quakemap-backend-go/internal/store"
)

func TestRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"e":0,"l":123,"g":4,"y":5,"r":6,"t":"11:45"}`))
	}))
	defer server.Close()
	s := store.New()
	c := NewClient(config.Config{KMoniURL: server.URL, KMoniInterval: time.Second, HTTPTimeout: time.Second}, s)
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, ok := s.Shake()
	if !ok || got.ShakeLevel != 123 || got.Green != 4 || got.Yellow != 5 || got.Red != 6 || got.Status != 0 {
		t.Fatalf("unexpected shake level: %+v ok=%v", got, ok)
	}
}
