package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─── Models ──────────────────────────────────────────────────────────────────

type WaterType string

const (
	TownsWater WaterType = "towns"
	RiverWater WaterType = "river"
	Both       WaterType = "both"
)

type Site struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Meter struct {
	ID         string    `json:"id"`
	SiteID     string    `json:"site_id"`
	Department string    `json:"department"`
	Name       string    `json:"name"`
	WaterType  WaterType `json:"water_type"`
	Feed       string    `json:"feed"`
	Source     string    `json:"source"` // "manual","eemon","trend"
	Unit       string    `json:"unit"`   // "litres","m3"
}

type Reading struct {
	ID        string    `json:"id"`
	MeterID   string    `json:"meter_id"`
	SiteID    string    `json:"site_id"`
	Value     float64   `json:"value"`
	Usage     float64   `json:"usage"`
	Date      time.Time `json:"date"`
	Source    string    `json:"source"`
	Notes     string    `json:"notes,omitempty"`
	WaterType WaterType `json:"water_type"`
}

type TonnesEntry struct {
	ID         string    `json:"id"`
	SiteID     string    `json:"site_id"`
	Department string    `json:"department"`
	Tonnes     float64   `json:"tonnes"`
	Date       time.Time `json:"date"`
	Notes      string    `json:"notes,omitempty"`
}

type DB struct {
	mu       sync.RWMutex
	path     string
	Meters   []Meter       `json:"meters"`
	Readings []Reading     `json:"readings"`
	Tonnes   []TonnesEntry `json:"tonnes"`
}

// ─── Database ────────────────────────────────────────────────────────────────

var db *DB

func loadDB(path string) *DB {
	d := &DB{path: path}
	data, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(data, d)
	}
	if d.Meters == nil {
		d.Meters = seedMeters()
	}
	return d
}

func (d *DB) save() {
	data, _ := json.MarshalIndent(d, "", "  ")
	os.WriteFile(d.path, data, 0644)
}

func (d *DB) addReading(r Reading) {
	d.mu.Lock()
	defer d.mu.Unlock()
	r.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	// Calculate usage from last reading
	var last float64
	for i := len(d.Readings) - 1; i >= 0; i-- {
		if d.Readings[i].MeterID == r.MeterID {
			last = d.Readings[i].Value
			break
		}
	}
	if last > 0 && r.Value > last {
		r.Usage = r.Value - last
	}
	d.Readings = append(d.Readings, r)
	d.save()
}

func (d *DB) addTonnes(t TonnesEntry) {
	d.mu.Lock()
	defer d.mu.Unlock()
	t.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	d.Tonnes = append(d.Tonnes, t)
	d.save()
}

// ─── Seed Data ───────────────────────────────────────────────────────────────

func seedMeters() []Meter {
	return []Meter{
		// ── ROTHERHAM ──────────────────────────────────────────────────────
		// TCM Feed
		{ID: "r-tmg", SiteID: "rotherham", Department: "TCM", Name: "Thrybergh Main Gate", WaterType: TownsWater, Feed: "TCM Feed", Source: "manual", Unit: "m3"},
		{ID: "r-tgh", SiteID: "rotherham", Department: "TCM", Name: "Thrybergh Gate House", WaterType: TownsWater, Feed: "TCM Feed", Source: "manual", Unit: "m3"},
		{ID: "r-tbm", SiteID: "rotherham", Department: "TCM", Name: "TBM Tunnel", WaterType: TownsWater, Feed: "TCM Feed", Source: "manual", Unit: "m3"},
		{ID: "r-vc", SiteID: "rotherham", Department: "TCM", Name: "Visitors Centre + Transport", WaterType: TownsWater, Feed: "TCM Feed", Source: "manual", Unit: "m3"},
		{ID: "r-tbmr", SiteID: "rotherham", Department: "TCM", Name: "TBM River", WaterType: RiverWater, Feed: "TCM Feed", Source: "manual", Unit: "m3"},
		{ID: "r-rwtp", SiteID: "rotherham", Department: "TCM", Name: "RWTP", WaterType: Both, Feed: "TCM Feed", Source: "manual", Unit: "m3"},
		// ACP Department
		{ID: "r-acp", SiteID: "rotherham", Department: "ACP", Name: "ACP Main Feed", WaterType: TownsWater, Feed: "TCM Feed", Source: "manual", Unit: "m3"},
		{ID: "r-ks", SiteID: "rotherham", Department: "ACP", Name: "Kress Square", WaterType: TownsWater, Feed: "TCM Feed", Source: "manual", Unit: "m3"},
		{ID: "r-vd", SiteID: "rotherham", Department: "ACP", Name: "VAC Degasser", WaterType: TownsWater, Feed: "TCM Feed", Source: "manual", Unit: "m3"},
		{ID: "r-abcrw", SiteID: "rotherham", Department: "ACP", Name: "ABC RW", WaterType: RiverWater, Feed: "TCM Feed", Source: "manual", Unit: "m3"},
		{ID: "r-abctw", SiteID: "rotherham", Department: "ACP", Name: "ABC Towns Water", WaterType: TownsWater, Feed: "TCM Feed", Source: "manual", Unit: "m3"},
		{ID: "r-ams", SiteID: "rotherham", Department: "ACP", Name: "AMS Amenities", WaterType: TownsWater, Feed: "TCM Feed", Source: "manual", Unit: "m3"},
		// BBR Department
		{ID: "r-bbr", SiteID: "rotherham", Department: "BBR", Name: "BBR", WaterType: TownsWater, Feed: "Aldwarke Lane", Source: "manual", Unit: "m3"},
		{ID: "r-bs", SiteID: "rotherham", Department: "BBR", Name: "Bike Shed", WaterType: TownsWater, Feed: "Aldwarke Lane", Source: "manual", Unit: "m3"},
		{ID: "r-aocc", SiteID: "rotherham", Department: "BBR", Name: "AOCC", WaterType: TownsWater, Feed: "Aldwarke Lane", Source: "manual", Unit: "m3"},
		// Aldwarke Lane Feed
		{ID: "r-boc", SiteID: "rotherham", Department: "Aldwarke", Name: "BOC", WaterType: TownsWater, Feed: "Aldwarke Lane", Source: "manual", Unit: "m3"},
		{ID: "r-cgl", SiteID: "rotherham", Department: "Aldwarke", Name: "Cap Gemini - Car Scrap (Left)", WaterType: TownsWater, Feed: "Aldwarke Lane", Source: "manual", Unit: "m3"},
		{ID: "r-cgr", SiteID: "rotherham", Department: "Aldwarke", Name: "Cap Gemini - Car Scrap (Right)", WaterType: TownsWater, Feed: "Aldwarke Lane", Source: "manual", Unit: "m3"},
		{ID: "r-th", SiteID: "rotherham", Department: "Aldwarke", Name: "Test House", WaterType: TownsWater, Feed: "Aldwarke Lane", Source: "manual", Unit: "m3"},
		{ID: "r-jw", SiteID: "rotherham", Department: "TCM", Name: "Jet Wash (Transport)", WaterType: TownsWater, Feed: "TCM Feed", Source: "manual", Unit: "m3"},
		// Green Lane Feed
		{ID: "r-cew", SiteID: "rotherham", Department: "Green Lane", Name: "CEW Graveyard", WaterType: TownsWater, Feed: "Green Lane", Source: "manual", Unit: "m3"},
		{ID: "r-ago", SiteID: "rotherham", Department: "Green Lane", Name: "AGO", WaterType: TownsWater, Feed: "Green Lane", Source: "manual", Unit: "m3"},

		// ── STOCKSBRIDGE ───────────────────────────────────────────────────
		// Towns Water - East Bank
		{ID: "s-rms1", SiteID: "stocksbridge", Department: "Remelt", Name: "RMS 1", WaterType: TownsWater, Feed: "East Bank", Source: "eemon", Unit: "litres"},
		{ID: "s-rms2", SiteID: "stocksbridge", Department: "Remelt", Name: "RMS 2", WaterType: TownsWater, Feed: "East Bank", Source: "eemon", Unit: "litres"},
		{ID: "s-vim", SiteID: "stocksbridge", Department: "Remelt", Name: "VIM", WaterType: TownsWater, Feed: "East Bank", Source: "eemon", Unit: "litres"},
		{ID: "s-bm16", SiteID: "stocksbridge", Department: "Billet Mill", Name: "BM Meter 16", WaterType: TownsWater, Feed: "East Bank", Source: "eemon", Unit: "litres"},
		{ID: "s-bm17", SiteID: "stocksbridge", Department: "Billet Mill", Name: "BM Meter 17", WaterType: TownsWater, Feed: "East Bank", Source: "eemon", Unit: "litres"},
		{ID: "s-bms14", SiteID: "stocksbridge", Department: "Billet Mill", Name: "BM Saws Meter 14", WaterType: TownsWater, Feed: "East Bank", Source: "eemon", Unit: "litres"},
		{ID: "s-croft", SiteID: "stocksbridge", Department: "Croft", Name: "Croft Supply", WaterType: TownsWater, Feed: "East Bank", Source: "eemon", Unit: "litres"},
		// River Water - West Bank
		{ID: "s-wq1", SiteID: "stocksbridge", Department: "West Bank", Name: "Water Quench 1", WaterType: RiverWater, Feed: "West Bank", Source: "eemon", Unit: "litres"},
		{ID: "s-wq2", SiteID: "stocksbridge", Department: "West Bank", Name: "Water Quench 2", WaterType: RiverWater, Feed: "West Bank", Source: "eemon", Unit: "litres"},
		{ID: "s-sb", SiteID: "stocksbridge", Department: "Spring Bank", Name: "Spring Bank", WaterType: RiverWater, Feed: "Spring Bank", Source: "eemon", Unit: "litres"},
		// Manual meters
		{ID: "s-wbg", SiteID: "stocksbridge", Department: "West Bank", Name: "West Bank Gardens", WaterType: TownsWater, Feed: "West Bank", Source: "manual", Unit: "m3"},
		{ID: "s-g2", SiteID: "stocksbridge", Department: "General", Name: "No.2 Gate Meter", WaterType: TownsWater, Feed: "East Bank", Source: "manual", Unit: "m3"},
		{ID: "s-g1", SiteID: "stocksbridge", Department: "General", Name: "No.1 Gate (Hawthorne Brook)", WaterType: TownsWater, Feed: "East Bank", Source: "manual", Unit: "m3"},
		{ID: "s-ebte", SiteID: "stocksbridge", Department: "East Bank", Name: "EB Trade Effluent", WaterType: TownsWater, Feed: "East Bank", Source: "manual", Unit: "m3"},
		{ID: "s-ebn", SiteID: "stocksbridge", Department: "East Bank", Name: "EB Network", WaterType: TownsWater, Feed: "East Bank", Source: "manual", Unit: "m3"},
		{ID: "s-ebt", SiteID: "stocksbridge", Department: "East Bank", Name: "EB Tank Meter", WaterType: TownsWater, Feed: "East Bank", Source: "manual", Unit: "m3"},
		// Other EEmon
		{ID: "s-wboq", SiteID: "stocksbridge", Department: "West Bank", Name: "West Bank Oil Quench", WaterType: TownsWater, Feed: "West Bank", Source: "eemon", Unit: "litres"},
		{ID: "s-wbcht", SiteID: "stocksbridge", Department: "West Bank", Name: "West Bank CHT", WaterType: TownsWater, Feed: "West Bank", Source: "eemon", Unit: "litres"},
		{ID: "s-bbr", SiteID: "stocksbridge", Department: "Billet Mill", Name: "Billet Bank Reeler", WaterType: TownsWater, Feed: "East Bank", Source: "eemon", Unit: "litres"},
		{ID: "s-esr1", SiteID: "stocksbridge", Department: "East Bank", Name: "ESR 1", WaterType: TownsWater, Feed: "East Bank", Source: "eemon", Unit: "litres"},
		{ID: "s-eb", SiteID: "stocksbridge", Department: "East Bank", Name: "East Bank Total", WaterType: TownsWater, Feed: "East Bank", Source: "eemon", Unit: "litres"},

		// ── BRINSWORTH ─────────────────────────────────────────────────────
		{ID: "b-ttw", SiteID: "brinsworth", Department: "General", Name: "Total Towns Water", WaterType: TownsWater, Feed: "Towns", Source: "manual", Unit: "m3"},
		{ID: "b-glr", SiteID: "brinsworth", Department: "General", Name: "Grange Lane Reservoir (Riverside Pumphouse)", WaterType: RiverWater, Feed: "River", Source: "manual", Unit: "m3"},
		{ID: "b-hmr", SiteID: "brinsworth", Department: "Hot Mill", Name: "Hot Mill River Water Meter", WaterType: RiverWater, Feed: "River", Source: "trend", Unit: "m3"},
		{ID: "b-hrp", SiteID: "brinsworth", Department: "HRP", Name: "HRP River Water Meter", WaterType: RiverWater, Feed: "River", Source: "trend", Unit: "m3"},
		{ID: "b-cmr", SiteID: "brinsworth", Department: "Cold Mill", Name: "Cold Mill River Water Meter", WaterType: RiverWater, Feed: "River", Source: "trend", Unit: "m3"},

		// ── WEDNESBURY ─────────────────────────────────────────────────────
		{ID: "w-m1", SiteID: "wednesbury", Department: "General", Name: "Main Meter 1", WaterType: TownsWater, Feed: "Mains", Source: "manual", Unit: "m3"},
		{ID: "w-m2", SiteID: "wednesbury", Department: "General", Name: "Main Meter 2", WaterType: TownsWater, Feed: "Mains", Source: "manual", Unit: "m3"},
		{ID: "w-pro", SiteID: "wednesbury", Department: "Production", Name: "Production Meter", WaterType: TownsWater, Feed: "Mains", Source: "manual", Unit: "m3"},
		{ID: "w-ame", SiteID: "wednesbury", Department: "Amenities", Name: "Amenities Meter", WaterType: TownsWater, Feed: "Mains", Source: "manual", Unit: "m3"},
	}
}

var sites = []Site{
	{ID: "rotherham", Name: "Rotherham"},
	{ID: "stocksbridge", Name: "Stocksbridge"},
	{ID: "brinsworth", Name: "Brinsworth"},
	{ID: "wednesbury", Name: "Wednesbury"},
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toM3(v float64, unit string) float64 {
	if unit == "litres" {
		return v / 1000.0
	}
	return v
}

func respond(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func parseDate(s string) time.Time {
	formats := []string{"2006-01-02", "02/01/2006", time.RFC3339}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Now()
}

// ─── Handlers ────────────────────────────────────────────────────────────────

func handleSites(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, sites)
}

func handleMeters(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	db.mu.RLock()
	defer db.mu.RUnlock()
	result := []Meter{}
	for _, m := range db.Meters {
		if siteID == "" || m.SiteID == siteID {
			result = append(result, m)
		}
	}
	respond(w, 200, result)
}

func handleReadings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var body struct {
			MeterID string  `json:"meter_id"`
			Value   float64 `json:"value"`
			Date    string  `json:"date"`
			Notes   string  `json:"notes"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		// Find meter
		db.mu.RLock()
		var meter *Meter
		for i := range db.Meters {
			if db.Meters[i].ID == body.MeterID {
				meter = &db.Meters[i]
				break
			}
		}
		db.mu.RUnlock()
		if meter == nil {
			respond(w, 400, map[string]string{"error": "meter not found"})
			return
		}
		db.addReading(Reading{
			MeterID:   body.MeterID,
			SiteID:    meter.SiteID,
			Value:     body.Value,
			Date:      parseDate(body.Date),
			Source:    "manual",
			Notes:     body.Notes,
			WaterType: meter.WaterType,
		})
		respond(w, 201, map[string]string{"status": "ok"})
		return
	}

	// GET
	q := r.URL.Query()
	siteID := q.Get("site_id")
	meterID := q.Get("meter_id")
	from := q.Get("from")
	to := q.Get("to")

	db.mu.RLock()
	defer db.mu.RUnlock()

	var fromT, toT time.Time
	if from != "" {
		fromT = parseDate(from)
	}
	if to != "" {
		toT = parseDate(to)
	} else {
		toT = time.Now().Add(24 * time.Hour)
	}

	result := []Reading{}
	for _, rd := range db.Readings {
		if siteID != "" && rd.SiteID != siteID {
			continue
		}
		if meterID != "" && rd.MeterID != meterID {
			continue
		}
		if !fromT.IsZero() && rd.Date.Before(fromT) {
			continue
		}
		if rd.Date.After(toT) {
			continue
		}
		result = append(result, rd)
	}
	respond(w, 200, result)
}

func handleTonnes(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var body TonnesEntry
		json.NewDecoder(r.Body).Decode(&body)
		if body.Date.IsZero() {
			body.Date = time.Now()
		}
		db.addTonnes(body)
		respond(w, 201, map[string]string{"status": "ok"})
		return
	}

	q := r.URL.Query()
	siteID := q.Get("site_id")
	from := q.Get("from")
	to := q.Get("to")

	db.mu.RLock()
	defer db.mu.RUnlock()

	var fromT, toT time.Time
	if from != "" {
		fromT = parseDate(from)
	}
	if to != "" {
		toT = parseDate(to)
	} else {
		toT = time.Now().Add(24 * time.Hour)
	}

	result := []TonnesEntry{}
	for _, t := range db.Tonnes {
		if siteID != "" && t.SiteID != siteID {
			continue
		}
		if !fromT.IsZero() && t.Date.Before(fromT) {
			continue
		}
		if t.Date.After(toT) {
			continue
		}
		result = append(result, t)
	}
	respond(w, 200, result)
}

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

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	var fromT time.Time
	toT := time.Now().Add(24 * time.Hour)
	if from != "" {
		fromT = parseDate(from)
	} else {
		fromT = time.Now().AddDate(0, -3, 0)
	}
	if to != "" {
		toT = parseDate(to)
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	// Build meter map
	meterMap := map[string]Meter{}
	for _, m := range db.Meters {
		meterMap[m.ID] = m
	}

	type deptKey struct{ site, dept string }
	deptM3 := map[deptKey]float64{}
	deptTowns := map[deptKey]float64{}
	deptRiver := map[deptKey]float64{}

	// Weekly timeline buckets
	type weekKey struct{ site, week string }
	weekTowns := map[weekKey]float64{}
	weekRiver := map[weekKey]float64{}

	siteTotals := map[string]struct{ towns, river float64 }{}

	for _, rd := range db.Readings {
		if siteID != "" && rd.SiteID != siteID {
			continue
		}
		if rd.Date.Before(fromT) || rd.Date.After(toT) {
			continue
		}
		m, ok := meterMap[rd.MeterID]
		if !ok {
			continue
		}
		usageM3 := toM3(rd.Usage, m.Unit)
		if usageM3 <= 0 {
			continue
		}

		dk := deptKey{rd.SiteID, m.Department}
		wk := weekKey{rd.SiteID, rd.Date.Format("2006-W") + fmt.Sprintf("%02d", rd.Date.Day()/7+1)}

		deptM3[dk] += usageM3
		st := siteTotals[rd.SiteID]
		switch rd.WaterType {
		case TownsWater:
			deptTowns[dk] += usageM3
			weekTowns[wk] += usageM3
			st.towns += usageM3
		case RiverWater:
			deptRiver[dk] += usageM3
			weekRiver[wk] += usageM3
			st.river += usageM3
		case Both:
			deptTowns[dk] += usageM3 / 2
			deptRiver[dk] += usageM3 / 2
			weekTowns[wk] += usageM3 / 2
			weekRiver[wk] += usageM3 / 2
			st.towns += usageM3 / 2
			st.river += usageM3 / 2
		}
		siteTotals[rd.SiteID] = st
	}

	// Tonnes by dept
	deptTonnes := map[deptKey]float64{}
	siteTonnesTotal := map[string]float64{}
	for _, t := range db.Tonnes {
		if siteID != "" && t.SiteID != siteID {
			continue
		}
		if t.Date.Before(fromT) || t.Date.After(toT) {
			continue
		}
		dk := deptKey{t.SiteID, t.Department}
		deptTonnes[dk] += t.Tonnes
		siteTonnesTotal[t.SiteID] += t.Tonnes
	}

	buildSite := func(sid, sname string) DashboardData {
		d := DashboardData{SiteID: sid, SiteName: sname}
		st := siteTotals[sid]
		d.TownsM3 = math.Round(st.towns*100) / 100
		d.RiverM3 = math.Round(st.river*100) / 100
		d.TotalM3 = math.Round((st.towns+st.river)*100) / 100
		d.TotalTonnes = math.Round(siteTonnesTotal[sid]*100) / 100
		if d.TotalTonnes > 0 {
			d.M3PerTonne = math.Round(d.TotalM3/d.TotalTonnes*1000) / 1000
		}

		// Departments
		seen := map[string]bool{}
		for _, m := range db.Meters {
			if m.SiteID != sid {
				continue
			}
			if seen[m.Department] {
				continue
			}
			seen[m.Department] = true
			dk := deptKey{sid, m.Department}
			m3 := math.Round(deptM3[dk]*100) / 100
			tw := math.Round(deptTowns[dk]*100) / 100
			rv := math.Round(deptRiver[dk]*100) / 100
			tn := math.Round(deptTonnes[dk]*100) / 100
			var m3t float64
			if tn > 0 {
				m3t = math.Round(m3/tn*1000) / 1000
			}
			d.Departments = append(d.Departments, DeptSummary{
				Department: m.Department,
				M3:         m3,
				TownsM3:    tw,
				RiverM3:    rv,
				Tonnes:     tn,
				M3PerTonne: m3t,
				WaterType:  m.WaterType,
			})
		}

		// Timeline - weekly aggregation
		weeks := map[string][2]float64{}
		for wk, v := range weekTowns {
			if wk.site != sid {
				continue
			}
			arr := weeks[wk.week]
			arr[0] += v
			weeks[wk.week] = arr
		}
		for wk, v := range weekRiver {
			if wk.site != sid {
				continue
			}
			arr := weeks[wk.week]
			arr[1] += v
			weeks[wk.week] = arr
		}
		wkeys := make([]string, 0, len(weeks))
		for k := range weeks {
			wkeys = append(wkeys, k)
		}
		sort.Strings(wkeys)
		for _, wk := range wkeys {
			v := weeks[wk]
			d.Timeline = append(d.Timeline, TimePoint{
				Date:  wk,
				Towns: math.Round(v[0]*100) / 100,
				River: math.Round(v[1]*100) / 100,
				Total: math.Round((v[0]+v[1])*100) / 100,
			})
		}

		d.WaterTypeSplit = []PieSlice{
			{Label: "Towns Water", Value: d.TownsM3},
			{Label: "River Water", Value: d.RiverM3},
		}
		return d
	}

	if siteID != "" {
		for _, s := range sites {
			if s.ID == siteID {
				respond(w, 200, buildSite(s.ID, s.Name))
				return
			}
		}
		respond(w, 404, map[string]string{"error": "site not found"})
		return
	}

	result := []DashboardData{}
	for _, s := range sites {
		result = append(result, buildSite(s.ID, s.Name))
	}
	respond(w, 200, result)
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(r.URL.Query().Get("q"))
	siteID := r.URL.Query().Get("site_id")
	dateFrom := r.URL.Query().Get("from")
	dateTo := r.URL.Query().Get("to")
	wt := r.URL.Query().Get("water_type")

	db.mu.RLock()
	defer db.mu.RUnlock()

	meterMap := map[string]Meter{}
	for _, m := range db.Meters {
		meterMap[m.ID] = m
	}

	type SearchResult struct {
		Reading    Reading `json:"reading"`
		MeterName  string  `json:"meter_name"`
		Department string  `json:"department"`
		SiteName   string  `json:"site_name"`
		M3         float64 `json:"m3"`
	}

	var fromT, toT time.Time
	if dateFrom != "" {
		fromT = parseDate(dateFrom)
	}
	if dateTo != "" {
		toT = parseDate(dateTo)
	} else {
		toT = time.Now().Add(24 * time.Hour)
	}

	siteNames := map[string]string{}
	for _, s := range sites {
		siteNames[s.ID] = s.Name
	}

	results := []SearchResult{}
	for _, rd := range db.Readings {
		if siteID != "" && rd.SiteID != siteID {
			continue
		}
		if !fromT.IsZero() && rd.Date.Before(fromT) {
			continue
		}
		if rd.Date.After(toT) {
			continue
		}
		m := meterMap[rd.MeterID]
		if wt != "" && string(m.WaterType) != wt {
			continue
		}
		if q != "" {
			haystack := strings.ToLower(m.Name + " " + m.Department + " " + rd.SiteID + " " + rd.Notes)
			if !strings.Contains(haystack, q) {
				continue
			}
		}
		results = append(results, SearchResult{
			Reading:    rd,
			MeterName:  m.Name,
			Department: m.Department,
			SiteName:   siteNames[rd.SiteID],
			M3:         math.Round(toM3(rd.Usage, m.Unit)*100) / 100,
		})
	}
	respond(w, 200, results)
}

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

func handleReport(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" {
		from = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02")
	}

	req, _ := http.NewRequest("GET", "/api/dashboard?from="+from+"&to="+to, nil)
	// Re-use dashboard logic
	var dds []DashboardData
	rr := &responseRecorder{}
	handleDashboard(rr, req)
	json.Unmarshal(rr.body, &dds)

	rep := Report{
		Title:     "Water Usage Report",
		Period:    from + " to " + to,
		Generated: time.Now(),
	}
	for _, d := range dds {
		rep.Sites = append(rep.Sites, SiteReport{
			SiteName:    d.SiteName,
			TotalM3:     d.TotalM3,
			TownsM3:     d.TownsM3,
			RiverM3:     d.RiverM3,
			TotalTonnes: d.TotalTonnes,
			M3PerTonne:  d.M3PerTonne,
			Departments: d.Departments,
		})
	}
	respond(w, 200, rep)
}

type responseRecorder struct {
	body []byte
}

func (rr *responseRecorder) Header() http.Header { return http.Header{} }
func (rr *responseRecorder) Write(b []byte) (int, error) {
	rr.body = append(rr.body, b...)
	return len(b), nil
}
func (rr *responseRecorder) WriteHeader(int) {}

func handleImportSeed(w http.ResponseWriter, r *http.Request) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.Readings = []Reading{}
	// Shift seed dates to be relative to now - 26 weeks back
	baseDate := time.Now().AddDate(0, -6, 0)

	type seedRow struct {
		meterID string
		date    string
		value   float64
	}

	// Sample readings from the xlsx data (Rotherham - week readings)
	// Weekly rows: each row is one week's cumulative reading
	rotherhamRows := [][19]float64{
		{122732, 4154, 6410, 56339, 612372, 404781, 886932, 103675, 9276930, 430129, 237717, 58170, 47144, 50420, 53362, 90087, 2158, 81, 349253},
		{122748, 4155, 6423, 56339, 612372, 404781, 887060, 103694, 9279635, 432449, 242580, 63034, 47145, 50423, 53362, 90087, 2163, 82, 349253},
		{122765, 4155, 6444, 56339, 612372, 404781, 887208, 103715, 9282337, 434384, 247431, 67859, 47145, 50426, 53362, 90087, 2168, 82, 349253},
		{122778, 4157, 6456, 56339, 612372, 404781, 887332, 103733, 9285069, 435659, 251680, 72091, 47145, 50430, 53362, 90088, 2174, 82, 349253},
		{122789, 4159, 6467, 56339, 612372, 404781, 887455, 103749, 9287612, 435891, 254847, 75246, 47145, 50434, 53362, 90088, 2179, 82, 349253},
		{122800, 4160, 6479, 56339, 612372, 404781, 887576, 103765, 9290313, 436051, 257977, 78355, 47145, 50437, 53362, 90088, 2183, 82, 349253},
		{122814, 4160, 6491, 56340, 612372, 404781, 887576, 103784, 9292913, 436204, 260953, 81309, 47144, 50440, 53362, 90088, 2191, 82, 349253},
		{122829, 4161, 6503, 56340, 612372, 404781, 887576, 103796, 9295551, 436501, 264135, 84472, 47145, 50444, 53362, 90088, 2194, 88, 349253},
		{122839, 4161, 6537, 56340, 612372, 404781, 887576, 103801, 9302665, 437121, 272580, 92865, 47145, 50451, 53362, 90086, 2201, 88, 349253},
		{122847, 4162, 6542, 56340, 612372, 404781, 887576, 103813, 9303396, 437245, 273504, 93793, 47145, 50452, 53362, 90088, 2203, 88, 349253},
		{123314, 4166, 6549, 56340, 612372, 404781, 887576, 103834, 9306427, 437991, 277527, 97795, 47145, 50518, 53362, 91568, 2203, 84, 349253},
		{123337, 4168, 6549, 56340, 612372, 404781, 887576, 103861, 9309562, 438272, 281268, 101502, 47168, 50523, 53362, 91656, 2214, 84, 350654},
		{123351, 4170, 6549, 56340, 612372, 404781, 887576, 103877, 9311927, 438562, 284033, 104250, 47168, 50526, 53362, 91657, 2225, 84, 350654},
		{123372, 4171, 6549, 56340, 612372, 404781, 887576, 103899, 9314663, 438644, 287293, 107490, 47168, 50530, 53362, 91659, 2225, 84, 350654},
		{123397, 4171, 6549, 56340, 612372, 404781, 887576, 103921, 9317478, 439399, 291101, 111275, 47168, 50534, 53362, 91662, 2234, 84, 350654},
		{123421, 4172, 6549, 56341, 612373, 404781, 887576, 103948, 9320648, 439692, 294906, 115055, 47168, 50539, 53362, 91664, 2235, 85, 350828},
		{123440, 4172, 6549, 56341, 612373, 404781, 887576, 103967, 9323057, 440037, 297889, 118020, 47168, 50542, 53362, 91669, 2241, 85, 350828},
		{123460, 4172, 6549, 56341, 612373, 404781, 887576, 103989, 9325874, 440922, 301864, 121974, 47168, 50546, 53362, 91776, 2245, 85, 350828},
		{123490, 4174, 6549, 56341, 612373, 404781, 887576, 104007, 9328657, 441622, 305602, 125692, 47168, 50550, 53362, 91780, 2249, 86, 350829},
		{123523, 4176, 6549, 56340, 612373, 404781, 887576, 104028, 9331469, 442237, 309293, 129359, 47168, 50555, 53362, 91782, 2254, 86, 350829},
		{123547, 4176, 6549, 56340, 612373, 404781, 887576, 104047, 9334187, 442589, 312612, 132657, 47168, 50558, 53362, 91784, 2260, 86, 350829},
		{123574, 4176, 6549, 56341, 612373, 404781, 887576, 104064, 9337073, 442887, 316063, 136086, 47168, 50566, 53362, 91802, 2263, 86, 350829},
		{123600, 4176, 6549, 56341, 612373, 404781, 887576, 104080, 9339928, 443200, 319481, 139483, 47168, 50571, 53362, 91804, 2268, 86, 350829},
	}
	// Build structs using relative dates from baseDate
	type rotRow struct {
		date                                                                                                  string
		bbr, bikes, aocc, ks, vd, abcrw, abctw, ams, tbmald, tbm, tgh, tmg, tbmr, boc, cgl, cgr, th, jw, rwtp float64
	}
	rotherhamData := make([]rotRow, len(rotherhamRows))
	for i, row := range rotherhamRows {
		dt := baseDate.AddDate(0, 0, i*7)
		rotherhamData[i] = rotRow{dt.Format("2006-01-02"), row[0], row[1], row[2], row[3], row[4], row[5], row[6], row[7], row[8], row[9], row[10], row[11], row[12], row[13], row[14], row[15], row[16], row[17], row[18]}
	}

	makeID := func() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }
	time.Sleep(time.Nanosecond)

	for _, row := range rotherhamData {
		dt := parseDate(row.date)
		pairs := [][2]interface{}{
			{"r-bbr", row.bbr}, {"r-bs", row.bikes}, {"r-aocc", row.aocc},
			{"r-ks", row.ks}, {"r-vd", row.vd}, {"r-abcrw", row.abcrw},
			{"r-abctw", row.abctw}, {"r-ams", row.ams}, {"r-tbmr", row.tbmr},
			{"r-tbm", row.tbm}, {"r-tgh", row.tgh}, {"r-tmg", row.tmg},
			{"r-tbmr", row.tbmr}, {"r-boc", row.boc}, {"r-cgl", row.cgl},
			{"r-cgr", row.cgr}, {"r-th", row.th}, {"r-jw", row.jw}, {"r-rwtp", row.rwtp},
		}
		for _, p := range pairs {
			mid := p[0].(string)
			val := p[1].(float64)
			var m Meter
			for _, mm := range db.Meters {
				if mm.ID == mid {
					m = mm
					break
				}
			}
			db.Readings = append(db.Readings, Reading{
				ID:        makeID(),
				MeterID:   mid,
				SiteID:    "rotherham",
				Value:     val,
				Date:      dt,
				Source:    "manual",
				WaterType: m.WaterType,
			})
			time.Sleep(time.Nanosecond)
		}
	}

	// After all readings added, compute usage deltas for all meters
	sbBase := time.Now().AddDate(0, -4, 0)
	sbData := []struct {
		meterID string
		offset  int
		value   float64
	}{
		{"s-rms1", 0, 2353132}, {"s-rms2", 0, 1286108}, {"s-vim", 0, 2447},
		{"s-wq1", 0, 15444850}, {"s-wq2", 0, 394740}, {"s-sb", 0, 100},
		{"s-eb", 0, 2018210}, {"s-croft", 0, 87830808},
		{"s-rms1", 90, 2410000}, {"s-rms2", 90, 1320000}, {"s-vim", 90, 2650},
		{"s-wq1", 90, 15980000}, {"s-wq2", 90, 410000}, {"s-sb", 90, 105},
		{"s-eb", 90, 2150000}, {"s-croft", 90, 88500000},
	}
	for _, sd := range sbData {
		var m Meter
		for _, mm := range db.Meters {
			if mm.ID == sd.meterID {
				m = mm
				break
			}
		}
		db.Readings = append(db.Readings, Reading{
			ID: makeID(), MeterID: sd.meterID, SiteID: "stocksbridge",
			Value: sd.value, Date: sbBase.AddDate(0, 0, sd.offset), Source: "eemon", WaterType: m.WaterType,
		})
		time.Sleep(time.Nanosecond)
	}

	// Compute deltas for all meters
	byMeter := map[string][]Reading{}
	for i, rd := range db.Readings {
		byMeter[rd.MeterID] = append(byMeter[rd.MeterID], db.Readings[i])
	}
	db.Readings = []Reading{}
	for _, rds := range byMeter {
		sort.Slice(rds, func(i, j int) bool { return rds[i].Date.Before(rds[j].Date) })
		for i := range rds {
			if i > 0 && rds[i].Value > rds[i-1].Value {
				rds[i].Usage = rds[i].Value - rds[i-1].Value
			}
			db.Readings = append(db.Readings, rds[i])
		}
	}

	// Seed tonnes - use relative dates
	m1ago := time.Now().AddDate(0, -1, 0)
	m2ago := time.Now().AddDate(0, -2, 0)
	m3ago := time.Now().AddDate(0, -3, 0)
	db.Tonnes = []TonnesEntry{
		{ID: makeID(), SiteID: "rotherham", Department: "ACP", Tonnes: 8500, Date: m3ago},
		{ID: makeID(), SiteID: "rotherham", Department: "BBR", Tonnes: 3200, Date: m3ago},
		{ID: makeID(), SiteID: "rotherham", Department: "TCM", Tonnes: 12000, Date: m3ago},
		{ID: makeID(), SiteID: "rotherham", Department: "ACP", Tonnes: 9100, Date: m2ago},
		{ID: makeID(), SiteID: "rotherham", Department: "BBR", Tonnes: 3400, Date: m2ago},
		{ID: makeID(), SiteID: "rotherham", Department: "TCM", Tonnes: 11800, Date: m2ago},
		{ID: makeID(), SiteID: "rotherham", Department: "ACP", Tonnes: 9300, Date: m1ago},
		{ID: makeID(), SiteID: "rotherham", Department: "BBR", Tonnes: 3500, Date: m1ago},
		{ID: makeID(), SiteID: "rotherham", Department: "TCM", Tonnes: 12300, Date: m1ago},
		{ID: makeID(), SiteID: "stocksbridge", Department: "Remelt", Tonnes: 15000, Date: m3ago},
		{ID: makeID(), SiteID: "stocksbridge", Department: "Billet Mill", Tonnes: 22000, Date: m3ago},
		{ID: makeID(), SiteID: "stocksbridge", Department: "Remelt", Tonnes: 14500, Date: m2ago},
		{ID: makeID(), SiteID: "stocksbridge", Department: "Billet Mill", Tonnes: 21000, Date: m2ago},
		{ID: makeID(), SiteID: "brinsworth", Department: "Hot Mill", Tonnes: 18000, Date: m2ago},
		{ID: makeID(), SiteID: "brinsworth", Department: "Cold Mill", Tonnes: 12000, Date: m2ago},
	}

	db.save()
	respond(w, 200, map[string]string{"status": "seeded", "readings": strconv.Itoa(len(db.Readings))})
}

func handleEEmonSync(w http.ResponseWriter, r *http.Request) {
	// Stub for EEmon API integration
	// In production: call EEmon REST API with credentials from config
	cfg := struct {
		BaseURL  string `json:"base_url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}{}
	if r.Method == http.MethodPost {
		json.NewDecoder(r.Body).Decode(&cfg)
	}
	respond(w, 200, map[string]interface{}{
		"status":  "configured",
		"message": "EEmon sync endpoint ready. POST {base_url, username, password} to configure live connection.",
		"meters":  []string{"s-rms1", "s-rms2", "s-vim", "s-bm16", "s-bm17", "s-bms14", "s-croft", "s-wq1", "s-wq2", "s-sb"},
	})
}

func handleTrendSync(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, map[string]interface{}{
		"status":  "configured",
		"message": "Trend system sync endpoint ready. POST {base_url, api_key} to configure live connection.",
		"meters":  []string{"b-hmr", "b-hrp", "b-cmr"},
	})
}

func handleKPIs(w http.ResponseWriter, r *http.Request) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	meterMap := map[string]Meter{}
	for _, m := range db.Meters {
		meterMap[m.ID] = m
	}

	type SiteKPI struct {
		SiteID     string  `json:"site_id"`
		SiteName   string  `json:"site_name"`
		TotalM3    float64 `json:"total_m3"`
		TownsM3    float64 `json:"towns_m3"`
		RiverM3    float64 `json:"river_m3"`
		Tonnes     float64 `json:"tonnes"`
		M3PerTonne float64 `json:"m3_per_tonne"`
	}

	from := time.Now().AddDate(0, -6, 0)
	to := time.Now().Add(24 * time.Hour)

	siteM3 := map[string][2]float64{}
	for _, rd := range db.Readings {
		if rd.Date.Before(from) || rd.Date.After(to) {
			continue
		}
		m := meterMap[rd.MeterID]
		u := toM3(rd.Usage, m.Unit)
		if u <= 0 {
			continue
		}
		arr := siteM3[rd.SiteID]
		switch rd.WaterType {
		case TownsWater:
			arr[0] += u
		case RiverWater:
			arr[1] += u
		case Both:
			arr[0] += u / 2
			arr[1] += u / 2
		}
		siteM3[rd.SiteID] = arr
	}

	siteTonnes := map[string]float64{}
	for _, t := range db.Tonnes {
		if t.Date.Before(from) || t.Date.After(to) {
			continue
		}
		siteTonnes[t.SiteID] += t.Tonnes
	}

	siteNames := map[string]string{}
	for _, s := range sites {
		siteNames[s.ID] = s.Name
	}

	kpis := []SiteKPI{}
	for _, s := range sites {
		arr := siteM3[s.ID]
		total := arr[0] + arr[1]
		tn := siteTonnes[s.ID]
		var m3t float64
		if tn > 0 {
			m3t = total / tn
		}
		kpis = append(kpis, SiteKPI{
			SiteID:     s.ID,
			SiteName:   s.Name,
			TotalM3:    math.Round(total*100) / 100,
			TownsM3:    math.Round(arr[0]*100) / 100,
			RiverM3:    math.Round(arr[1]*100) / 100,
			Tonnes:     math.Round(tn*100) / 100,
			M3PerTonne: math.Round(m3t*1000) / 1000,
		})
	}
	respond(w, 200, kpis)
}

// ─── CORS & Router ────────────────────────────────────────────────────────────

func withCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		h(w, r)
	}
}

func main() {
	dataDir := "./data"
	os.MkdirAll(dataDir, 0755)
	db = loadDB(filepath.Join(dataDir, "water.json"))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/sites", withCORS(handleSites))
	mux.HandleFunc("/api/meters", withCORS(handleMeters))
	mux.HandleFunc("/api/readings", withCORS(handleReadings))
	mux.HandleFunc("/api/tonnes", withCORS(handleTonnes))
	mux.HandleFunc("/api/dashboard", withCORS(handleDashboard))
	mux.HandleFunc("/api/search", withCORS(handleSearch))
	mux.HandleFunc("/api/report", withCORS(handleReport))
	mux.HandleFunc("/api/kpis", withCORS(handleKPIs))
	mux.HandleFunc("/api/seed", withCORS(handleImportSeed))
	mux.HandleFunc("/api/eemon/sync", withCORS(handleEEmonSync))
	mux.HandleFunc("/api/trend/sync", withCORS(handleTrendSync))

	// Serve frontend
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./frontend/index.html")
	})

	log.Println("🌊 Speciality Steels Water Management System")
	log.Println("   Server: http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
