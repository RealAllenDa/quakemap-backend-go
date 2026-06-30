package dmdata

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"quakemap-backend-go/internal/jmaxml"
	"quakemap-backend-go/internal/model"
	"quakemap-backend-go/internal/store"
)

const (
	TypeEEWForecast = "VXSE45"
	TypeEEWWarning  = "VXSE43"
	TypeIntensity   = "VXSE51"
	TypeDestination = "VXSE52"
	TypeDetail      = "VXSE53"
	TypeUpdate      = "VXSE61"
	TypeTsunami     = "VTSE41"
	TypeTsunamiInfo = "VTSE51"
)

var SupportedTypes = []string{
	TypeIntensity, TypeDestination, TypeDetail, TypeUpdate,
	TypeTsunami, TypeTsunamiInfo, TypeEEWWarning, TypeEEWForecast,
}

type SocketData struct {
	Type           string    `json:"type"`
	Version        string    `json:"version"`
	ID             string    `json:"id"`
	Classification string    `json:"classification"`
	Passing        []Passing `json:"passing"`
	Head           DataHead  `json:"head"`
	Format         string    `json:"format"`
	Compression    string    `json:"compression"`
	Encoding       string    `json:"encoding"`
	Body           string    `json:"body"`
}

type Passing struct {
	Name string    `json:"name"`
	Time time.Time `json:"time"`
}
type DataHead struct {
	Type        string    `json:"type"`
	Author      string    `json:"author"`
	Time        time.Time `json:"time"`
	Designation *string   `json:"designation"`
	Test        bool      `json:"test"`
	XML         bool      `json:"xml"`
}

type point struct{ lat, lon, regionCode, regionName string }

type Processor struct {
	store       *store.Store
	areasByName map[string]point
	areasByCode map[string]point
	stations    map[string]point
	now         func() time.Time
}

func NewProcessor(s *store.Store, centroidDir string) *Processor {
	p := &Processor{store: s, areasByName: map[string]point{}, areasByCode: map[string]point{}, stations: map[string]point{}, now: time.Now}
	p.loadCentroids(centroidDir)
	dmdataLog.WithFields(logrus.Fields{
		"area_codes": len(p.areasByCode),
		"area_names": len(p.areasByName),
		"stations":   len(p.stations),
	}).Debug("loaded JMA centroid data")
	return p
}

func (p *Processor) ParseSocketData(message SocketData) error {
	dmdataLog.WithFields(logrus.Fields{
		"id":             message.ID,
		"telegram":       message.Head.Type,
		"classification": message.Classification,
		"format":         message.Format,
		"compression":    message.Compression,
		"encoding":       message.Encoding,
	}).Debug("validating DMData telegram envelope")
	if message.Format != "xml" {
		return errors.New("dmdata: format is not xml")
	}
	if message.Compression != "gzip" || message.Encoding != "base64" {
		return errors.New("dmdata: expected base64 encoded gzip body")
	}
	raw, err := DecodeBody(message.Body)
	if err != nil {
		return err
	}
	dmdataLog.WithFields(logrus.Fields{
		"id":            message.ID,
		"telegram":      message.Head.Type,
		"encoded_bytes": len(message.Body),
		"xml_bytes":     len(raw),
	}).Debug("decoded DMData telegram body")
	p.store.SetRawData(message.Body)
	return p.ParseXML(message.Head.Type, raw, message.Head.Test)
}

func DecodeBody(body string) ([]byte, error) {
	compressed, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return nil, fmt.Errorf("dmdata: decode base64: %w", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("dmdata: open gzip: %w", err)
	}
	defer zr.Close()
	raw, err := io.ReadAll(io.LimitReader(zr, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("dmdata: decompress: %w", err)
	}
	return raw, nil
}

func (p *Processor) ParseXML(messageType string, raw []byte, socketTest bool) error {
	if !slices.Contains(SupportedTypes, messageType) {
		return fmt.Errorf("dmdata: unsupported telegram %q", messageType)
	}
	var report jmaxml.Report
	if err := xml.Unmarshal(raw, &report); err != nil {
		return fmt.Errorf("dmdata: unmarshal JMAXML: %w", err)
	}
	dmdataLog.WithFields(logrus.Fields{
		"telegram":    messageType,
		"event_id":    report.Head.EventID,
		"info_type":   report.Head.InfoType,
		"title":       report.Head.Title,
		"socket_test": socketTest,
		"xml_bytes":   len(raw),
	}).Debug("unmarshaled JMAXML report")
	return p.ParseReport(messageType, report, socketTest)
}

func (p *Processor) ParseJSONReport(messageType string, raw []byte) error {
	dmdataLog.WithFields(logrus.Fields{"telegram": messageType, "json_bytes": len(raw)}).Debug("converting debug JSON report to JMAXML")
	xmlData, err := JSONReportToXML(raw)
	if err != nil {
		return err
	}
	return p.ParseXML(messageType, xmlData, false)
}

func (p *Processor) ParseReport(messageType string, report jmaxml.Report, socketTest bool) error {
	dmdataLog.WithFields(logrus.Fields{"telegram": messageType, "event_id": report.Head.EventID}).Debug("routing JMAXML report")
	switch messageType {
	case TypeEEWForecast, TypeEEWWarning:
		return p.parseEEW(report, messageType, socketTest)
	case TypeIntensity, TypeDestination, TypeDetail:
		return p.parseEarthquake(report, messageType)
	case TypeUpdate:
		// The retained API has no representation for hypocenter-only updates.
		dmdataLog.WithField("event_id", report.Head.EventID).Debug("ignored unsupported hypocenter-only update")
		return nil
	case TypeTsunami, TypeTsunamiInfo:
		return p.parseTsunami(report, messageType)
	default:
		return fmt.Errorf("dmdata: unsupported telegram %q", messageType)
	}
}

func (p *Processor) parseEEW(r jmaxml.Report, messageType string, socketTest bool) error {
	if r.Head.InfoType != "発表" {
		p.store.SetEEW(model.EEWCancelled{Status: 0, IsCancel: true})
		dmdataLog.WithFields(logrus.Fields{"event_id": r.Head.EventID, "info_type": r.Head.InfoType}).Debug("stored EEW cancellation")
		return nil
	}
	if len(r.Body.Earthquakes) == 0 {
		return errors.New("dmdata: EEW has no earthquake element")
	}
	eq := r.Body.Earthquakes[0]
	lat, lon, depth := parseCoordinate(firstCoordinate(eq.Hypocenter.Area.Coordinates))
	origin := eq.OriginTime.Time
	if origin.IsZero() {
		origin = eq.ArrivalTime.Time
	}
	serial, _ := strconv.Atoi(strings.TrimSpace(r.Head.Serial))
	maxInt, maxLG := "0", ""
	areas := map[string]model.IntensityPoint{}
	if r.Body.Intensity != nil && r.Body.Intensity.Forecast != nil {
		forecast := r.Body.Intensity.Forecast
		maxInt = normalizeIntensity(firstNonempty(forecast.ForecastInt.From, forecast.MaxInt))
		maxLG = normalizeLGIntensity(firstNonempty(forecast.ForecastLGInt.From, forecast.MaxLGInt))
		for _, pref := range forecast.Prefs {
			for _, area := range pref.Areas {
				var intensity, lgIntensity string
				if area.ForecastInt.To == "over" {
					intensity = normalizeIntensity(firstNonempty(area.ForecastInt.From, area.MaxInt))
				} else {
					intensity = normalizeIntensity(firstNonempty(area.ForecastInt.To, area.ForecastInt.From, area.MaxInt))
				}
				if area.ForecastLGInt.To == "over" {
					lgIntensity = normalizeLGIntensity(firstNonempty(area.ForecastLGInt.From, area.MaxLGInt))
				} else {
					lgIntensity = normalizeLGIntensity(firstNonempty(area.ForecastLGInt.To, area.ForecastLGInt.From, area.MaxLGInt))
				}
				pt := p.areaPointWithLGIntensity(area.Name, area.Code, intensity, lgIntensity, true)
				areas[area.Name] = pt
			}
		}
	}
	if maxInt == "" {
		maxInt = "0"
	}
	magnitude := magnitudeValue(eq.Magnitudes)
	reportTime := r.Head.ReportDateTime.Time
	if reportTime.IsZero() {
		reportTime = r.Control.DateTime.Time
	}
	eew := model.EEW{
		Status: 0, Type: "svir", IsPLUM: strings.TrimSpace(eq.Condition) != "", IsCancel: false,
		IsTest: socketTest || r.Control.Status != "通常", MaxIntensity: maxInt, MaxLGIntensity: maxLG,
		ReportTime: formatDateTime(reportTime), ReportTimestamp: reportTime.Unix(), ReportNum: serial,
		ReportFlag: map[bool]string{true: "1", false: "0"}[messageType == TypeEEWWarning || warningComment(r.Body.Comments)],
		ReportID:   r.Head.EventID, OccurTimestamp: origin.Unix(), IsFinal: strings.TrimSpace(r.Body.NextAdvisory) != "",
		Magnitude: magnitude, Hypocenter: model.Hypocenter{Name: eq.Hypocenter.Area.Name, Latitude: lat, Longitude: lon, Depth: formatDepth(depth)},
		AreaColoring: model.AreaColoring{Areas: areas, RecommendedAreas: true}, SWave: nil, PWave: nil,
	}
	p.store.SetEEW(eew)
	dmdataLog.WithFields(logrus.Fields{
		"event_id":       eew.ReportID,
		"report_number":  eew.ReportNum,
		"max_intensity":  eew.MaxIntensity,
		"forecast_areas": len(eew.AreaColoring.Areas),
		"is_final":       eew.IsFinal,
		"is_test":        eew.IsTest,
	}).Debug("updated EEW state")
	return nil
}

func (p *Processor) parseEarthquake(r jmaxml.Report, messageType string) error {
	if r.Head.InfoType == "取消" {
		p.store.CancelEarthquake()
		dmdataLog.WithField("event_id", r.Head.EventID).Debug("reverted cancelled earthquake report")
		return nil
	}
	issue := "ScalePrompt"
	switch messageType {
	case TypeDestination:
		issue = "Destination"
	case TypeDetail:
		issue = "DetailScale"
		if r.Head.Title == "遠地地震に関する情報" || r.Body.Intensity == nil {
			issue = "Foreign"
		}
	}
	id := r.Head.EventID
	eq := model.Earthquake{
		ID: &id, Type: issue, ReceiveTime: p.now().Format("2006/01/02 15:04:05.000"),
		Magnitude: "-1", MaxIntensity: "-1",
		TsunamiComments: model.TsunamiComments{Domestic: domesticTsunami(r.Body.Comments), Foreign: "None"},
		Hypocenter:      map[string]any{},
		AreaIntensity:   model.EarthquakeIntensity{Areas: map[string]model.IntensityPoint{}, Station: map[string]model.IntensityPoint{}},
	}
	if issue == "Foreign" {
		eq.TsunamiComments.Foreign = foreignTsunami(r.Body.Comments)
	}
	if issue == "ScalePrompt" {
		eq.OccurTime = formatDateTime(r.Head.TargetDateTime.Time)
	} else if len(r.Body.Earthquakes) > 0 {
		bodyEQ := r.Body.Earthquakes[0]
		occur := bodyEQ.ArrivalTime.Time
		if occur.IsZero() {
			occur = bodyEQ.OriginTime.Time
		}
		eq.OccurTime = formatDateTime(occur)
		lat, lon, depth := parseCoordinate(firstCoordinate(bodyEQ.Hypocenter.Area.Coordinates))
		eq.Hypocenter = model.Hypocenter{Name: bodyEQ.Hypocenter.Area.Name, Latitude: lat, Longitude: lon, Depth: formatDepth(depth)}
		eq.Magnitude = magnitudeValue(bodyEQ.Magnitudes)
	}
	if r.Body.Intensity != nil && r.Body.Intensity.Observation != nil && issue != "Destination" && issue != "Foreign" {
		obs := r.Body.Intensity.Observation
		eq.MaxIntensity = normalizeIntensity(obs.MaxInt)
		for _, pref := range obs.Prefs {
			for _, area := range pref.Areas {
				if issue == "ScalePrompt" {
					pt := p.areaPoint(area.Name, area.Code, normalizeIntensity(area.MaxInt), true)
					eq.AreaIntensity.Areas[area.Name] = pt
					continue
				}
				stations := append([]jmaxml.IntensityStation{}, area.Stations...)
				for _, city := range area.Cities {
					stations = append(stations, city.Stations...)
				}
				for _, station := range stations {
					name := strings.TrimSuffix(station.Name, "＊")
					pt, err := p.stationPoint(name, normalizeIntensity(station.Int))
					if err != nil {
						logrus.WithFields(logrus.Fields{
							"name": name,
						}).Warn("malformed/nonexistent station point")
					} else {
						eq.AreaIntensity.Station[name] = pt
						if pt.RegionName != "" {
							existing, ok := eq.AreaIntensity.Areas[pt.RegionName]
							if !ok || pt.IntensityCode > existing.IntensityCode {
								a := p.areaPoint(pt.RegionName, pt.RegionCode, pt.Intensity, true)
								eq.AreaIntensity.Areas[pt.RegionName] = a
							}
						}
					}
				}
			}
		}
	}
	p.store.SetEarthquake(eq)
	dmdataLog.WithFields(logrus.Fields{
		"event_id":      id,
		"issue_type":    eq.Type,
		"magnitude":     eq.Magnitude,
		"max_intensity": eq.MaxIntensity,
		"area_count":    len(eq.AreaIntensity.Areas),
		"station_count": len(eq.AreaIntensity.Station),
	}).Debug("updated earthquake state")
	return nil
}

func (p *Processor) parseTsunami(r jmaxml.Report, messageType string) error {
	if r.Head.InfoType == "取消" {
		blank := model.TsunamiExpectation{}
		p.store.SetTsunami(blank, false, false)
		dmdataLog.WithField("event_id", r.Head.EventID).Debug("cleared cancelled tsunami report")
		return nil
	}
	if r.Body.Tsunami == nil {
		return errors.New("dmdata: tsunami telegram has no Tsunami element")
	}
	if r.Body.Tsunami.Forecast != nil {
		origin := "TE"
		if messageType == TypeTsunamiInfo {
			origin = "TW"
		}
		info := model.TsunamiExpectation{ReceiveTime: new(formatDateTime(r.Head.ReportDateTime.Time)), Origin: &origin, Areas: []model.TsunamiArea{}, ForecastAreas: []model.TsunamiArea{}}
		for _, item := range r.Body.Tsunami.Forecast.Items {
			grade := tsunamiGrade(item.Category.Kind)
			if grade == "cancel" {
				continue
			}
			area := model.TsunamiArea{Name: item.Area.Name, Grade: grade, Height: tsunamiHeight(item.MaxHeight.TsunamiHeight), Time: tsunamiTime(item.FirstHeight)}
			if grade == "Forecast" {
				info.ForecastAreas = append(info.ForecastAreas, area)
			} else {
				info.Areas = append(info.Areas, area)
			}
		}
		p.store.SetTsunami(info, len(info.Areas) > 0, len(info.ForecastAreas) > 0)
		dmdataLog.WithFields(logrus.Fields{
			"event_id":       r.Head.EventID,
			"origin":         origin,
			"warning_areas":  len(info.Areas),
			"forecast_areas": len(info.ForecastAreas),
		}).Debug("updated tsunami forecast state")
	}
	if r.Body.Tsunami.Observation != nil {
		watch := model.TsunamiObservation{ReceiveTime: new(formatDateTime(r.Head.ReportDateTime.Time)), Areas: []model.TsunamiObservationArea{}}
		for _, item := range r.Body.Tsunami.Observation.Items {
			for _, station := range item.Stations {
				watch.Areas = append(watch.Areas, observationArea(station))
			}
		}
		p.store.SetTsunamiObservation(watch)
		dmdataLog.WithFields(logrus.Fields{"event_id": r.Head.EventID, "stations": len(watch.Areas)}).Debug("updated tsunami observation state")
	}
	return nil
}

func (p *Processor) loadCentroids(dir string) {
	loadCSV(filepath.Join(dir, "jma_area_centroid.csv"), func(row []string) {
		if len(row) < 4 {
			return
		}
		pt := point{lat: row[2], lon: row[3]}
		p.areasByCode[row[0]], p.areasByName[row[1]] = pt, pt
	})
	loadCSV(filepath.Join(dir, "intensity_stations.csv"), func(row []string) {
		if len(row) < 5 {
			return
		}
		p.stations[row[0]] = point{regionCode: row[1], regionName: row[2], lat: row[3], lon: row[4]}
	})
}

func loadCSV(path string, use func([]string)) {
	f, err := os.Open(path)
	if err != nil {
		dmdataLog.WithField("path", path).WithError(err).Warn("failed to load centroid CSV")
		return
	}
	defer f.Close()
	r := csv.NewReader(bufio.NewReader(f))
	r.FieldsPerRecord = -1
	for {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			dmdataLog.WithField("path", path).WithError(err).Warn("stopped reading centroid CSV")
			break
		}
		use(row)
	}
}

func (p *Processor) areaPoint(name, code, intensity string, isArea bool) model.IntensityPoint {
	pt, ok := p.areasByName[name]
	if !ok {
		pt = p.areasByCode[code]
	}
	return model.IntensityPoint{Name: name, Intensity: intensity, Latitude: pt.lat, Longitude: pt.lon, IsArea: isArea, IntensityCode: intensityCode(intensity)}
}

func (p *Processor) areaPointWithLGIntensity(name, code, intensity, lgIntensity string, isArea bool) model.IntensityPoint {
	point := p.areaPoint(name, code, intensity, isArea)
	point.LGIntensity = new(lgIntensity)
	return point
}

func (p *Processor) stationPoint(name, intensity string) (model.IntensityPoint, error) {
	pt := p.stations[name]
	if pt.lat == "" || pt.lon == "" {
		return model.IntensityPoint{
			Name: name, Intensity: intensity, Latitude: pt.lat, Longitude: pt.lon, IsArea: false, IntensityCode: intensityCode(intensity), RegionCode: pt.regionCode, RegionName: pt.regionName,
		}, errors.New("malformed/nonexistent station point")
	}
	return model.IntensityPoint{
		Name: name, Intensity: intensity, Latitude: pt.lat, Longitude: pt.lon, IsArea: false, IntensityCode: intensityCode(intensity), RegionCode: pt.regionCode, RegionName: pt.regionName,
	}, nil
}

var coordinateRE = regexp.MustCompile(`([+-][0-9.]+)([+-][0-9.]+)([+-][0-9.]+)?`)

func parseCoordinate(value string) (float64, float64, int) {
	m := coordinateRE.FindStringSubmatch(strings.TrimSpace(value))
	if len(m) < 3 {
		return -200, -200, -1
	}
	lat, _ := strconv.ParseFloat(m[1], 64)
	lon, _ := strconv.ParseFloat(m[2], 64)
	depth := -1
	if len(m) > 3 && m[3] != "" {
		metres, _ := strconv.ParseFloat(m[3], 64)
		depth = int(-metres / 1000)
	}
	return lat, lon, depth
}
func firstCoordinate(v []jmaxml.ElementValue) string {
	if len(v) == 0 {
		return ""
	}
	return v[0].Value
}
func firstNonempty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
func formatDateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006/01/02 15:04:05")
}
func formatDepth(d int) string {
	if d == 0 {
		return "Shallow"
	}
	if d == 700 {
		return "Over 700km"
	}
	if d < 0 {
		return "Unknown"
	}
	return strconv.Itoa(d) + "km"
}
func magnitudeValue(v []jmaxml.ElementValue) string {
	if len(v) == 0 {
		return "-1"
	}
	value := strings.TrimSpace(v[0].Value)
	if value == "" || value == "NaN" {
		if strings.Contains(v[0].Description, "Ｍ８を超える") {
			return "Over 8"
		}
		return "Unknown"
	}
	return value
}
func normalizeIntensity(v string) string {
	v = strings.TrimSpace(v)
	replace := map[string]string{
		"震度０":       "0",
		"震度１":       "1",
		"震度２":       "2",
		"震度３":       "3",
		"震度４":       "4",
		"震度５弱":      "5-",
		"震度５弱以上未入電": "5?",
		"震度５強":      "5+",
		"震度６弱":      "6-",
		"震度６強":      "6+",
		"震度７":       "7",
		"5弱":        "5-",
		"5強":        "5+",
		"6弱":        "6-",
		"6強":        "6+",
		"５弱":        "5-",
		"５強":        "5+",
		"６弱":        "6-",
		"６強":        "6+",
	}
	if n, ok := replace[v]; ok {
		return n
	}
	return v
}
func normalizeLGIntensity(v string) string {
	v = strings.TrimSpace(v)
	if v == "0" {
		return ""
	}
	return v
}
func intensityCode(v string) int {
	return map[string]int{"1": 10, "2": 20, "3": 30, "4": 40, "5-": 45, "5?": 46, "5+": 50, "6-": 55, "6+": 60, "7": 70}[normalizeIntensity(v)]
}
func warningComment(c *jmaxml.Comments) bool {
	return c != nil && c.WarningComment != nil && strings.Contains(c.WarningComment.Code, "0201")
}
func domesticTsunami(c *jmaxml.Comments) string {
	if c == nil || c.ForecastComment == nil {
		return "Unknown"
	}
	code := c.ForecastComment.Code
	switch {
	case strings.Contains(code, "0215") || strings.Contains(code, "0230"):
		return "None"
	case strings.Contains(code, "0212") || strings.Contains(code, "0213") || strings.Contains(code, "0214"):
		return "NonEffective"
	case strings.Contains(code, "0211"):
		return "Warning"
	case strings.Contains(code, "0217") || strings.Contains(code, "0229"):
		return "Checking"
	default:
		return "None"
	}
}
func foreignTsunami(c *jmaxml.Comments) string {
	if c == nil || c.ForecastComment == nil {
		return "Unknown"
	}
	code := c.ForecastComment.Code
	for token, status := range map[string]string{"0221": "WarningPacificWide", "0222": "WarningPacific", "0223": "WarningNorthwestPacific", "0224": "WarningIndianWide", "0225": "WarningIndian", "0226": "WarningNearby", "0227": "NonEffectiveNearby", "0228": "Potential"} {
		if strings.Contains(code, token) {
			return status
		}
	}
	return "None"
}
func tsunamiGrade(k jmaxml.Kind) string {
	name, code := k.Name, k.Code
	if strings.Contains(name, "解除") || code == "00" {
		return "cancel"
	}
	if strings.Contains(name, "大津波") || strings.HasPrefix(code, "53") || strings.HasPrefix(code, "52") {
		return "MajorWarning"
	}
	if strings.Contains(name, "津波警報") || strings.HasPrefix(code, "51") {
		return "Warning"
	}
	if strings.Contains(name, "注意報") || strings.HasPrefix(code, "62") {
		return "Watch"
	}
	if strings.Contains(name, "津波予報") || strings.HasPrefix(code, "71") {
		return "Forecast"
	}
	return "Unknown"
}
func tsunamiHeight(v jmaxml.ElementValue) string {
	d := strings.TrimSpace(v.Description)
	value := strings.TrimSpace(v.Value)
	switch {
	case d == "巨大":
		return "HUGE"
	case d == "高い":
		return "HIGH"
	case strings.Contains(d, "１０ｍ超") || strings.Contains(d, "10m超"):
		return "10<span class='indicator'>m</span> Above"
	case strings.Contains(d, "１０ｍ") || value == "10":
		return "10<span class='indicator'>m</span>"
	case strings.Contains(d, "５ｍ") || value == "5":
		return "5<span class='indicator'>m</span>"
	case strings.Contains(d, "３ｍ") || value == "3":
		return "3<span class='indicator'>m</span>"
	case strings.Contains(d, "１ｍ") || value == "1":
		return "1<span class='indicator'>m</span>"
	case strings.Contains(d, "０．２ｍ未満") || value == "0.2":
		return "Below 0.2m"
	default:
		return "Unknown"
	}
}
func tsunamiTime(v jmaxml.FirstHeight) model.TsunamiTime {
	condition := strings.TrimSpace(v.Condition)
	if condition != "" {
		status := -1
		text := "Unknown"
		switch {
		case strings.Contains(condition, "ただちに") || strings.Contains(condition, "到達中"):
			status = 0
			text = "Arriving Now"
		case strings.Contains(condition, "到達と推測"):
			status = 1
			text = "Arrival Expected"
		case strings.Contains(condition, "到達を確認"):
			status = 2
			text = "Arrived"
		}
		return model.TsunamiTime{Type: "no_time", Time: text, Status: &status}
	}
	if !v.ArrivalTime.Time.IsZero() {
		return model.TsunamiTime{Type: "time", Time: v.ArrivalTime.Format("01-02 15:04"), Timestamp: new(v.ArrivalTime.Unix())}
	}
	return model.TsunamiTime{Type: "no_time", Time: "Unknown", Status: new(-1)}
}
func observationArea(v jmaxml.TsunamiStation) model.TsunamiObservationArea {
	condition := "None"
	if strings.Contains(v.MaxHeight.Condition, "微弱") {
		condition = "Weak"
	}
	if strings.Contains(v.MaxHeight.Condition, "観測中") {
		condition = "Observing"
	}
	heightCondition := "None"
	if strings.Contains(v.MaxHeight.TsunamiHeight.Condition, "上昇") {
		heightCondition = "Rising"
	}
	tm := "None"
	if !v.MaxHeight.DateTime.Time.IsZero() {
		tm = v.MaxHeight.DateTime.Format("01-02 15:04")
	}
	height := strings.TrimSpace(v.MaxHeight.TsunamiHeight.Value)
	if height == "" {
		height = "None"
	}
	return model.TsunamiObservationArea{Name: v.Name, Height: height, Time: tm, Condition: condition, HeightCondition: heightCondition, HeightIsMax: strings.Contains(v.MaxHeight.TsunamiHeight.Description, "以上")}
}

// JSONReportToXML accepts xmltodict-style JSON used by the legacy test corpus.
func JSONReportToXML(raw []byte) ([]byte, error) {
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("dmdata: decode JSON report: %w", err)
	}
	report, ok := root["Report"]
	if !ok {
		return nil, errors.New("dmdata: JSON has no Report root")
	}
	var out bytes.Buffer
	enc := xml.NewEncoder(&out)
	defer enc.Close()
	if err := encodeJSONElement(enc, "Report", report); err != nil {
		return nil, err
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
func encodeJSONElement(enc *xml.Encoder, name string, value any) error {
	name = localName(name)
	if values, ok := value.([]any); ok {
		for _, item := range values {
			if err := encodeJSONElement(enc, name, item); err != nil {
				return err
			}
		}
		return nil
	}
	start := xml.StartElement{Name: xml.Name{Local: name}}
	obj, isObj := value.(map[string]any)
	if isObj {
		for key, item := range obj {
			if strings.HasPrefix(key, "@") && !strings.HasPrefix(key, "@xmlns") {
				start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: localName(strings.TrimPrefix(key, "@"))}, Value: fmt.Sprint(item)})
			}
		}
	}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if isObj {
		if text, ok := obj["#text"]; ok {
			if err := enc.EncodeToken(xml.CharData(fmt.Sprint(text))); err != nil {
				return err
			}
		}
		for key, item := range obj {
			if strings.HasPrefix(key, "@") || key == "#text" {
				continue
			}
			if err := encodeJSONElement(enc, key, item); err != nil {
				return err
			}
		}
	} else if value != nil {
		if err := enc.EncodeToken(xml.CharData(fmt.Sprint(value))); err != nil {
			return err
		}
	}
	return enc.EncodeToken(start.End())
}
func localName(v string) string {
	if i := strings.IndexByte(v, ':'); i >= 0 {
		return v[i+1:]
	}
	return v
}
