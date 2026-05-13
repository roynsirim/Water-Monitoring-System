package database

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"water-monitoring-system/internal/models"
)

// DB wraps the JSON file database with thread-safe operations
type DB struct {
	mu          sync.RWMutex
	path        string
	Meters      []models.Meter          `json:"meters"`
	Readings    []models.Reading        `json:"readings"`
	Tonnes      []models.TonnesEntry    `json:"tonnes"`
	Preferences models.UserPreferences  `json:"preferences"`
	ConnStatus  models.ConnectionStatus `json:"connection_status"`
}

// Open loads the database from the specified JSON file path
func Open(path string) (*DB, error) {
	db := &DB{path: path}

	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, db); err != nil {
			return nil, fmt.Errorf("parsing database: %w", err)
		}
	}

	// Initialize with defaults if empty
	if len(db.Meters) == 0 {
		db.Meters = SeedMeters()
		// Seed sample data when creating fresh database
		seedSampleReadings(db)
	}
	if db.Preferences.Theme == "" {
		db.Preferences = models.DefaultPreferences()
	}
	if db.Readings == nil {
		db.Readings = []models.Reading{}
	}
	if db.Tonnes == nil {
		db.Tonnes = []models.TonnesEntry{}
	}

	return db, nil
}

// Save persists the database to disk
func (db *DB) Save() error {
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling database: %w", err)
	}
	return os.WriteFile(db.path, data, 0644)
}

// ─── Sites ────────────────────────────────────────────────────────────────────

// GetSites returns all configured sites
func (db *DB) GetSites() []models.Site {
	return models.DefaultSites()
}

// GetSiteNames returns a map of site ID to name
func (db *DB) GetSiteNames() map[string]string {
	names := make(map[string]string)
	for _, s := range models.DefaultSites() {
		names[s.ID] = s.Name
	}
	return names
}

// ─── Meters ───────────────────────────────────────────────────────────────────

// GetMeters returns all meters, optionally filtered by site
func (db *DB) GetMeters(siteID string) []models.Meter {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var result []models.Meter
	if siteID == "" {
		result = append(result, db.Meters...)
	} else {
		for _, m := range db.Meters {
			if m.SiteID == siteID {
				result = append(result, m)
			}
		}
	}

	// Sort by name
	for i := 0; i < len(result)-1; i++ {
		for j := 0; j < len(result)-i-1; j++ {
			if result[j].Name > result[j+1].Name {
				result[j], result[j+1] = result[j+1], result[j]
			}
		}
	}
	return result
}

// GetMeter returns a meter by ID
func (db *DB) GetMeter(id string) *models.Meter {
	db.mu.RLock()
	defer db.mu.RUnlock()

	for i := range db.Meters {
		if db.Meters[i].ID == id {
			return &db.Meters[i]
		}
	}
	return nil
}

// GetMeterMap returns a map of meter ID to meter
func (db *DB) GetMeterMap() map[string]models.Meter {
	db.mu.RLock()
	defer db.mu.RUnlock()

	m := make(map[string]models.Meter)
	for _, meter := range db.Meters {
		m[meter.ID] = meter
	}
	return m
}

// ─── Readings ─────────────────────────────────────────────────────────────────

// AddReading adds a new meter reading
func (db *DB) AddReading(r models.Reading) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	r.ID = fmt.Sprintf("%d", time.Now().UnixNano())

	// Calculate usage from last reading
	var last float64
	for i := len(db.Readings) - 1; i >= 0; i-- {
		if db.Readings[i].MeterID == r.MeterID {
			last = db.Readings[i].Value
			break
		}
	}
	if last > 0 && r.Value > last {
		r.Usage = r.Value - last
	}

	db.Readings = append(db.Readings, r)
	return db.Save()
}

// GetReadings returns readings filtered by criteria
func (db *DB) GetReadings(siteID, meterID string, from, to time.Time) []models.Reading {
	db.mu.RLock()
	defer db.mu.RUnlock()

	result := []models.Reading{}
	for _, rd := range db.Readings {
		if siteID != "" && rd.SiteID != siteID {
			continue
		}
		if meterID != "" && rd.MeterID != meterID {
			continue
		}
		if !from.IsZero() && rd.Date.Before(from) {
			continue
		}
		if !to.IsZero() && rd.Date.After(to) {
			continue
		}
		result = append(result, rd)
	}
	return result
}

// GetLastReadingTime returns the last reading time for each meter
func (db *DB) GetLastReadingTimes() map[string]time.Time {
	db.mu.RLock()
	defer db.mu.RUnlock()

	last := make(map[string]time.Time)
	for _, rd := range db.Readings {
		if t, ok := last[rd.MeterID]; !ok || rd.Date.After(t) {
			last[rd.MeterID] = rd.Date
		}
	}
	return last
}

// ─── Tonnes ───────────────────────────────────────────────────────────────────

// AddTonnes adds a new tonnes entry
func (db *DB) AddTonnes(t models.TonnesEntry) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	t.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	db.Tonnes = append(db.Tonnes, t)
	return db.Save()
}

// GetTonnes returns tonnes filtered by criteria
func (db *DB) GetTonnes(siteID string, from, to time.Time) []models.TonnesEntry {
	db.mu.RLock()
	defer db.mu.RUnlock()

	result := []models.TonnesEntry{}
	for _, t := range db.Tonnes {
		if siteID != "" && t.SiteID != siteID {
			continue
		}
		if !from.IsZero() && t.Date.Before(from) {
			continue
		}
		if !to.IsZero() && t.Date.After(to) {
			continue
		}
		result = append(result, t)
	}
	return result
}

// ─── Auto-Fill ────────────────────────────────────────────────────────────────

// AutoFillMissingData estimates a reading based on historical averages
func (db *DB) AutoFillMissingData(meterID string, targetDate time.Time) *models.Reading {
	db.mu.RLock()
	defer db.mu.RUnlock()

	// Get readings for this meter in last 30 days
	thirtyDaysAgo := targetDate.AddDate(0, 0, -30)
	var totalUsage float64
	var count int
	var lastReading *models.Reading

	for i := len(db.Readings) - 1; i >= 0; i-- {
		rd := db.Readings[i]
		if rd.MeterID == meterID {
			if lastReading == nil {
				lastReading = &db.Readings[i]
			}
			if rd.Date.After(thirtyDaysAgo) && rd.Usage > 0 {
				totalUsage += rd.Usage
				count++
			}
		}
	}

	if count == 0 || lastReading == nil {
		return nil
	}

	avgDailyUsage := totalUsage / float64(count*7) // Convert weekly to daily
	daysSinceLast := int(targetDate.Sub(lastReading.Date).Hours() / 24)
	estimatedUsage := avgDailyUsage * float64(daysSinceLast)

	// Find meter for water type
	var waterType models.WaterType
	var siteID string
	for _, m := range db.Meters {
		if m.ID == meterID {
			waterType = m.WaterType
			siteID = m.SiteID
			break
		}
	}

	return &models.Reading{
		MeterID:     meterID,
		SiteID:      siteID,
		Value:       lastReading.Value + estimatedUsage,
		Usage:       estimatedUsage,
		Date:        targetDate,
		Source:      models.SourceEstimated,
		Notes:       "Auto-filled estimate based on historical average",
		WaterType:   waterType,
		IsEstimated: true,
	}
}

// ─── Preferences ──────────────────────────────────────────────────────────────

// GetPreferences returns user preferences
func (db *DB) GetPreferences() models.UserPreferences {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.Preferences
}

// UpdatePreferences updates user preferences
func (db *DB) UpdatePreferences(prefs models.UserPreferences) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if prefs.Theme != "" {
		db.Preferences.Theme = prefs.Theme
	}
	db.Preferences.AutoFillEnabled = prefs.AutoFillEnabled
	if prefs.DefaultDateRange > 0 {
		db.Preferences.DefaultDateRange = prefs.DefaultDateRange
	}

	return db.Save()
}

// ─── Connection Status ────────────────────────────────────────────────────────

// GetConnectionStatus returns the connection status
func (db *DB) GetConnectionStatus() models.ConnectionStatus {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.ConnStatus
}

// ─── Clear Data ───────────────────────────────────────────────────────────────

// ClearData clears all readings and tonnes data
func (db *DB) ClearData() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.Readings = []models.Reading{}
	db.Tonnes = []models.TonnesEntry{}
	return db.Save()
}

// ─── Seed Data ────────────────────────────────────────────────────────────────

// SeedSampleData populates the database with sample data
func (db *DB) SeedSampleData() (int, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.Readings = []models.Reading{}
	seedSampleReadings(db)

	if err := db.Save(); err != nil {
		return 0, err
	}
	return len(db.Readings), nil
}
