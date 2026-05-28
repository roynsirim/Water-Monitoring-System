package database

import (
	"sort"
	"time"

	"water-monitoring-system/internal/models"
)

// MedianFillMissingData estimates a reading using the MEDIAN of recent usage
// values for the meter (more robust to outliers than the average used by
// AutoFillMissingData).
//
// - lookbackDays: how far back to gather historical usages (default 90 if <=0)
// - returns nil if there is no historical data to base an estimate on.
func (db *DB) MedianFillMissingData(meterID string, targetDate time.Time, lookbackDays int) *models.Reading {
	if lookbackDays <= 0 {
		lookbackDays = 90
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	since := targetDate.AddDate(0, 0, -lookbackDays)
	var usages []float64
	var lastReading *models.Reading
	var waterType models.WaterType
	var siteID string

	for i := len(db.Readings) - 1; i >= 0; i-- {
		rd := db.Readings[i]
		if rd.MeterID != meterID {
			continue
		}
		if lastReading == nil {
			lastReading = &db.Readings[i]
		}
		if rd.Date.After(since) && rd.Usage > 0 && !rd.IsEstimated {
			usages = append(usages, rd.Usage)
		}
	}

	for _, m := range db.Meters {
		if m.ID == meterID {
			waterType = m.WaterType
			siteID = m.SiteID
			break
		}
	}

	if len(usages) == 0 || lastReading == nil {
		return nil
	}

	med := median(usages)

	return &models.Reading{
		MeterID:     meterID,
		SiteID:      siteID,
		Value:       lastReading.Value + med,
		Usage:       med,
		Date:        targetDate,
		Source:      models.SourceEstimated,
		Notes:       "Median-fill estimate based on historical readings",
		WaterType:   waterType,
		IsEstimated: true,
	}
}

// MedianFillAll fills the latest missing reading for every active meter that
// hasn't reported in the given freshness window. Returns the number of
// readings inserted.
func (db *DB) MedianFillAll(siteID string, targetDate time.Time, freshnessDays, lookbackDays int) (int, error) {
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

func median(xs []float64) float64 {
	cp := make([]float64, len(xs))
	copy(cp, xs)
	sort.Float64s(cp)
	n := len(cp)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return cp[n/2]
	}
	return (cp[n/2-1] + cp[n/2]) / 2
}
