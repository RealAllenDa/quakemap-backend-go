package model

import "encoding/json"

type TimeSync struct {
	ServerTimestamp int64 `json:"server_timestamp"`
	Difference      int64 `json:"difference"`
}

type GenericResponse struct {
	Status int    `json:"status"`
	Data   string `json:"data"`
}

type DMDataStatus struct {
	Status           string  `json:"status"`
	ActiveSocketID   *string `json:"active_socket_id"`
	WebsocketErrored bool    `json:"websocket_errored"`
	LastPongTime     int64   `json:"last_pong_time"`
	PongTimeDelta    int64   `json:"pong_time_delta"`
}

type EarthquakeInfo struct {
	Info []Earthquake `json:"info"`
	EEW  any          `json:"eew"`
}

func (e EarthquakeInfo) MarshalJSON() ([]byte, error) {
	type wire EarthquakeInfo
	if e.Info == nil {
		e.Info = []Earthquake{}
	}
	return json.Marshal(wire(e))
}

type Earthquake struct {
	ID              *string             `json:"id"`
	Type            string              `json:"type"`
	OccurTime       string              `json:"occur_time"`
	ReceiveTime     string              `json:"receive_time"`
	Magnitude       string              `json:"magnitude"`
	MaxIntensity    string              `json:"max_intensity"`
	TsunamiComments TsunamiComments     `json:"tsunami_comments"`
	Hypocenter      any                 `json:"hypocenter"`
	AreaIntensity   EarthquakeIntensity `json:"area_intensity"`
}

type TsunamiComments struct {
	Domestic string `json:"domestic"`
	Foreign  string `json:"foreign"`
}

type Hypocenter struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Depth     string  `json:"depth"`
}

type EarthquakeIntensity struct {
	Areas   map[string]IntensityPoint `json:"areas"`
	Station map[string]IntensityPoint `json:"station"`
}

type IntensityPoint struct {
	Name          string  `json:"name"`
	Intensity     string  `json:"intensity"`
	LGIntensity   *string `json:"lg_intensity,omitempty"`
	Latitude      string  `json:"latitude"`
	Longitude     string  `json:"longitude"`
	IsArea        bool    `json:"is_area"`
	IntensityCode int     `json:"intensity_code"`
	RegionCode    string  `json:"region_code,omitempty"`
	RegionName    string  `json:"region_name,omitempty"`
}

type EEW struct {
	Status          int                       `json:"status"`
	Type            string                    `json:"type"`
	IsPLUM          bool                      `json:"is_plum"`
	IsCancel        bool                      `json:"is_cancel"`
	IsTest          bool                      `json:"is_test"`
	MaxIntensity    string                    `json:"max_intensity"`
	MaxLGIntensity  string                    `json:"max_lg_intensity"`
	ReportTime      string                    `json:"report_time"`
	ReportTimestamp int64                     `json:"report_timestamp"`
	ReportNum       int                       `json:"report_num"`
	ReportFlag      string                    `json:"report_flag"`
	ReportID        string                    `json:"report_id"`
	OccurTimestamp  int64                     `json:"occur_timestamp"`
	IsFinal         bool                      `json:"is_final"`
	Magnitude       string                    `json:"magnitude"`
	Hypocenter      Hypocenter                `json:"hypocenter"`
	AreaIntensity   map[string]IntensityPoint `json:"area_intensity,omitempty"`
	AreaColoring    AreaColoring              `json:"area_coloring"`
	SWave           *float64                  `json:"s_wave"`
	PWave           *float64                  `json:"p_wave"`
}

type EEWCancelled struct {
	Status   int  `json:"status"`
	IsCancel bool `json:"is_cancel"`
}

type AreaColoring struct {
	Areas            map[string]IntensityPoint `json:"areas"`
	RecommendedAreas bool                      `json:"recommended_areas"`
}

type ShakeLevel struct {
	ShakeLevel int `json:"shake_level"`
	Green      int `json:"green"`
	Yellow     int `json:"yellow"`
	Red        int `json:"red"`
	Status     int `json:"status"`
}

type TsunamiTotal struct {
	Status         string             `json:"status"`
	StatusForecast string             `json:"status_forecast"`
	Map            any                `json:"map"`
	Info           TsunamiExpectation `json:"info"`
	Watch          TsunamiObservation `json:"watch"`
}

type TsunamiExpectation struct {
	ReceiveTime   *string       `json:"receive_time"`
	Areas         []TsunamiArea `json:"areas"`
	ForecastAreas []TsunamiArea `json:"forecast_areas"`
	Origin        *string       `json:"origin"`
}

type TsunamiArea struct {
	Name   string      `json:"name"`
	Grade  string      `json:"grade"`
	Height string      `json:"height"`
	Time   TsunamiTime `json:"time"`
}

type TsunamiTime struct {
	Type      string `json:"type"`
	Time      string `json:"time"`
	Status    *int   `json:"status,omitempty"`
	Timestamp *int64 `json:"timestamp,omitempty"`
}

type TsunamiObservation struct {
	Areas       []TsunamiObservationArea `json:"areas"`
	ReceiveTime *string                  `json:"receive_time"`
}

type TsunamiObservationArea struct {
	Name            string `json:"name"`
	Height          string `json:"height"`
	Time            string `json:"time"`
	Condition       string `json:"condition"`
	HeightCondition string `json:"height_condition"`
	HeightIsMax     bool   `json:"height_is_max"`
}
