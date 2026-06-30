// Package jmaxml is the Go representation of the JMA XML schemas under /xsd.
// It covers the generic report/control/head schemas (jmx.xsd, jmx_ib.xsd),
// element-basis scalar values (jmx_eb.xsd), and the complete earthquake,
// intensity, EEW, and tsunami branches used by this service (jmx_seis.xsd).
package jmaxml

import (
	"encoding/xml"
	"strings"
	"time"
)

// DateTime corresponds to xs:dateTime. JMA occasionally emits a nillable or
// blank TargetDateTime, so zero time is accepted.
type DateTime struct{ time.Time }

func (d *DateTime) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var value string
	if err := dec.DecodeElement(&value, &start); err != nil {
		return err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		d.Time = time.Time{}
		return nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

type Report struct {
	XMLName xml.Name `xml:"Report"`
	Control Control  `xml:"Control"`
	Head    Head     `xml:"Head"`
	Body    Body     `xml:"Body"`
}

type Control struct {
	Title            string   `xml:"Title"`
	DateTime         DateTime `xml:"DateTime"`
	Status           string   `xml:"Status"`
	EditorialOffice  string   `xml:"EditorialOffice"`
	PublishingOffice string   `xml:"PublishingOffice"`
}

type Head struct {
	Title           string   `xml:"Title"`
	ReportDateTime  DateTime `xml:"ReportDateTime"`
	TargetDateTime  DateTime `xml:"TargetDateTime"`
	TargetDTDubious string   `xml:"TargetDTDubious"`
	TargetDuration  string   `xml:"TargetDuration"`
	ValidDateTime   DateTime `xml:"ValidDateTime"`
	EventID         string   `xml:"EventID"`
	InfoType        string   `xml:"InfoType"`
	Serial          string   `xml:"Serial"`
	InfoKind        string   `xml:"InfoKind"`
	InfoKindVersion string   `xml:"InfoKindVersion"`
	Headline        Headline `xml:"Headline"`
}

type Headline struct {
	Text        string        `xml:"Text"`
	Information []Information `xml:"Information"`
}

type Information struct {
	Type  string            `xml:"type,attr"`
	Items []InformationItem `xml:"Item"`
}

type InformationItem struct {
	Kinds     []Kind    `xml:"Kind"`
	LastKinds []Kind    `xml:"LastKind"`
	Areas     HeadAreas `xml:"Areas"`
}

type HeadAreas struct {
	CodeType string     `xml:"codeType,attr"`
	Areas    []NameCode `xml:"Area"`
}

// ElementValue is the XSD simple-content pattern used by jmx_eb values.
type ElementValue struct {
	Value       string `xml:",chardata"`
	Type        string `xml:"type,attr"`
	Unit        string `xml:"unit,attr"`
	Condition   string `xml:"condition,attr"`
	Description string `xml:"description,attr"`
	Datum       string `xml:"datum,attr"`
}

type Body struct {
	Naming       *Naming      `xml:"Naming"`
	Tsunami      *Tsunami     `xml:"Tsunami"`
	Earthquakes  []Earthquake `xml:"Earthquake"`
	Intensity    *Intensity   `xml:"Intensity"`
	Text         string       `xml:"Text"`
	NextAdvisory string       `xml:"NextAdvisory"`
	Comments     *Comments    `xml:"Comments"`
}

type Naming struct {
	Name string `xml:"Name"`
	Code string `xml:"Code"`
}

type Earthquake struct {
	OriginTime  DateTime       `xml:"OriginTime"`
	ArrivalTime DateTime       `xml:"ArrivalTime"`
	Condition   string         `xml:"Condition"`
	Hypocenter  Hypocenter     `xml:"Hypocenter"`
	Magnitudes  []ElementValue `xml:"Magnitude"`
}

type Hypocenter struct {
	Area     HypoArea `xml:"Area"`
	Source   string   `xml:"Source"`
	Accuracy Accuracy `xml:"Accuracy"`
}

type HypoArea struct {
	Name         string         `xml:"Name"`
	Code         ElementValue   `xml:"Code"`
	Coordinates  []ElementValue `xml:"Coordinate"`
	ReduceName   string         `xml:"ReduceName"`
	ReduceCode   ElementValue   `xml:"ReduceCode"`
	DetailedName string         `xml:"DetailedName"`
	DetailedCode ElementValue   `xml:"DetailedCode"`
	NameFromMark string         `xml:"NameFromMark"`
	MarkCode     ElementValue   `xml:"MarkCode"`
	Direction    string         `xml:"Direction"`
	Distance     ElementValue   `xml:"Distance"`
	LandOrSea    string         `xml:"LandOrSea"`
}

type Accuracy struct {
	Epicenter                    RankedValue `xml:"Epicenter"`
	Depth                        RankedValue `xml:"Depth"`
	MagnitudeCalculation         RankedValue `xml:"MagnitudeCalculation"`
	NumberOfMagnitudeCalculation int         `xml:"NumberOfMagnitudeCalculation"`
}

type RankedValue struct {
	Value string `xml:",chardata"`
	Rank  string `xml:"rank,attr"`
	Rank2 string `xml:"rank2,attr"`
}

type Intensity struct {
	Forecast    *IntensityDetail `xml:"Forecast"`
	Observation *IntensityDetail `xml:"Observation"`
}

type IntensityDetail struct {
	CodeDefine    CodeDefine        `xml:"CodeDefine"`
	MaxInt        string            `xml:"MaxInt"`
	MaxLGInt      string            `xml:"MaxLgInt"`
	LGCategory    string            `xml:"LgCategory"`
	ForecastInt   ForecastRange     `xml:"ForecastInt"`
	ForecastLGInt ForecastRange     `xml:"ForecastLgInt"`
	Appendix      IntensityAppendix `xml:"Appendix"`
	Prefs         []IntensityPref   `xml:"Pref"`
}

type CodeDefine struct {
	Types []CodeDefineType `xml:"Type"`
}
type CodeDefineType struct {
	Value string `xml:",chardata"`
	XPath string `xml:"xpath,attr"`
}

type ForecastRange struct {
	From string `xml:"From"`
	To   string `xml:"To"`
}

type IntensityAppendix struct {
	MaxIntChange         int `xml:"MaxIntChange"`
	MaxLGIntChange       int `xml:"MaxLgIntChange"`
	MaxIntChangeReason   int `xml:"MaxIntChangeReason"`
	MaxLGIntChangeReason int `xml:"MaxLgIntChangeReason"`
}

type IntensityPref struct {
	Name          string          `xml:"Name"`
	Code          string          `xml:"Code"`
	Category      Category        `xml:"Category"`
	MaxInt        string          `xml:"MaxInt"`
	MaxLGInt      string          `xml:"MaxLgInt"`
	ForecastInt   ForecastRange   `xml:"ForecastInt"`
	ForecastLGInt ForecastRange   `xml:"ForecastLgInt"`
	ArrivalTime   DateTime        `xml:"ArrivalTime"`
	Condition     string          `xml:"Condition"`
	Revise        string          `xml:"Revise"`
	Areas         []IntensityArea `xml:"Area"`
}

type IntensityArea struct {
	Name          string             `xml:"Name"`
	Code          string             `xml:"Code"`
	Category      Category           `xml:"Category"`
	MaxInt        string             `xml:"MaxInt"`
	MaxLGInt      string             `xml:"MaxLgInt"`
	ForecastInt   ForecastRange      `xml:"ForecastInt"`
	ForecastLGInt ForecastRange      `xml:"ForecastLgInt"`
	ArrivalTime   DateTime           `xml:"ArrivalTime"`
	Condition     string             `xml:"Condition"`
	Revise        string             `xml:"Revise"`
	Cities        []IntensityCity    `xml:"City"`
	Stations      []IntensityStation `xml:"IntensityStation"`
}

type IntensityCity struct {
	Name          string             `xml:"Name"`
	Code          string             `xml:"Code"`
	Category      Category           `xml:"Category"`
	MaxInt        string             `xml:"MaxInt"`
	MaxLGInt      string             `xml:"MaxLgInt"`
	ForecastInt   ForecastRange      `xml:"ForecastInt"`
	ForecastLGInt ForecastRange      `xml:"ForecastLgInt"`
	ArrivalTime   DateTime           `xml:"ArrivalTime"`
	Condition     string             `xml:"Condition"`
	Revise        string             `xml:"Revise"`
	Stations      []IntensityStation `xml:"IntensityStation"`
}

type IntensityStation struct {
	Name           string        `xml:"Name"`
	Code           string        `xml:"Code"`
	Int            string        `xml:"Int"`
	K              float64       `xml:"K"`
	LGInt          string        `xml:"LgInt"`
	LGIntPerPeriod []PeriodValue `xml:"LgIntPerPeriod"`
	SVA            ElementValue  `xml:"Sva"`
	SVAPerPeriod   []PeriodValue `xml:"SvaPerPeriod"`
	Revise         string        `xml:"Revise"`
}

type PeriodValue struct {
	Value  string `xml:",chardata"`
	Period string `xml:"period,attr"`
	Unit   string `xml:"unit,attr"`
}

type Tsunami struct {
	Release     string         `xml:"Release"`
	Observation *TsunamiDetail `xml:"Observation"`
	Estimation  *TsunamiDetail `xml:"Estimation"`
	Forecast    *TsunamiDetail `xml:"Forecast"`
}

type TsunamiDetail struct {
	CodeDefine CodeDefine    `xml:"CodeDefine"`
	Items      []TsunamiItem `xml:"Item"`
}

type TsunamiItem struct {
	Area        ForecastArea     `xml:"Area"`
	Category    Category         `xml:"Category"`
	FirstHeight FirstHeight      `xml:"FirstHeight"`
	MaxHeight   MaxHeight        `xml:"MaxHeight"`
	Duration    string           `xml:"Duration"`
	Stations    []TsunamiStation `xml:"Station"`
}

type ForecastArea struct {
	Name   string     `xml:"Name"`
	Code   string     `xml:"Code"`
	Cities []NameCode `xml:"City"`
}

type NameCode struct {
	Name string `xml:"Name"`
	Code string `xml:"Code"`
}

type Category struct {
	Kind     Kind `xml:"Kind"`
	LastKind Kind `xml:"LastKind"`
}

type Kind struct {
	Name      string `xml:"Name"`
	Code      string `xml:"Code"`
	Condition string `xml:"Condition"`
}

type FirstHeight struct {
	ArrivalTimeFrom DateTime     `xml:"ArrivalTimeFrom"`
	ArrivalTimeTo   DateTime     `xml:"ArrivalTimeTo"`
	ArrivalTime     DateTime     `xml:"ArrivalTime"`
	Condition       string       `xml:"Condition"`
	Initial         string       `xml:"Initial"`
	TsunamiHeight   ElementValue `xml:"TsunamiHeight"`
	Revise          string       `xml:"Revise"`
	Period          float64      `xml:"Period"`
}

type MaxHeight struct {
	DateTime          DateTime     `xml:"DateTime"`
	Condition         string       `xml:"Condition"`
	TsunamiHeightFrom ElementValue `xml:"TsunamiHeightFrom"`
	TsunamiHeightTo   ElementValue `xml:"TsunamiHeightTo"`
	TsunamiHeight     ElementValue `xml:"TsunamiHeight"`
	Revise            string       `xml:"Revise"`
	Period            float64      `xml:"Period"`
}

type CurrentHeight struct {
	StartTime     DateTime     `xml:"StartTime"`
	EndTime       DateTime     `xml:"EndTime"`
	Condition     string       `xml:"Condition"`
	TsunamiHeight ElementValue `xml:"TsunamiHeight"`
}

type TsunamiStation struct {
	Name             string        `xml:"Name"`
	Code             string        `xml:"Code"`
	Sensor           string        `xml:"Sensor"`
	HighTideDateTime DateTime      `xml:"HighTideDateTime"`
	FirstHeight      FirstHeight   `xml:"FirstHeight"`
	MaxHeight        MaxHeight     `xml:"MaxHeight"`
	CurrentHeight    CurrentHeight `xml:"CurrentHeight"`
}

type Comments struct {
	WarningComment     *CommentForm `xml:"WarningComment"`
	ForecastComment    *CommentForm `xml:"ForecastComment"`
	ObservationComment *CommentForm `xml:"ObservationComment"`
	VarComment         *CommentForm `xml:"VarComment"`
	FreeFormComment    string       `xml:"FreeFormComment"`
	URI                string       `xml:"URI"`
}

type CommentForm struct {
	CodeType string `xml:"codeType,attr"`
	Text     string `xml:"Text"`
	Code     string `xml:"Code"`
}
