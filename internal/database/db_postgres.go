package database

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"water-monitoring-system/internal/models"

	"github.com/google/uuid"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// PostgresDB wraps a PostgreSQL database connection
type PostgresDB struct {
	conn *sql.DB
	dsn  string
}

// OpenPostgres connects to a PostgreSQL database and initializes tables
func OpenPostgres(dsn string) (*PostgresDB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}

	// Configure connection pool for production use
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging postgres: %w", err)
	}

	pdb := &PostgresDB{conn: db, dsn: dsn}

	// Initialize schema
	if err := pdb.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	return pdb, nil
}

// initSchema creates tables if they don't exist
func (db *PostgresDB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS meters (
		id VARCHAR(64) PRIMARY KEY,
		site_id VARCHAR(64) NOT NULL,
		department VARCHAR(128) NOT NULL,
		name VARCHAR(256) NOT NULL,
		water_type VARCHAR(32) NOT NULL,
		feed VARCHAR(128),
		source VARCHAR(32) NOT NULL,
		unit VARCHAR(32) NOT NULL,
		is_active BOOLEAN NOT NULL DEFAULT true,
		is_main_meter BOOLEAN NOT NULL DEFAULT false,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS readings (
		id VARCHAR(64) PRIMARY KEY,
		meter_id VARCHAR(64) NOT NULL REFERENCES meters(id) ON DELETE CASCADE,
		site_id VARCHAR(64) NOT NULL,
		value DOUBLE PRECISION NOT NULL,
		usage DOUBLE PRECISION NOT NULL DEFAULT 0,
		reading_date TIMESTAMP WITH TIME ZONE NOT NULL,
		source VARCHAR(32) NOT NULL,
		notes TEXT,
		water_type VARCHAR(32) NOT NULL,
		is_estimated BOOLEAN NOT NULL DEFAULT false,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS tonnes (
		id VARCHAR(64) PRIMARY KEY,
		site_id VARCHAR(64) NOT NULL,
		department VARCHAR(128) NOT NULL,
		tonnes DOUBLE PRECISION NOT NULL,
		entry_date TIMESTAMP WITH TIME ZONE NOT NULL,
		notes TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS preferences (
		id INTEGER PRIMARY KEY DEFAULT 1,
		theme VARCHAR(32) NOT NULL DEFAULT 'dark',
		auto_fill_enabled BOOLEAN NOT NULL DEFAULT false,
		default_date_range INTEGER NOT NULL DEFAULT 90,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		CHECK (id = 1)
	);

	-- Insert default preferences if not exists
	INSERT INTO preferences (id, theme, auto_fill_enabled, default_date_range)
	VALUES (1, 'dark', false, 90)
	ON CONFLICT (id) DO NOTHING;

	-- Create indexes for common queries
	CREATE INDEX IF NOT EXISTS idx_readings_meter_id ON readings(meter_id);
	CREATE INDEX IF NOT EXISTS idx_readings_site_id ON readings(site_id);
	CREATE INDEX IF NOT EXISTS idx_readings_date ON readings(reading_date);
	CREATE INDEX IF NOT EXISTS idx_metres_site_id ON meters(site_id);
	CREATE INDEX IF NOT EXISTS idx_tonnes_site_id ON tonnes(site_id);
	CREATE INDEX IF NOT EXISTS idx_tonnes_date ON tonnes(entry_date);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// Close closes the database connection
func (db *PostgresDB) Close() error {
	return db.conn.Close()
}

// Save is a no-op for PostgreSQL (changes are persisted immediately)
func (db *PostgresDB) Save() error {
	return nil
}

// ─── Sites ────────────────────────────────────────────────────────────────────

// GetSites returns all configured sites
func (db *PostgresDB) GetSites() []models.Site {
	return models.DefaultSites()
}

// GetSiteNames returns a map of site ID to name
func (db *PostgresDB) GetSiteNames() map[string]string {
	names := make(map[string]string)
	for _, s := range models.DefaultSites() {
		names[s.ID] = s.Name
	}
	return names
}

// ─── Meters ───────────────────────────────────────────────────────────────────

// GetMeters returns all meters, optionally filtered by site
func (db *PostgresDB) GetMeters(siteID string) []models.Meter {
	var rows *sql.Rows
	var err error

	if siteID == "" {
		rows, err = db.conn.Query(`
			SELECT id, site_id, department, name, water_type, feed, source, unit, is_active, is_main_meter
			FROM meters
			ORDER BY name
		`)
	} else {
		rows, err = db.conn.Query(`
			SELECT id, site_id, department, name, water_type, feed, source, unit, is_active, is_main_meter
			FROM meters
			WHERE site_id = $1
			ORDER BY name
		`, siteID)
	}

	if err != nil {
		return []models.Meter{}
	}
	defer rows.Close()

	meters := []models.Meter{}
	for rows.Next() {
		var m models.Meter
		var feed sql.NullString
		if err := rows.Scan(&m.ID, &m.SiteID, &m.Department, &m.Name, &m.WaterType, &feed, &m.Source, &m.Unit, &m.IsActive, &m.IsMainMeter); err != nil {
			continue
		}
		if feed.Valid {
			m.Feed = feed.String
		}
		meters = append(meters, m)
	}

	// Check for errors from iteration
	if err := rows.Err(); err != nil {
		return []models.Meter{}
	}

	return meters
}

// GetMeter returns a meter by ID
func (db *PostgresDB) GetMeter(id string) *models.Meter {
	var m models.Meter
	var feed sql.NullString

	err := db.conn.QueryRow(`
		SELECT id, site_id, department, name, water_type, feed, source, unit, is_active, is_main_meter
		FROM meters WHERE id = $1
	`, id).Scan(&m.ID, &m.SiteID, &m.Department, &m.Name, &m.WaterType, &feed, &m.Source, &m.Unit, &m.IsActive, &m.IsMainMeter)

	if err != nil {
		return nil
	}
	if feed.Valid {
		m.Feed = feed.String
	}
	return &m
}

// GetMeterMap returns a map of meter ID to meter
func (db *PostgresDB) GetMeterMap() map[string]models.Meter {
	meters := db.GetMeters("")
	m := make(map[string]models.Meter)
	for _, meter := range meters {
		m[meter.ID] = meter
	}
	return m
}

// UpsertMeter inserts or updates a meter
func (db *PostgresDB) UpsertMeter(m models.Meter) error {
	_, err := db.conn.Exec(`
		INSERT INTO meters (id, site_id, department, name, water_type, feed, source, unit, is_active, is_main_meter, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO UPDATE SET
			site_id = EXCLUDED.site_id,
			department = EXCLUDED.department,
			name = EXCLUDED.name,
			water_type = EXCLUDED.water_type,
			feed = EXCLUDED.feed,
			source = EXCLUDED.source,
			unit = EXCLUDED.unit,
			is_active = EXCLUDED.is_active,
			is_main_meter = EXCLUDED.is_main_meter,
			updated_at = CURRENT_TIMESTAMP
	`, m.ID, m.SiteID, m.Department, m.Name, m.WaterType, m.Feed, m.Source, m.Unit, m.IsActive, m.IsMainMeter)
	return err
}

// SeedMetersIfEmpty seeds meters only if the meters table is empty
func (db *PostgresDB) SeedMetersIfEmpty() error {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM meters").Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return nil // Already seeded
	}

	return db.SeedMeters()
}

// SeedMeters seeds all meters to the database
func (db *PostgresDB) SeedMeters() error {
	meters := SeedMeters()

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO meters (id, site_id, department, name, water_type, feed, source, unit, is_active, is_main_meter)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			site_id = EXCLUDED.site_id,
			department = EXCLUDED.department,
			name = EXCLUDED.name,
			water_type = EXCLUDED.water_type,
			feed = EXCLUDED.feed,
			source = EXCLUDED.source,
			unit = EXCLUDED.unit,
			is_active = EXCLUDED.is_active,
			is_main_meter = EXCLUDED.is_main_meter,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, m := range meters {
		_, err := stmt.Exec(m.ID, m.SiteID, m.Department, m.Name, m.WaterType, m.Feed, m.Source, m.Unit, m.IsActive, m.IsMainMeter)
		if err != nil {
			return fmt.Errorf("inserting meter %s: %w", m.ID, err)
		}
	}

	return tx.Commit()
}

// ─── Readings ─────────────────────────────────────────────────────────────────

// AddReading adds a new meter reading
func (db *PostgresDB) AddReading(r models.Reading) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}

	// Calculate usage from last reading
	var lastValue float64
	err := db.conn.QueryRow(`
		SELECT value FROM readings 
		WHERE meter_id = $1 
		ORDER BY reading_date DESC 
		LIMIT 1
	`, r.MeterID).Scan(&lastValue)

	if err == nil && lastValue > 0 && r.Value > lastValue {
		r.Usage = r.Value - lastValue
	}

	_, err = db.conn.Exec(`
		INSERT INTO readings (id, meter_id, site_id, value, usage, reading_date, source, notes, water_type, is_estimated)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, r.ID, r.MeterID, r.SiteID, r.Value, r.Usage, r.Date, r.Source, r.Notes, r.WaterType, r.IsEstimated)

	return err
}

// AddReadingBatch adds multiple readings in a single transaction
func (db *PostgresDB) AddReadingBatch(readings []models.Reading) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO readings (id, meter_id, site_id, value, usage, reading_date, source, notes, water_type, is_estimated)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range readings {
		if r.ID == "" {
			r.ID = uuid.New().String()
		}
		_, err := stmt.Exec(r.ID, r.MeterID, r.SiteID, r.Value, r.Usage, r.Date, r.Source, r.Notes, r.WaterType, r.IsEstimated)
		if err != nil {
			return fmt.Errorf("inserting reading for meter %s: %w", r.MeterID, err)
		}
	}

	return tx.Commit()
}

// GetReadings returns readings filtered by criteria
func (db *PostgresDB) GetReadings(siteID, meterID string, from, to time.Time) []models.Reading {
	query := `
		SELECT id, meter_id, site_id, value, usage, reading_date, source, COALESCE(notes, ''), water_type, is_estimated
		FROM readings
		WHERE 1=1
	`
	args := []interface{}{}
	argNum := 1

	if siteID != "" {
		query += fmt.Sprintf(" AND site_id = $%d", argNum)
		args = append(args, siteID)
		argNum++
	}
	if meterID != "" {
		query += fmt.Sprintf(" AND meter_id = $%d", argNum)
		args = append(args, meterID)
		argNum++
	}
	if !from.IsZero() {
		query += fmt.Sprintf(" AND reading_date >= $%d", argNum)
		args = append(args, from)
		argNum++
	}
	if !to.IsZero() {
		query += fmt.Sprintf(" AND reading_date <= $%d", argNum)
		args = append(args, to)
		argNum++
	}

	query += " ORDER BY reading_date ASC"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return []models.Reading{}
	}
	defer rows.Close()

	readings := []models.Reading{}
	for rows.Next() {
		var r models.Reading
		if err := rows.Scan(&r.ID, &r.MeterID, &r.SiteID, &r.Value, &r.Usage, &r.Date, &r.Source, &r.Notes, &r.WaterType, &r.IsEstimated); err != nil {
			continue
		}
		readings = append(readings, r)
	}

	// Check for errors from iteration
	if err := rows.Err(); err != nil {
		return []models.Reading{}
	}

	return readings
}

// GetLastReadingTimes returns the last reading time for each meter
func (db *PostgresDB) GetLastReadingTimes() map[string]time.Time {
	rows, err := db.conn.Query(`
		SELECT meter_id, MAX(reading_date) as last_date
		FROM readings
		GROUP BY meter_id
	`)
	if err != nil {
		return make(map[string]time.Time)
	}
	defer rows.Close()

	result := make(map[string]time.Time)
	for rows.Next() {
		var meterID string
		var lastDate time.Time
		if err := rows.Scan(&meterID, &lastDate); err != nil {
			continue
		}
		result[meterID] = lastDate
	}

	// Check for errors from iteration
	if err := rows.Err(); err != nil {
		return make(map[string]time.Time)
	}

	return result
}

// ─── Tonnes ───────────────────────────────────────────────────────────────────

// AddTonnes adds a new tonnes entry
func (db *PostgresDB) AddTonnes(t models.TonnesEntry) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}

	_, err := db.conn.Exec(`
		INSERT INTO tonnes (id, site_id, department, tonnes, entry_date, notes)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, t.ID, t.SiteID, t.Department, t.Tonnes, t.Date, t.Notes)

	return err
}

// AddTonnesBatch adds multiple tonnes entries in a single transaction
func (db *PostgresDB) AddTonnesBatch(entries []models.TonnesEntry) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO tonnes (id, site_id, department, tonnes, entry_date, notes)
		VALUES ($1, $2, $3, $4, $5, $6)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, t := range entries {
		if t.ID == "" {
			t.ID = uuid.New().String()
		}
		_, err := stmt.Exec(t.ID, t.SiteID, t.Department, t.Tonnes, t.Date, t.Notes)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetTonnes returns tonnes filtered by criteria
func (db *PostgresDB) GetTonnes(siteID string, from, to time.Time) []models.TonnesEntry {
	query := `
		SELECT id, site_id, department, tonnes, entry_date, COALESCE(notes, '')
		FROM tonnes
		WHERE 1=1
	`
	args := []interface{}{}
	argNum := 1

	if siteID != "" {
		query += fmt.Sprintf(" AND site_id = $%d", argNum)
		args = append(args, siteID)
		argNum++
	}
	if !from.IsZero() {
		query += fmt.Sprintf(" AND entry_date >= $%d", argNum)
		args = append(args, from)
		argNum++
	}
	if !to.IsZero() {
		query += fmt.Sprintf(" AND entry_date <= $%d", argNum)
		args = append(args, to)
		argNum++
	}

	query += " ORDER BY entry_date ASC"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return []models.TonnesEntry{}
	}
	defer rows.Close()

	entries := []models.TonnesEntry{}
	for rows.Next() {
		var t models.TonnesEntry
		if err := rows.Scan(&t.ID, &t.SiteID, &t.Department, &t.Tonnes, &t.Date, &t.Notes); err != nil {
			continue
		}
		entries = append(entries, t)
	}

	// Check for errors from iteration
	if err := rows.Err(); err != nil {
		return []models.TonnesEntry{}
	}

	return entries
}

// ─── Auto-Fill ────────────────────────────────────────────────────────────────

// AutoFillMissingData estimates a reading based on historical averages
func (db *PostgresDB) AutoFillMissingData(meterID string, targetDate time.Time) *models.Reading {
	thirtyDaysAgo := targetDate.AddDate(0, 0, -30)

	// Get last reading
	var lastReading models.Reading
	err := db.conn.QueryRow(`
		SELECT id, meter_id, site_id, value, usage, reading_date, source, COALESCE(notes, ''), water_type, is_estimated
		FROM readings
		WHERE meter_id = $1
		ORDER BY reading_date DESC
		LIMIT 1
	`, meterID).Scan(&lastReading.ID, &lastReading.MeterID, &lastReading.SiteID, &lastReading.Value,
		&lastReading.Usage, &lastReading.Date, &lastReading.Source, &lastReading.Notes,
		&lastReading.WaterType, &lastReading.IsEstimated)

	if err != nil {
		return nil
	}

	// Calculate average usage
	var totalUsage float64
	var count int
	err = db.conn.QueryRow(`
		SELECT COALESCE(SUM(usage), 0), COUNT(*)
		FROM readings
		WHERE meter_id = $1 AND reading_date >= $2 AND usage > 0
	`, meterID, thirtyDaysAgo).Scan(&totalUsage, &count)

	if err != nil || count == 0 {
		return nil
	}

	avgDailyUsage := totalUsage / float64(count*7) // Convert weekly to daily
	daysSinceLast := int(targetDate.Sub(lastReading.Date).Hours() / 24)
	estimatedUsage := avgDailyUsage * float64(daysSinceLast)

	// Get meter info
	meter := db.GetMeter(meterID)
	if meter == nil {
		return nil
	}

	return &models.Reading{
		MeterID:     meterID,
		SiteID:      meter.SiteID,
		Value:       lastReading.Value + estimatedUsage,
		Usage:       estimatedUsage,
		Date:        targetDate,
		Source:      models.SourceEstimated,
		Notes:       "Auto-filled estimate based on historical average",
		WaterType:   meter.WaterType,
		IsEstimated: true,
	}
}

// MedianFillMissingData estimates a reading using the MEDIAN of recent usage values
func (db *PostgresDB) MedianFillMissingData(meterID string, targetDate time.Time, lookbackDays int) *models.Reading {
	if lookbackDays <= 0 {
		lookbackDays = 90
	}

	since := targetDate.AddDate(0, 0, -lookbackDays)

	// Get last reading
	var lastReading models.Reading
	err := db.conn.QueryRow(`
		SELECT id, meter_id, site_id, value, usage, reading_date, source, COALESCE(notes, ''), water_type, is_estimated
		FROM readings
		WHERE meter_id = $1
		ORDER BY reading_date DESC
		LIMIT 1
	`, meterID).Scan(&lastReading.ID, &lastReading.MeterID, &lastReading.SiteID, &lastReading.Value,
		&lastReading.Usage, &lastReading.Date, &lastReading.Source, &lastReading.Notes,
		&lastReading.WaterType, &lastReading.IsEstimated)

	if err != nil {
		return nil
	}

	// Get usages for median calculation
	rows, err := db.conn.Query(`
		SELECT usage
		FROM readings
		WHERE meter_id = $1 AND reading_date >= $2 AND usage > 0 AND is_estimated = false
	`, meterID, since)
	if err != nil {
		return nil
	}
	defer rows.Close()

	usages := []float64{}
	for rows.Next() {
		var u float64
		if err := rows.Scan(&u); err == nil {
			usages = append(usages, u)
		}
	}

	// Check for errors from iteration
	if err := rows.Err(); err != nil {
		return nil
	}

	if len(usages) == 0 {
		return nil
	}

	med := median(usages)

	// Get meter info
	meter := db.GetMeter(meterID)
	if meter == nil {
		return nil
	}

	return &models.Reading{
		MeterID:     meterID,
		SiteID:      meter.SiteID,
		Value:       lastReading.Value + med,
		Usage:       med,
		Date:        targetDate,
		Source:      models.SourceEstimated,
		Notes:       "Median-fill estimate based on historical readings",
		WaterType:   meter.WaterType,
		IsEstimated: true,
	}
}

// MedianFillAll fills the latest missing reading for every active meter
func (db *PostgresDB) MedianFillAll(siteID string, targetDate time.Time, freshnessDays, lookbackDays int) (int, error) {
	if freshnessDays <= 0 {
		freshnessDays = 7
	}

	lastReadings := db.GetLastReadingTimes()
	stale := targetDate.AddDate(0, 0, -freshnessDays)
	meters := db.GetMeters(siteID)

	var filled int
	for _, m := range meters {
		if !m.IsActive {
			continue
		}
		if t, ok := lastReadings[m.ID]; ok && t.After(stale) {
			continue
		}
		est := db.MedianFillMissingData(m.ID, targetDate, lookbackDays)
		if est == nil {
			continue
		}
		if err := db.AddReading(*est); err != nil {
			return filled, err
		}
		filled++
	}
	return filled, nil
}

// ─── Preferences ──────────────────────────────────────────────────────────────

// GetPreferences returns user preferences
func (db *PostgresDB) GetPreferences() models.UserPreferences {
	var prefs models.UserPreferences
	err := db.conn.QueryRow(`
		SELECT theme, auto_fill_enabled, default_date_range
		FROM preferences
		WHERE id = 1
	`).Scan(&prefs.Theme, &prefs.AutoFillEnabled, &prefs.DefaultDateRange)

	if err != nil {
		return models.DefaultPreferences()
	}
	return prefs
}

// UpdatePreferences updates user preferences
func (db *PostgresDB) UpdatePreferences(prefs models.UserPreferences) error {
	_, err := db.conn.Exec(`
		UPDATE preferences
		SET theme = COALESCE(NULLIF($1, ''), theme),
		    auto_fill_enabled = $2,
		    default_date_range = CASE WHEN $3 > 0 THEN $3 ELSE default_date_range END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, prefs.Theme, prefs.AutoFillEnabled, prefs.DefaultDateRange)
	return err
}

// ─── Connection Status ────────────────────────────────────────────────────────

// GetConnectionStatus returns the connection status
func (db *PostgresDB) GetConnectionStatus() models.ConnectionStatus {
	// Check database connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	status := models.ConnectionStatus{
		LastCheck: time.Now(),
	}

	if err := db.conn.PingContext(ctx); err != nil {
		// Database is down but we don't have a Database field in ConnectionStatus
		// The status struct is designed for external integrations (EEmon, Trend)
		// We just return the current timestamp
		return status
	}

	return status
}

// ─── Clear Data ───────────────────────────────────────────────────────────────

// ClearData clears all readings and tonnes data
func (db *PostgresDB) ClearData() error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM readings"); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM tonnes"); err != nil {
		return err
	}

	return tx.Commit()
}

// ─── Seed Data ────────────────────────────────────────────────────────────────

// SeedSampleData populates the database with sample data
func (db *PostgresDB) SeedSampleData() (int, error) {
	// Clear existing readings
	if err := db.ClearData(); err != nil {
		return 0, err
	}

	// Generate sample readings using the same logic as JSON version
	readings, tonnes := generateSampleData(db.GetMeterMap())

	// Insert readings in batch
	if err := db.AddReadingBatch(readings); err != nil {
		return 0, err
	}

	// Insert tonnes in batch
	if err := db.AddTonnesBatch(tonnes); err != nil {
		return 0, err
	}

	return len(readings), nil
}

// generateSampleData creates sample readings and tonnes entries
func generateSampleData(meterMap map[string]models.Meter) ([]models.Reading, []models.TonnesEntry) {
	var readings []models.Reading
	var tonnes []models.TonnesEntry

	baseDate := time.Now().AddDate(0, -6, 0)
	makeID := func() string { return uuid.New().String() }

	// Rotherham weekly readings
	rotherhamRows := [][10]float64{
		{122732, 4154, 6410, 56339, 612372, 404781, 886932, 103675, 9276930, 430129},
		{122748, 4155, 6423, 56339, 612372, 404781, 887060, 103694, 9279635, 432449},
		{122765, 4155, 6444, 56339, 612372, 404781, 887208, 103715, 9282337, 434384},
		{122778, 4157, 6456, 56339, 612372, 404781, 887332, 103733, 9285069, 435659},
		{122789, 4159, 6467, 56339, 612372, 404781, 887455, 103749, 9287612, 435891},
		{122800, 4160, 6479, 56339, 612372, 404781, 887576, 103765, 9290313, 436051},
		{122814, 4160, 6491, 56340, 612372, 404781, 887576, 103784, 9292913, 436204},
		{122829, 4161, 6503, 56340, 612372, 404781, 887576, 103796, 9295551, 436501},
		{122839, 4161, 6537, 56340, 612372, 404781, 887576, 103801, 9302665, 437121},
		{122847, 4162, 6542, 56340, 612372, 404781, 887576, 103813, 9303396, 437245},
	}

	meterIDs := []string{"r-acp", "r-abctw", "r-ams", "r-aocc", "r-ks", "r-vd", "r-bbr", "r-tbm", "r-cew", "r-boc"}

	for i, row := range rotherhamRows {
		dt := baseDate.AddDate(0, 0, i*7)
		for j, val := range row {
			if j >= len(meterIDs) {
				break
			}
			mID := meterIDs[j]
			m := meterMap[mID]
			readings = append(readings, models.Reading{
				ID:        makeID(),
				MeterID:   mID,
				SiteID:    models.SiteRotherham,
				Value:     val,
				Date:      dt,
				Source:    models.SourceManual,
				WaterType: m.WaterType,
			})
		}
	}

	// Stocksbridge EEmon readings
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
		m := meterMap[sd.meterID]
		readings = append(readings, models.Reading{
			ID:        makeID(),
			MeterID:   sd.meterID,
			SiteID:    models.SiteStocksbridge,
			Value:     sd.value,
			Date:      sbBase.AddDate(0, 0, sd.offset),
			Source:    models.SourceEEmon,
			WaterType: m.WaterType,
		})
	}

	// Brinsworth readings
	brBase := time.Now().AddDate(0, -3, 0)
	brData := []struct {
		meterID string
		offset  int
		value   float64
	}{
		{"b-ttw", 0, 45000}, {"b-glr", 0, 120000}, {"b-hmr", 0, 85000}, {"b-hrp", 0, 32000}, {"b-cmr", 0, 28000},
		{"b-ttw", 30, 48500}, {"b-glr", 30, 128000}, {"b-hmr", 30, 92000}, {"b-hrp", 30, 35000}, {"b-cmr", 30, 31000},
		{"b-ttw", 60, 52000}, {"b-glr", 60, 135000}, {"b-hmr", 60, 98000}, {"b-hrp", 60, 38000}, {"b-cmr", 60, 34000},
		{"b-ttw", 90, 55500}, {"b-glr", 90, 142000}, {"b-hmr", 90, 105000}, {"b-hrp", 90, 41000}, {"b-cmr", 90, 37000},
	}
	for _, bd := range brData {
		m := meterMap[bd.meterID]
		readings = append(readings, models.Reading{
			ID:        makeID(),
			MeterID:   bd.meterID,
			SiteID:    models.SiteBrinsworth,
			Value:     bd.value,
			Date:      brBase.AddDate(0, 0, bd.offset),
			Source:    m.Source,
			WaterType: m.WaterType,
		})
	}

	// Wednesbury readings
	wdBase := time.Now().AddDate(0, -3, 0)
	wdData := []struct {
		meterID string
		offset  int
		value   float64
	}{
		{"w-m1", 0, 15000}, {"w-m2", 0, 12000}, {"w-pro", 0, 8500}, {"w-ame", 0, 3200},
		{"w-m1", 30, 16200}, {"w-m2", 30, 13100}, {"w-pro", 30, 9200}, {"w-ame", 30, 3450},
		{"w-m1", 60, 17400}, {"w-m2", 60, 14200}, {"w-pro", 60, 9900}, {"w-ame", 60, 3700},
		{"w-m1", 90, 18600}, {"w-m2", 90, 15300}, {"w-pro", 90, 10600}, {"w-ame", 90, 3950},
	}
	for _, wd := range wdData {
		m := meterMap[wd.meterID]
		readings = append(readings, models.Reading{
			ID:        makeID(),
			MeterID:   wd.meterID,
			SiteID:    models.SiteWednesbury,
			Value:     wd.value,
			Date:      wdBase.AddDate(0, 0, wd.offset),
			Source:    m.Source,
			WaterType: m.WaterType,
		})
	}

	// Compute usage deltas
	byMeter := make(map[string][]models.Reading)
	for i := range readings {
		byMeter[readings[i].MeterID] = append(byMeter[readings[i].MeterID], readings[i])
	}
	readings = []models.Reading{}
	for _, rds := range byMeter {
		sort.Slice(rds, func(i, j int) bool { return rds[i].Date.Before(rds[j].Date) })
		for i := range rds {
			if i > 0 && rds[i].Value > rds[i-1].Value {
				rds[i].Usage = rds[i].Value - rds[i-1].Value
			}
			readings = append(readings, rds[i])
		}
	}

	// Seed tonnes
	m1ago := time.Now().AddDate(0, -1, 0)
	m2ago := time.Now().AddDate(0, -2, 0)
	m3ago := time.Now().AddDate(0, -3, 0)
	tonnes = []models.TonnesEntry{
		{ID: makeID(), SiteID: models.SiteRotherham, Department: models.DeptACP, Tonnes: 8500, Date: m3ago},
		{ID: makeID(), SiteID: models.SiteRotherham, Department: models.DeptBBR, Tonnes: 3200, Date: m3ago},
		{ID: makeID(), SiteID: models.SiteRotherham, Department: models.DeptTCM, Tonnes: 12000, Date: m3ago},
		{ID: makeID(), SiteID: models.SiteRotherham, Department: models.DeptEngineeringServices, Tonnes: 2000, Date: m3ago},
		{ID: makeID(), SiteID: models.SiteRotherham, Department: models.DeptACP, Tonnes: 9100, Date: m2ago},
		{ID: makeID(), SiteID: models.SiteRotherham, Department: models.DeptBBR, Tonnes: 3400, Date: m2ago},
		{ID: makeID(), SiteID: models.SiteRotherham, Department: models.DeptTCM, Tonnes: 11800, Date: m2ago},
		{ID: makeID(), SiteID: models.SiteRotherham, Department: models.DeptEngineeringServices, Tonnes: 2100, Date: m2ago},
		{ID: makeID(), SiteID: models.SiteRotherham, Department: models.DeptACP, Tonnes: 9300, Date: m1ago},
		{ID: makeID(), SiteID: models.SiteRotherham, Department: models.DeptBBR, Tonnes: 3500, Date: m1ago},
		{ID: makeID(), SiteID: models.SiteRotherham, Department: models.DeptTCM, Tonnes: 12300, Date: m1ago},
		{ID: makeID(), SiteID: models.SiteRotherham, Department: models.DeptEngineeringServices, Tonnes: 2200, Date: m1ago},
		{ID: makeID(), SiteID: models.SiteStocksbridge, Department: models.DeptRemelt, Tonnes: 15000, Date: m3ago},
		{ID: makeID(), SiteID: models.SiteStocksbridge, Department: models.DeptBilletMill, Tonnes: 22000, Date: m3ago},
		{ID: makeID(), SiteID: models.SiteStocksbridge, Department: models.DeptRemelt, Tonnes: 14500, Date: m2ago},
		{ID: makeID(), SiteID: models.SiteStocksbridge, Department: models.DeptBilletMill, Tonnes: 21000, Date: m2ago},
		{ID: makeID(), SiteID: models.SiteBrinsworth, Department: models.DeptHotMill, Tonnes: 18000, Date: m3ago},
		{ID: makeID(), SiteID: models.SiteBrinsworth, Department: models.DeptColdMill, Tonnes: 12000, Date: m3ago},
		{ID: makeID(), SiteID: models.SiteBrinsworth, Department: models.DeptHotMill, Tonnes: 19000, Date: m2ago},
		{ID: makeID(), SiteID: models.SiteBrinsworth, Department: models.DeptColdMill, Tonnes: 12500, Date: m2ago},
		{ID: makeID(), SiteID: models.SiteBrinsworth, Department: models.DeptHRP, Tonnes: 8000, Date: m2ago},
		{ID: makeID(), SiteID: models.SiteBrinsworth, Department: models.DeptHotMill, Tonnes: 20000, Date: m1ago},
		{ID: makeID(), SiteID: models.SiteBrinsworth, Department: models.DeptColdMill, Tonnes: 13000, Date: m1ago},
		{ID: makeID(), SiteID: models.SiteBrinsworth, Department: models.DeptHRP, Tonnes: 8500, Date: m1ago},
		{ID: makeID(), SiteID: models.SiteWednesbury, Department: models.DeptProduction, Tonnes: 5000, Date: m3ago},
		{ID: makeID(), SiteID: models.SiteWednesbury, Department: models.DeptAmenities, Tonnes: 500, Date: m3ago},
		{ID: makeID(), SiteID: models.SiteWednesbury, Department: models.DeptProduction, Tonnes: 5200, Date: m2ago},
		{ID: makeID(), SiteID: models.SiteWednesbury, Department: models.DeptAmenities, Tonnes: 520, Date: m2ago},
	}

	return readings, tonnes
}
