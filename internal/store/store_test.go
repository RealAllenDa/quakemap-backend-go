package store

import (
	"testing"

	"quakemap-backend-go/internal/model"
)

func TestRestoreAPIState(t *testing.T) {
	id := "event"
	earthquake := model.EarthquakeInfo{
		Info: []model.Earthquake{{ID: &id, Type: "ScalePrompt"}},
		EEW:  map[string]any{"status": float64(0)},
	}
	received := "2026/06/29 12:00:00"
	tsunami := model.TsunamiTotal{
		Status:         "1",
		StatusForecast: "1",
		Info:           model.TsunamiExpectation{ReceiveTime: &received, Areas: []model.TsunamiArea{}},
		Watch:          model.TsunamiObservation{Areas: []model.TsunamiObservationArea{}},
	}
	s := New()
	s.RestoreAPIState(earthquake, tsunami)
	gotEarthquake := s.EarthquakeInfo()
	if len(gotEarthquake.Info) != 1 || gotEarthquake.Info[0].Type != "ScalePrompt" {
		t.Fatalf("unexpected earthquake state: %+v", gotEarthquake)
	}
	gotTsunami := s.Tsunami()
	if gotTsunami.Status != "1" || gotTsunami.StatusForecast != "1" || gotTsunami.Info.ReceiveTime == nil {
		t.Fatalf("unexpected tsunami state: %+v", gotTsunami)
	}
	if !s.IsTsunami() {
		t.Fatal("expected restored tsunami warning")
	}

	// The restored ScalePrompt remains available for the next Destination.
	s.SetEarthquake(model.Earthquake{ID: &id, Type: "Destination"})
	if got := s.EarthquakeInfo(); len(got.Info) != 2 {
		t.Fatalf("expected ScalePrompt/Destination pair, got %+v", got.Info)
	}
}

func TestEarthquakeInfoIsNonNilWhenEmpty(t *testing.T) {
	s := New()
	if got := s.EarthquakeInfo(); got.Info == nil || len(got.Info) != 0 {
		t.Fatalf("expected non-nil empty earthquake list, got %#v", got.Info)
	}
	s.RestoreAPIState(model.EarthquakeInfo{}, model.TsunamiTotal{})
	if got := s.EarthquakeInfo(); got.Info == nil || len(got.Info) != 0 {
		t.Fatalf("expected restored non-nil empty earthquake list, got %#v", got.Info)
	}
}
