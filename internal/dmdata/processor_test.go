package dmdata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"quakemap-backend-go/internal/model"
	"quakemap-backend-go/internal/store"
)

func testProcessor() (*Processor, *store.Store) {
	s := store.New()
	return NewProcessor(s, filepath.Join("..", "..", "assets", "centroid")), s
}

func TestSocketMessageCorpus(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "test", "assets", "dmdata", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var message SocketData
			if err := json.Unmarshal(raw, &message); err != nil {
				t.Fatal(err)
			}
			p, _ := testProcessor()
			if err := p.ParseSocketData(message); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestJMAXMLFixtureCorpus(t *testing.T) {
	cases := []struct {
		name, dir, typ string
	}{
		{"forecast", "eew_forecast", TypeEEWForecast},
		{"warning", "eew_warning", TypeEEWWarning},
		{"tsunami_expectation", "tsunami_expectation", TypeTsunami},
		{"tsunami_watch", "tsunami_watch", TypeTsunamiInfo},
		{"earthquake_intensity", "earthquake_intensity", TypeIntensity},
		{"earthquake_destination", "earthquake_destination", TypeDestination},
		{"earthquake_detail", "earthquake_detail", TypeDetail},
		{"earthquake_update", "earthquake_update", TypeUpdate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files, err := filepath.Glob(filepath.Join("..", "..", "test", "assets", tc.dir, "*.xml"))
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range files {
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				p, _ := testProcessor()
				if err := p.ParseXML(tc.typ, raw, true); err != nil {
					t.Errorf("%s: %v", filepath.Base(path), err)
				}
			}
		})
	}
}

func TestRawJMAXMLJSONCorpus(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "test", "assets", "raw_messages", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, _ := testProcessor()
	for _, path := range files {
		name := filepath.Base(path)
		typ := ""
		for _, candidate := range SupportedTypes {
			if strings.Contains(name, candidate) {
				typ = candidate
				break
			}
		}
		if typ == "" {
			t.Errorf("%s: no telegram type", name)
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := p.ParseJSONReport(typ, raw); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestEarthquakeOutputContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "test", "assets", "raw_messages", "2024-01-01T07_10_04Z_VXSE53.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, s := testProcessor()
	if err := p.ParseJSONReport(TypeDetail, raw); err != nil {
		t.Fatal(err)
	}
	got := s.EarthquakeInfo()
	if len(got.Info) != 1 {
		t.Fatalf("expected one earthquake, got %d", len(got.Info))
	}
	eq := got.Info[0]
	if eq.Type != "DetailScale" || eq.Magnitude != "5.7" || eq.MaxIntensity != "5+" {
		t.Fatalf("unexpected earthquake: %+v", eq)
	}
	hypo, ok := eq.Hypocenter.(model.Hypocenter)
	if !ok {
		t.Fatalf("unexpected hypocenter type %T", eq.Hypocenter)
	}
	if hypo.Name != "石川県能登地方" || hypo.Depth != "10km" || hypo.Latitude != 37.5 || hypo.Longitude != 137.3 {
		t.Fatalf("unexpected hypocenter: %+v", hypo)
	}
	if len(eq.AreaIntensity.Station) == 0 {
		t.Fatal("expected station intensities")
	}
	for name, point := range eq.AreaIntensity.Areas {
		if point.LGIntensity != nil {
			t.Fatalf("earthquake area %q unexpectedly has long-period intensity %q", name, *point.LGIntensity)
		}
	}
	for name, point := range eq.AreaIntensity.Station {
		if point.LGIntensity != nil {
			t.Fatalf("earthquake station %q unexpectedly has long-period intensity %q", name, *point.LGIntensity)
		}
	}
}

func TestEEWAndTsunamiOutputContracts(t *testing.T) {
	t.Run("eew", func(t *testing.T) {
		raw, err := os.ReadFile(filepath.Join("..", "..", "test", "assets", "eew_forecast", "77_01_01_240613_VXSE45.xml"))
		if err != nil {
			t.Fatal(err)
		}
		p, s := testProcessor()
		if err := p.ParseXML(TypeEEWForecast, raw, true); err != nil {
			t.Fatal(err)
		}
		got, ok := s.EarthquakeInfo().EEW.(model.EEW)
		if !ok {
			t.Fatalf("unexpected EEW type %T", s.EarthquakeInfo().EEW)
		}
		if got.ReportID != "20240417231454" || got.MaxIntensity != "3" || got.Hypocenter.Name != "豊後水道" || len(got.AreaColoring.Areas) != 0 ||
			got.MaxLGIntensity != "" {
			t.Fatalf("unexpected EEW: %+v", got)
		}
	})
	t.Run("eew_long_period_intensity", func(t *testing.T) {
		raw, err := os.ReadFile(filepath.Join("..", "..", "test", "assets", "eew_warning", "37_01_02_240613_VXSE43.xml"))
		if err != nil {
			t.Fatal(err)
		}
		p, s := testProcessor()
		if err := p.ParseXML(TypeEEWWarning, raw, true); err != nil {
			t.Fatal(err)
		}
		got, ok := s.EarthquakeInfo().EEW.(model.EEW)
		if !ok {
			t.Fatalf("unexpected EEW type %T", s.EarthquakeInfo().EEW)
		}
		if got.MaxLGIntensity != "2" {
			t.Fatalf("unexpected maximum long-period intensity: %q", got.MaxLGIntensity)
		}
		point, ok := got.AreaColoring.Areas["大分県中部"]
		if !ok {
			t.Fatal("expected 大分県中部 forecast area")
		}
		if point.LGIntensity == nil || *point.LGIntensity != "2" {
			t.Fatalf("unexpected area long-period intensity: %#v", point.LGIntensity)
		}
		for name, point := range got.AreaColoring.Areas {
			if point.LGIntensity == nil {
				t.Fatalf("EEW area %q is missing lg_intensity", name)
			}
		}
	})
	t.Run("tsunami_expectation", func(t *testing.T) {
		p, s := testProcessor()
		raw, err := os.ReadFile(filepath.Join("..", "..", "test", "assets", "tsunami_expectation", "32-39_11_02_250206_VTSE41.xml"))
		if err != nil {
			t.Fatal(err)
		}
		if err := p.ParseXML(TypeTsunami, raw, true); err != nil {
			t.Fatal(err)
		}
		got := s.Tsunami()
		if got.Status != "1" || got.Info.ReceiveTime == nil || len(got.Info.Areas) == 0 {
			t.Fatalf("unexpected tsunami expectation: %+v", got)
		}
	})
	t.Run("tsunami_watch_not_parsed", func(t *testing.T) {
		p, s := testProcessor()
		raw, err := os.ReadFile(filepath.Join("..", "..", "test", "assets", "tsunami_watch", "32-39_11_03_250206_VTSE51.xml"))
		if err != nil {
			t.Fatal(err)
		}
		if err := p.ParseXML(TypeTsunamiInfo, raw, true); err != nil {
			t.Fatal(err)
		}
		got := s.Tsunami()
		if got.Watch.ReceiveTime != nil || len(got.Watch.Areas) != 0 {
			t.Fatalf("unexpected unparsed tsunami observation: %+v", got.Watch)
		}
	})
	t.Run("tsunami_watch_parsed", func(t *testing.T) {
		p, s := testProcessor()
		raw, err := os.ReadFile(filepath.Join("..", "..", "test", "assets", "tsunami_watch", "32-39_11_08_250206_VTSE51.xml"))
		if err != nil {
			t.Fatal(err)
		}
		if err := p.ParseXML(TypeTsunamiInfo, raw, true); err != nil {
			t.Fatal(err)
		}
		got := s.Tsunami()
		if got.Watch.ReceiveTime == nil || len(got.Watch.Areas) == 0 {
			t.Fatalf("unexpected tsunami observation: %+v", got.Watch)
		}
	})
}

func TestDecodeValidation(t *testing.T) {
	p, _ := testProcessor()
	if err := p.ParseSocketData(SocketData{Format: "json"}); err == nil {
		t.Fatal("expected format error")
	}
	if err := p.ParseSocketData(SocketData{Format: "xml", Compression: "zip", Encoding: "base64"}); err == nil {
		t.Fatal("expected codec error")
	}
	if _, err := DecodeBody("not-base64"); err == nil {
		t.Fatal("expected decode error")
	}
}
