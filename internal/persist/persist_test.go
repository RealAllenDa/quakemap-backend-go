package persist_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"quakemap-backend-go/internal/model"
	"quakemap-backend-go/internal/persist"
	"quakemap-backend-go/internal/store"
)

func TestRepositoryLifecycle(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "persist")
	repository, err := persist.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(directory); err != nil {
		t.Fatalf("persistence directory was not created: %v", err)
	}
	if _, exists, err := repository.Load(); err != nil || exists {
		t.Fatalf("first load: exists=%v err=%v", exists, err)
	}

	earthquake := model.EarthquakeInfo{Info: []model.Earthquake{}, EEW: map[string]any{"status": float64(0)}}
	tsunami := model.TsunamiTotal{Status: "1", StatusForecast: "0", Map: nil, Info: model.TsunamiExpectation{}, Watch: model.TsunamiObservation{}}
	if err := repository.Save(earthquake, tsunami); err != nil {
		t.Fatal(err)
	}
	state, exists, err := repository.Load()
	if err != nil || !exists {
		t.Fatalf("load: exists=%v err=%v", exists, err)
	}
	if state.EarthquakeInfo.Info == nil || state.TsunamiInfo.Status != "1" || state.SavedAt.IsZero() {
		t.Fatalf("unexpected restored state: %+v", state)
	}

	// Saving twice exercises replacement of an existing state file on Windows.
	tsunami.Status = "0"
	if err := repository.Save(earthquake, tsunami); err != nil {
		t.Fatal(err)
	}
	state, _, err = repository.Load()
	if err != nil || state.TsunamiInfo.Status != "0" {
		t.Fatalf("replacement load: state=%+v err=%v", state, err)
	}
}

func TestSavedAPIModelsRestoreSemantically(t *testing.T) {
	repository, err := persist.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := "event-id"
	earthquake := model.EarthquakeInfo{
		Info: []model.Earthquake{{
			ID:            &id,
			Type:          "DetailScale",
			Magnitude:     "5.7",
			MaxIntensity:  "5+",
			Hypocenter:    model.Hypocenter{Name: "test", Latitude: 35, Longitude: 139, Depth: "10km"},
			AreaIntensity: model.EarthquakeIntensity{Areas: map[string]model.IntensityPoint{}, Station: map[string]model.IntensityPoint{}},
		}},
		EEW: model.EEW{Status: 0, Type: "svir", ReportID: id, AreaColoring: model.AreaColoring{Areas: map[string]model.IntensityPoint{}}},
	}
	tsunami := model.TsunamiTotal{
		Status:         "1",
		StatusForecast: "0",
		Info:           model.TsunamiExpectation{Areas: []model.TsunamiArea{}, ForecastAreas: []model.TsunamiArea{}},
		Watch:          model.TsunamiObservation{Areas: []model.TsunamiObservationArea{}},
	}
	if err := repository.Save(earthquake, tsunami); err != nil {
		t.Fatal(err)
	}
	saved, exists, err := repository.Load()
	if err != nil || !exists {
		t.Fatalf("load: exists=%v err=%v", exists, err)
	}
	restored := store.New()
	restored.RestoreAPIState(saved.EarthquakeInfo, saved.TsunamiInfo)
	if !reflect.DeepEqual(asJSONValue(t, earthquake), asJSONValue(t, restored.EarthquakeInfo())) {
		t.Fatalf("earthquake API state changed during round trip")
	}
	if !reflect.DeepEqual(asJSONValue(t, tsunami), asJSONValue(t, restored.Tsunami())) {
		t.Fatalf("tsunami API state changed during round trip")
	}
}

func TestRepositoryRejectsCorruptAndUnknownState(t *testing.T) {
	repository, err := persist.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repository.Path(), []byte(`{"version":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := repository.Load(); err == nil || !exists {
		t.Fatalf("corrupt state: exists=%v err=%v", exists, err)
	}
	unknown, _ := json.Marshal(map[string]any{"version": 99})
	if err := os.WriteFile(repository.Path(), unknown, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := repository.Load(); err == nil || !exists {
		t.Fatalf("unknown state: exists=%v err=%v", exists, err)
	}
}

func asJSONValue(t *testing.T, value any) any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var normalized any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		t.Fatal(err)
	}
	return normalized
}
