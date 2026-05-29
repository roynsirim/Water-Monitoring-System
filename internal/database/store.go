package database

import (
	"time"

	"water-monitoring-system/internal/models"
)

// Store defines the interface for database operations
// Both JSON and PostgreSQL implementations satisfy this interface
type Store interface {
	// Save persists any pending changes (no-op for PostgreSQL)
	Save() error

	// Sites
	GetSites() []models.Site
	GetSiteNames() map[string]string

	// Meters
	GetMeters(siteID string) []models.Meter
	GetMeter(id string) *models.Meter
	GetMeterMap() map[string]models.Meter

	// Readings
	AddReading(r models.Reading) error
	GetReadings(siteID, meterID string, from, to time.Time) []models.Reading
	GetLastReadingTimes() map[string]time.Time

	// Tonnes
	AddTonnes(t models.TonnesEntry) error
	GetTonnes(siteID string, from, to time.Time) []models.TonnesEntry

	// Auto-fill
	AutoFillMissingData(meterID string, targetDate time.Time) *models.Reading
	MedianFillMissingData(meterID string, targetDate time.Time, lookbackDays int) *models.Reading
	MedianFillAll(siteID string, targetDate time.Time, freshnessDays, lookbackDays int) (int, error)

	// Preferences
	GetPreferences() models.UserPreferences
	UpdatePreferences(prefs models.UserPreferences) error

	// Connection Status
	GetConnectionStatus() models.ConnectionStatus

	// Data Management
	ClearData() error
	SeedSampleData() (int, error)
}

// Ensure both implementations satisfy the interface
var _ Store = (*DB)(nil)
var _ Store = (*PostgresDB)(nil)

// OpenStore opens a database connection based on driver type
// Returns a Store interface that works with either JSON or PostgreSQL
func OpenStore(driver, path, dsn string) (Store, error) {
	switch driver {
	case "postgres":
		pdb, err := OpenPostgres(dsn)
		if err != nil {
			return nil, err
		}
		// Seed meters if the table is empty
		if err := pdb.SeedMetersIfEmpty(); err != nil {
			return nil, err
		}
		return pdb, nil
	default:
		// Default to JSON store
		return Open(path)
	}
}
