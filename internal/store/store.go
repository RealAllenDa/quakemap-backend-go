package store

import (
	"sync"
	"time"

	"quakemap-backend-go/internal/model"
)

type Store struct {
	mu sync.RWMutex

	earthquakes         []model.Earthquake
	previousEarthquakes []model.Earthquake
	previousScalePrompt *model.Earthquake
	eew                 any
	rawData             string
	shake               *model.ShakeLevel
	tsunami             model.TsunamiExpectation
	observation         model.TsunamiObservation
	tsunamiWarning      bool
	tsunamiForecast     bool

	activeSocketID   *string
	websocketErrored bool
	lastPong         time.Time
}

func New() *Store {
	return &Store{
		earthquakes:      []model.Earthquake{},
		eew:              map[string]any{},
		tsunami:          model.TsunamiExpectation{},
		observation:      model.TsunamiObservation{},
		websocketErrored: true,
	}
}

func (s *Store) EarthquakeInfo() model.EarthquakeInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := append([]model.Earthquake{}, s.earthquakes...)
	return model.EarthquakeInfo{Info: items, EEW: s.eew}
}

func (s *Store) SetEEW(v any)        { s.mu.Lock(); s.eew = v; s.mu.Unlock() }
func (s *Store) ClearEEW()           { s.SetEEW(map[string]any{}) }
func (s *Store) SetRawData(v string) { s.mu.Lock(); s.rawData = v; s.mu.Unlock() }
func (s *Store) RawData() string     { s.mu.RLock(); defer s.mu.RUnlock(); return s.rawData }

func (s *Store) SetEarthquake(eq model.Earthquake) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.previousEarthquakes = append([]model.Earthquake(nil), s.earthquakes...)
	if eq.Type == "ScalePrompt" {
		s.previousScalePrompt = new(eq)
		s.earthquakes = []model.Earthquake{eq}
		return
	}
	if eq.Type == "Destination" && s.previousScalePrompt != nil && sameID(s.previousScalePrompt.ID, eq.ID) {
		s.earthquakes = []model.Earthquake{*s.previousScalePrompt, eq}
		return
	}
	s.previousScalePrompt = nil
	s.earthquakes = []model.Earthquake{eq}
}

func (s *Store) CancelEarthquake() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.previousEarthquakes) != 0 {
		s.earthquakes = append([]model.Earthquake(nil), s.previousEarthquakes...)
		s.previousEarthquakes = nil
	}
}

func sameID(a, b *string) bool {
	return a != nil && b != nil && *a == *b
}

func (s *Store) SetShake(v model.ShakeLevel) { s.mu.Lock(); s.shake = &v; s.mu.Unlock() }
func (s *Store) Shake() (model.ShakeLevel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.shake == nil {
		return model.ShakeLevel{}, false
	}
	return *s.shake, true
}

func (s *Store) SetTsunami(v model.TsunamiExpectation, warning, forecast bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tsunami, s.tsunamiWarning, s.tsunamiForecast = v, warning, forecast
}

func (s *Store) SetTsunamiObservation(v model.TsunamiObservation) {
	s.mu.Lock()
	s.observation = v
	s.mu.Unlock()
}

func (s *Store) Tsunami() model.TsunamiTotal {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := "0"
	if s.tsunamiWarning {
		status = "1"
	}
	forecast := "0"
	if s.tsunamiForecast {
		forecast = "1"
	}
	return model.TsunamiTotal{Status: status, StatusForecast: forecast, Map: nil, Info: s.tsunami, Watch: s.observation}
}

func (s *Store) RestoreAPIState(earthquake model.EarthquakeInfo, tsunami model.TsunamiTotal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.earthquakes = append([]model.Earthquake{}, earthquake.Info...)
	s.eew = earthquake.EEW
	if s.eew == nil {
		s.eew = map[string]any{}
	}
	s.previousEarthquakes = nil
	s.previousScalePrompt = nil
	for i := range s.earthquakes {
		if s.earthquakes[i].Type == "ScalePrompt" {
			s.previousScalePrompt = new(s.earthquakes[i])
			break
		}
	}
	s.tsunami = tsunami.Info
	s.observation = tsunami.Watch
	s.tsunamiWarning = tsunami.Status == "1"
	s.tsunamiForecast = tsunami.StatusForecast == "1"
}

func (s *Store) IsTsunami() bool { s.mu.RLock(); defer s.mu.RUnlock(); return s.tsunamiWarning }

func (s *Store) SocketConnected(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeSocketID = &id
	s.websocketErrored = false
	s.lastPong = time.Now()
}
func (s *Store) SocketError() {
	s.mu.Lock()
	s.websocketErrored = true
	s.activeSocketID = nil
	s.mu.Unlock()
}
func (s *Store) Pong() { s.mu.Lock(); s.lastPong = time.Now(); s.mu.Unlock() }

func (s *Store) DMDataStatus(now time.Time) model.DMDataStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	last := int64(0)
	delta := now.Unix()
	if !s.lastPong.IsZero() {
		last = s.lastPong.Unix()
		delta = int64(now.Sub(s.lastPong).Seconds())
	}
	status := "FAIL"
	if s.activeSocketID != nil && !s.websocketErrored && delta < 1800 {
		status = "OK"
	}
	var id *string
	if s.activeSocketID != nil {
		id = new(*s.activeSocketID)
	}
	return model.DMDataStatus{Status: status, ActiveSocketID: id, WebsocketErrored: s.websocketErrored, LastPongTime: last, PongTimeDelta: delta}
}
