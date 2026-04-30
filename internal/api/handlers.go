package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"water-monitoring-system/internal/database"
	"water-monitoring-system/internal/models"
)

// Handler bundles all HTTP handler methods
type Handler struct {
	db *database.DB
}

// NewHandler creates a new Handler with the given database
func NewHandler(db *database.DB) *Handler {
	return &Handler{db: db}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (h *Handler) respond(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
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

func toM3(v float64, unit string) float64 {
	if unit == "litres" {
		return v / 1000.0
	}
	return v
}

// ─── Sites ────────────────────────────────────────────────────────────────────

func (h *Handler) HandleSites(w http.ResponseWriter, r *http.Request) {
	h.respond(w, 200, h.db.GetSites())
}

// ─── Meters ───────────────────────────────────────────────────────────────────

func (h *Handler) HandleMeters(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	h.respond(w, 200, h.db.GetMeters(siteID))
}

// ─── Readings ─────────────────────────────────────────────────────────────────

func (h *Handler) HandleReadings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var body struct {
			MeterID string  `json:"meter_id"`
			Value   float64 `json:"value"`
			Date    string  `json:"date"`
			Notes   string  `json:"notes"`
		}
		json.NewDecoder(r.Body).Decode(&body)

		meter := h.db.GetMeter(body.MeterID)
		if meter == nil {
			h.respond(w, 400, map[string]string{"error": "meter not found"})
			return
		}

		reading := models.Reading{
			MeterID:   body.MeterID,
			SiteID:    meter.SiteID,
			Value:     body.Value,
			Date:      parseDate(body.Date),
			Source:    models.SourceManual,
			Notes:     body.Notes,
			WaterType: meter.WaterType,
		}
		if err := h.db.AddReading(reading); err != nil {
			h.respond(w, 500, map[string]string{"error": err.Error()})
			return
		}
		h.respond(w, 201, map[string]string{"status": "ok"})
		return
	}

	// GET
	q := r.URL.Query()
	siteID := q.Get("site_id")
	meterID := q.Get("meter_id")
	from := q.Get("from")
	to := q.Get("to")

	var fromT, toT time.Time
	if from != "" {
		fromT = parseDate(from)
	}
	if to != "" {
		toT = parseDate(to)
	} else {
		toT = time.Now().Add(24 * time.Hour)
	}

	h.respond(w, 200, h.db.GetReadings(siteID, meterID, fromT, toT))
}

// ─── Tonnes ───────────────────────────────────────────────────────────────────

func (h *Handler) HandleTonnes(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var body models.TonnesEntry
		json.NewDecoder(r.Body).Decode(&body)
		if body.Date.IsZero() {
			body.Date = time.Now()
		}
		if err := h.db.AddTonnes(body); err != nil {
			h.respond(w, 500, map[string]string{"error": err.Error()})
			return
		}
		h.respond(w, 201, map[string]string{"status": "ok"})
		return
	}

	// GET
	q := r.URL.Query()
	siteID := q.Get("site_id")
	from := q.Get("from")
	to := q.Get("to")

	var fromT, toT time.Time
	if from != "" {
		fromT = parseDate(from)
	}
	if to != "" {
		toT = parseDate(to)
	} else {
		toT = time.Now().Add(24 * time.Hour)
	}

	h.respond(w, 200, h.db.GetTonnes(siteID, fromT, toT))
}

// ─── Dashboard ────────────────────────────────────────────────────────────────

func (h *Handler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	siteID := r.URL.Query().Get("site_id")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	var fromT time.Time
	toT := time.Now().Add(24 * time.Hour)
	if from != "" {
		fromT = parseDate(from)
	} else {
		fromT = time.Now().AddDate(0, -6, 0) // Match KPIs: 6 months
	}
	if to != "" {
		toT = parseDate(to)
	}

	meterMap := h.db.GetMeterMap()
	sites := h.db.GetSites()
	siteNames := h.db.GetSiteNames()

	type deptKey struct{ site, dept string }
	deptM3 := map[deptKey]float64{}
	deptTowns := map[deptKey]float64{}
	deptRiver := map[deptKey]float64{}

	type weekKey struct{ site, week string }
	weekTowns := map[weekKey]float64{}
	weekRiver := map[weekKey]float64{}

	siteTotals := map[string]struct{ towns, river float64 }{}

	readings := h.db.GetReadings(siteID, "", fromT, toT)
	for _, rd := range readings {
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
		case models.TownsWater:
			deptTowns[dk] += usageM3
			weekTowns[wk] += usageM3
			st.towns += usageM3
		case models.RiverWater:
			deptRiver[dk] += usageM3
			weekRiver[wk] += usageM3
			st.river += usageM3
		case models.BothWater:
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
	tonnes := h.db.GetTonnes(siteID, fromT, toT)
	for _, t := range tonnes {
		dk := deptKey{t.SiteID, t.Department}
		deptTonnes[dk] += t.Tonnes
		siteTonnesTotal[t.SiteID] += t.Tonnes
	}

	buildSite := func(sid, sname string) models.DashboardData {
		d := models.DashboardData{SiteID: sid, SiteName: sname}
		st := siteTotals[sid]
		d.TownsM3 = math.Round(st.towns*100) / 100
		d.RiverM3 = math.Round(st.river*100) / 100
		d.TotalM3 = math.Round((st.towns+st.river)*100) / 100
		d.TotalTonnes = math.Round(siteTonnesTotal[sid]*100) / 100
		if d.TotalTonnes > 0 {
			d.M3PerTonne = math.Round(d.TotalM3/d.TotalTonnes*1000) / 1000
		}

		// Departments
		meters := h.db.GetMeters(sid)
		seen := map[string]bool{}
		for _, m := range meters {
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
			d.Departments = append(d.Departments, models.DeptSummary{
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
			d.Timeline = append(d.Timeline, models.TimePoint{
				Date:  wk,
				Towns: math.Round(v[0]*100) / 100,
				River: math.Round(v[1]*100) / 100,
				Total: math.Round((v[0]+v[1])*100) / 100,
			})
		}

		d.WaterTypeSplit = []models.PieSlice{
			{Label: "Towns Water", Value: d.TownsM3},
			{Label: "River Water", Value: d.RiverM3},
		}
		return d
	}

	if siteID != "" {
		if name, ok := siteNames[siteID]; ok {
			h.respond(w, 200, buildSite(siteID, name))
			return
		}
		h.respond(w, 404, map[string]string{"error": "site not found"})
		return
	}

	result := []models.DashboardData{}
	for _, s := range sites {
		result = append(result, buildSite(s.ID, s.Name))
	}
	h.respond(w, 200, result)
}

// ─── KPIs ─────────────────────────────────────────────────────────────────────

func (h *Handler) HandleKPIs(w http.ResponseWriter, r *http.Request) {
	meterMap := h.db.GetMeterMap()
	sites := h.db.GetSites()

	// Accept date filter parameters like dashboard
	fromParam := r.URL.Query().Get("from")
	toParam := r.URL.Query().Get("to")

	var from time.Time
	to := time.Now().Add(24 * time.Hour)
	if fromParam != "" {
		from = parseDate(fromParam)
	} else {
		from = time.Now().AddDate(0, -6, 0) // Default 6 months
	}
	if toParam != "" {
		to = parseDate(toParam)
	}

	siteM3 := map[string][2]float64{}
	readings := h.db.GetReadings("", "", from, to)
	for _, rd := range readings {
		m := meterMap[rd.MeterID]
		u := toM3(rd.Usage, m.Unit)
		if u <= 0 {
			continue
		}
		arr := siteM3[rd.SiteID]
		switch rd.WaterType {
		case models.TownsWater:
			arr[0] += u
		case models.RiverWater:
			arr[1] += u
		case models.BothWater:
			arr[0] += u / 2
			arr[1] += u / 2
		}
		siteM3[rd.SiteID] = arr
	}

	siteTonnes := map[string]float64{}
	tonnes := h.db.GetTonnes("", from, to)
	for _, t := range tonnes {
		siteTonnes[t.SiteID] += t.Tonnes
	}

	kpis := []models.SiteKPI{}
	for _, s := range sites {
		arr := siteM3[s.ID]
		total := arr[0] + arr[1]
		tn := siteTonnes[s.ID]
		var m3t float64
		if tn > 0 {
			m3t = total / tn
		}
		kpis = append(kpis, models.SiteKPI{
			SiteID:     s.ID,
			SiteName:   s.Name,
			TotalM3:    math.Round(total*100) / 100,
			TownsM3:    math.Round(arr[0]*100) / 100,
			RiverM3:    math.Round(arr[1]*100) / 100,
			Tonnes:     math.Round(tn*100) / 100,
			M3PerTonne: math.Round(m3t*1000) / 1000,
		})
	}
	h.respond(w, 200, kpis)
}

// ─── Search ───────────────────────────────────────────────────────────────────

func (h *Handler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(r.URL.Query().Get("q"))
	siteID := r.URL.Query().Get("site_id")
	dateFrom := r.URL.Query().Get("from")
	dateTo := r.URL.Query().Get("to")
	wt := r.URL.Query().Get("water_type")

	meterMap := h.db.GetMeterMap()
	siteNames := h.db.GetSiteNames()

	var fromT, toT time.Time
	if dateFrom != "" {
		fromT = parseDate(dateFrom)
	}
	if dateTo != "" {
		toT = parseDate(dateTo)
	} else {
		toT = time.Now().Add(24 * time.Hour)
	}

	readings := h.db.GetReadings(siteID, "", fromT, toT)
	results := []models.SearchResult{}
	for _, rd := range readings {
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
		results = append(results, models.SearchResult{
			Reading:    rd,
			MeterName:  m.Name,
			Department: m.Department,
			SiteName:   siteNames[rd.SiteID],
			M3:         math.Round(toM3(rd.Usage, m.Unit)*100) / 100,
		})
	}
	h.respond(w, 200, results)
}

// ─── Report ───────────────────────────────────────────────────────────────────

func (h *Handler) HandleReport(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" {
		from = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02")
	}

	// Build dashboard data for report
	req, _ := http.NewRequest("GET", "/api/dashboard?from="+from+"&to="+to, nil)
	rr := &responseRecorder{}
	h.HandleDashboard(rr, req)

	var dds []models.DashboardData
	json.Unmarshal(rr.body, &dds)

	rep := models.Report{
		Title:     "Water Usage Report",
		Period:    from + " to " + to,
		Generated: time.Now(),
	}
	for _, d := range dds {
		rep.Sites = append(rep.Sites, models.SiteReport{
			SiteName:    d.SiteName,
			TotalM3:     d.TotalM3,
			TownsM3:     d.TownsM3,
			RiverM3:     d.RiverM3,
			TotalTonnes: d.TotalTonnes,
			M3PerTonne:  d.M3PerTonne,
			Departments: d.Departments,
		})
	}
	h.respond(w, 200, rep)
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

// ─── Seed Data ────────────────────────────────────────────────────────────────

func (h *Handler) HandleSeed(w http.ResponseWriter, r *http.Request) {
	count, err := h.db.SeedSampleData()
	if err != nil {
		h.respond(w, 500, map[string]string{"error": err.Error()})
		return
	}
	h.respond(w, 200, map[string]interface{}{
		"status":   "seeded",
		"readings": count,
	})
}

// ─── Integration Stubs ────────────────────────────────────────────────────────

func (h *Handler) HandleEEmonSync(w http.ResponseWriter, r *http.Request) {
	h.respond(w, 200, map[string]interface{}{
		"status":  "configured",
		"message": "EEmon sync endpoint ready. POST {base_url, username, password} to configure live connection.",
		"meters":  []string{"s-rms1", "s-rms2", "s-vim", "s-bm16", "s-bm17", "s-bms14", "s-croft", "s-wq1", "s-wq2", "s-sb"},
	})
}

func (h *Handler) HandleTrendSync(w http.ResponseWriter, r *http.Request) {
	h.respond(w, 200, map[string]interface{}{
		"status":  "configured",
		"message": "Trend system sync endpoint ready. POST {base_url, api_key} to configure live connection.",
		"meters":  []string{"b-hmr", "b-hrp", "b-cmr"},
	})
}

// ─── Admin Handlers ───────────────────────────────────────────────────────────

func (h *Handler) HandlePreferences(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.respond(w, 200, h.db.GetPreferences())
		return
	}

	if r.Method == http.MethodPut || r.Method == http.MethodPost {
		var prefs models.UserPreferences
		json.NewDecoder(r.Body).Decode(&prefs)
		if err := h.db.UpdatePreferences(prefs); err != nil {
			h.respond(w, 500, map[string]string{"error": err.Error()})
			return
		}
		h.respond(w, 200, map[string]string{"status": "saved"})
		return
	}
	h.respond(w, 405, map[string]string{"error": "method not allowed"})
}

func (h *Handler) HandleAutoFill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respond(w, 405, map[string]string{"error": "method not allowed"})
		return
	}

	var body struct {
		MeterID    string `json:"meter_id"`
		TargetDate string `json:"target_date"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	targetDate := time.Now()
	if body.TargetDate != "" {
		targetDate = parseDate(body.TargetDate)
	}

	prefs := h.db.GetPreferences()
	if !prefs.AutoFillEnabled {
		h.respond(w, 400, map[string]string{"error": "Auto-fill is disabled in preferences"})
		return
	}

	est := h.db.AutoFillMissingData(body.MeterID, targetDate)
	if est == nil {
		h.respond(w, 404, map[string]string{"error": "Unable to generate estimate - insufficient historical data"})
		return
	}

	if err := h.db.AddReading(*est); err != nil {
		h.respond(w, 500, map[string]string{"error": err.Error()})
		return
	}
	h.respond(w, 201, map[string]interface{}{
		"status":    "created",
		"estimated": true,
		"reading":   est,
	})
}

func (h *Handler) HandleAutoFillAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respond(w, 405, map[string]string{"error": "method not allowed"})
		return
	}

	prefs := h.db.GetPreferences()
	if !prefs.AutoFillEnabled {
		h.respond(w, 400, map[string]string{"error": "Auto-fill is disabled in preferences"})
		return
	}

	var body struct {
		SiteID     string `json:"site_id"`
		TargetDate string `json:"target_date"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	targetDate := time.Now()
	if body.TargetDate != "" {
		targetDate = parseDate(body.TargetDate)
	}

	lastReadings := h.db.GetLastReadingTimes()
	sevenDaysAgo := targetDate.AddDate(0, 0, -7)

	meters := h.db.GetMeters(body.SiteID)
	var filled int
	for _, m := range meters {
		if !m.IsActive {
			continue
		}
		if t, ok := lastReadings[m.ID]; ok && t.After(sevenDaysAgo) {
			continue
		}
		est := h.db.AutoFillMissingData(m.ID, targetDate)
		if est != nil {
			h.db.AddReading(*est)
			filled++
		}
	}

	h.respond(w, 200, map[string]interface{}{
		"status":  "completed",
		"filled":  filled,
		"message": fmt.Sprintf("Auto-filled %d meter readings", filled),
	})
}

func (h *Handler) HandleConnectionStatus(w http.ResponseWriter, r *http.Request) {
	h.respond(w, 200, h.db.GetConnectionStatus())
}

func (h *Handler) HandleClearData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		h.respond(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	if err := h.db.ClearData(); err != nil {
		h.respond(w, 500, map[string]string{"error": err.Error()})
		return
	}
	h.respond(w, 200, map[string]string{"status": "cleared", "message": "All readings and tonnes data cleared"})
}
