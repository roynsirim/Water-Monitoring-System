package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// ─── Water Types ──────────────────────────────────────────────────────────────

type WaterType string

const (
	TownsWater WaterType = "towns"
	RiverWater WaterType = "river"
	BothWater  WaterType = "both"
)

// ─── Sites ────────────────────────────────────────────────────────────────────

const (
	SiteRotherham    = "rotherham"
	SiteStocksbridge = "stocksbridge"
	SiteBrinsworth   = "brinsworth"
	SiteWednesbury   = "wednesbury"
)

type Site struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DefaultSites returns all available sites
func DefaultSites() []Site {
	return []Site{
		{ID: SiteRotherham, Name: "Rotherham"},
		{ID: SiteStocksbridge, Name: "Stocksbridge"},
		{ID: SiteBrinsworth, Name: "Brinsworth"},
		{ID: SiteWednesbury, Name: "Wednesbury"},
	}
}

// ─── Departments ──────────────────────────────────────────────────────────────

const (
	DeptTCM                 = "TCM"
	DeptACP                 = "ACP"
	DeptBBR                 = "BBR"
	DeptEngineeringServices = "Engineering Services"
	DeptGeneral             = "General" // Used by other sites
	// Stocksbridge
	DeptRemelt     = "Remelt"
	DeptBilletMill = "Billet Mill"
	DeptWestBank   = "West Bank"
	DeptEastBank   = "East Bank"
	DeptSpringBank = "Spring Bank"
	DeptCroft      = "Croft"
	// Brinsworth
	DeptHotMill  = "Hot Mill"
	DeptColdMill = "Cold Mill"
	DeptHRP      = "HRP"
	// Wednesbury
	DeptProduction = "Production"
	DeptAmenities  = "Amenities"
)

// ─── Data Sources ─────────────────────────────────────────────────────────────

const (
	SourceManual    = "manual"
	SourceEEmon     = "eemon"
	SourceTrend     = "trend"
	SourceEstimated = "estimated"
)

// ─── Meter ────────────────────────────────────────────────────────────────────

type Meter struct {
	ID          string    `json:"id"`
	SiteID      string    `json:"site_id"`
	Department  string    `json:"department"`
	Name        string    `json:"name"`
	WaterType   WaterType `json:"water_type"`
	Feed        string    `json:"feed"`
	Source      string    `json:"source"` // manual, eemon, trend
	Unit        string    `json:"unit"`   // m3, litres
	IsActive    bool      `json:"is_active"`
	IsMainMeter bool      `json:"is_main_meter,omitempty"` // true for main meters
}

// ─── Reading ──────────────────────────────────────────────────────────────────

type Reading struct {
	ID          string    `json:"id"`
	MeterID     string    `json:"meter_id"`
	SiteID      string    `json:"site_id"`
	Value       float64   `json:"value"`
	Usage       float64   `json:"usage"`
	Date        time.Time `json:"date"`
	Source      string    `json:"source"`
	Notes       string    `json:"notes,omitempty"`
	WaterType   WaterType `json:"water_type"`
	IsEstimated bool      `json:"is_estimated,omitempty"`
}

// ─── Tonnes Entry ─────────────────────────────────────────────────────────────

type TonnesEntry struct {
	ID         string    `json:"id"`
	SiteID     string    `json:"site_id"`
	Department string    `json:"department"`
	Tonnes     float64   `json:"tonnes"`
	Date       time.Time `json:"date"`
	Notes      string    `json:"notes,omitempty"`
}

// UnmarshalJSON provides custom JSON unmarshaling to handle date formats from HTML date inputs (YYYY-MM-DD)
func (t *TonnesEntry) UnmarshalJSON(data []byte) error {
	type Alias TonnesEntry
	aux := &struct {
		Date string `json:"date"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	// Parse date from YYYY-MM-DD format or RFC3339
	if aux.Date != "" {
		// Try parsing as YYYY-MM-DD format first (from HTML date input)
		if parsedTime, err := time.Parse("2006-01-02", aux.Date); err == nil {
			t.Date = parsedTime
		} else {
			// Fall back to RFC3339 format
			if parsedTime, err := time.Parse(time.RFC3339, aux.Date); err == nil {
				t.Date = parsedTime
			} else {
				return fmt.Errorf("invalid date format: %s", aux.Date)
			}
		}
	}
	return nil
}

// ─── User Preferences ─────────────────────────────────────────────────────────

type UserPreferences struct {
	Theme            string `json:"theme"` // dark, light
	AutoFillEnabled  bool   `json:"auto_fill_enabled"`
	DefaultDateRange int    `json:"default_date_range"` // Days
}

// DefaultPreferences returns default user preferences
func DefaultPreferences() UserPreferences {
	return UserPreferences{
		Theme:            "dark",
		AutoFillEnabled:  false,
		DefaultDateRange: 90,
	}
}

// ─── Connection Status ────────────────────────────────────────────────────────

type ConnectionStatus struct {
	EEmon      bool      `json:"eemon"`
	Trend      bool      `json:"trend"`
	LastCheck  time.Time `json:"last_check"`
	EEmonError string    `json:"eemon_error,omitempty"`
	TrendError string    `json:"trend_error,omitempty"`
}

// ─── Dashboard Data ───────────────────────────────────────────────────────────

type DashboardData struct {
	SiteID         string        `json:"site_id"`
	SiteName       string        `json:"site_name"`
	TotalM3        float64       `json:"total_m3"`
	TownsM3        float64       `json:"towns_m3"`
	RiverM3        float64       `json:"river_m3"`
	TotalTonnes    float64       `json:"total_tonnes"`
	M3PerTonne     float64       `json:"m3_per_tonne"`
	Departments    []DeptSummary `json:"departments"`
	Timeline       []TimePoint   `json:"timeline"`
	WaterTypeSplit []PieSlice    `json:"water_type_split"`
}

type DeptSummary struct {
	Department string    `json:"department"`
	M3         float64   `json:"m3"`
	TownsM3    float64   `json:"towns_m3"`
	RiverM3    float64   `json:"river_m3"`
	Tonnes     float64   `json:"tonnes"`
	M3PerTonne float64   `json:"m3_per_tonne"`
	WaterType  WaterType `json:"water_type"`
}

type TimePoint struct {
	Date  string  `json:"date"`
	Towns float64 `json:"towns"`
	River float64 `json:"river"`
	Total float64 `json:"total"`
}

type PieSlice struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

// ─── KPI Data ─────────────────────────────────────────────────────────────────

type SiteKPI struct {
	SiteID     string  `json:"site_id"`
	SiteName   string  `json:"site_name"`
	TotalM3    float64 `json:"total_m3"`
	TownsM3    float64 `json:"towns_m3"`
	RiverM3    float64 `json:"river_m3"`
	Tonnes     float64 `json:"tonnes"`
	M3PerTonne float64 `json:"m3_per_tonne"`
}

// ─── Search Result ────────────────────────────────────────────────────────────

type SearchResult struct {
	Reading    Reading `json:"reading"`
	MeterName  string  `json:"meter_name"`
	Department string  `json:"department"`
	SiteName   string  `json:"site_name"`
	M3         float64 `json:"m3"`
}

// ─── Report ───────────────────────────────────────────────────────────────────

type Report struct {
	Title     string       `json:"title"`
	Period    string       `json:"period"`
	Sites     []SiteReport `json:"sites"`
	Generated time.Time    `json:"generated"`
}

type SiteReport struct {
	SiteName    string        `json:"site_name"`
	TotalM3     float64       `json:"total_m3"`
	TownsM3     float64       `json:"towns_m3"`
	RiverM3     float64       `json:"river_m3"`
	TotalTonnes float64       `json:"total_tonnes"`
	M3PerTonne  float64       `json:"m3_per_tonne"`
	Departments []DeptSummary `json:"departments"`
}
