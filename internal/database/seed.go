package database

import (
	"fmt"
	"sort"
	"time"

	"water-monitoring-system/internal/models"
)

// SeedMeters returns the default meter configuration
func SeedMeters() []models.Meter {
	return []models.Meter{
		// ── ROTHERHAM ──────────────────────────────────────────────────────
		// ACP Department
		{ID: "r-acp", SiteID: models.SiteRotherham, Department: models.DeptACP, Name: "ACP", WaterType: models.TownsWater, Feed: "TCM Feed", Source: models.SourceManual, Unit: "m3", IsActive: true, IsMainMeter: true},
		{ID: "r-abctw", SiteID: models.SiteRotherham, Department: models.DeptACP, Name: "ABC TW", WaterType: models.TownsWater, Feed: "TCM Feed", Source: models.SourceManual, Unit: "m3", IsActive: true},
		{ID: "r-ams", SiteID: models.SiteRotherham, Department: models.DeptACP, Name: "AMS Amenities", WaterType: models.TownsWater, Feed: "TCM Feed", Source: models.SourceManual, Unit: "m3", IsActive: true},
		{ID: "r-aocc", SiteID: models.SiteRotherham, Department: models.DeptACP, Name: "AOCC", WaterType: models.TownsWater, Feed: "Aldwarke Lane", Source: models.SourceManual, Unit: "m3", IsActive: true},
		{ID: "r-ks", SiteID: models.SiteRotherham, Department: models.DeptACP, Name: "Kress Square", WaterType: models.TownsWater, Feed: "TCM Feed", Source: models.SourceManual, Unit: "m3", IsActive: true},
		{ID: "r-vd", SiteID: models.SiteRotherham, Department: models.DeptACP, Name: "VAC Degasser", WaterType: models.TownsWater, Feed: "TCM Feed", Source: models.SourceManual, Unit: "m3", IsActive: true},
		// BBR Department
		{ID: "r-bbr", SiteID: models.SiteRotherham, Department: models.DeptBBR, Name: "BBR - Greenlane", WaterType: models.TownsWater, Feed: "Aldwarke Lane", Source: models.SourceManual, Unit: "m3", IsActive: true, IsMainMeter: true},
		// TCM Department
		{ID: "r-tbm", SiteID: models.SiteRotherham, Department: models.DeptTCM, Name: "TCM - Tunnel", WaterType: models.TownsWater, Feed: "TCM Feed", Source: models.SourceManual, Unit: "m3", IsActive: true, IsMainMeter: true},
		// Engineering Services Department
		{ID: "r-cew", SiteID: models.SiteRotherham, Department: models.DeptEngineeringServices, Name: "Cew Graveyard", WaterType: models.TownsWater, Feed: "Green Lane", Source: models.SourceManual, Unit: "m3", IsActive: true, IsMainMeter: true},
		{ID: "r-ago", SiteID: models.SiteRotherham, Department: models.DeptEngineeringServices, Name: "AGO", WaterType: models.TownsWater, Feed: "Green Lane", Source: models.SourceManual, Unit: "m3", IsActive: true},
		{ID: "r-cgl", SiteID: models.SiteRotherham, Department: models.DeptEngineeringServices, Name: "APM", WaterType: models.TownsWater, Feed: "Aldwarke Lane", Source: models.SourceManual, Unit: "m3", IsActive: true},
		{ID: "r-boc", SiteID: models.SiteRotherham, Department: models.DeptEngineeringServices, Name: "BOC", WaterType: models.TownsWater, Feed: "Aldwarke Lane", Source: models.SourceManual, Unit: "m3", IsActive: true},

		// ── STOCKSBRIDGE ───────────────────────────────────────────────────
		// Towns Water
		{ID: "s-rms1", SiteID: models.SiteStocksbridge, Department: models.DeptRemelt, Name: "RMS 1", WaterType: models.TownsWater, Feed: "East Bank", Source: models.SourceEEmon, Unit: "litres", IsActive: true},
		{ID: "s-rms2", SiteID: models.SiteStocksbridge, Department: models.DeptRemelt, Name: "RMS 2", WaterType: models.TownsWater, Feed: "East Bank", Source: models.SourceEEmon, Unit: "litres", IsActive: true},
		{ID: "s-vim", SiteID: models.SiteStocksbridge, Department: models.DeptRemelt, Name: "VIM", WaterType: models.TownsWater, Feed: "East Bank", Source: models.SourceEEmon, Unit: "litres", IsActive: true},
		{ID: "s-bm16", SiteID: models.SiteStocksbridge, Department: models.DeptBilletMill, Name: "BM Meter 16", WaterType: models.TownsWater, Feed: "East Bank", Source: models.SourceEEmon, Unit: "litres", IsActive: true},
		{ID: "s-bm17", SiteID: models.SiteStocksbridge, Department: models.DeptBilletMill, Name: "BM Meter 17", WaterType: models.TownsWater, Feed: "East Bank", Source: models.SourceEEmon, Unit: "litres", IsActive: true},
		{ID: "s-bms14", SiteID: models.SiteStocksbridge, Department: models.DeptBilletMill, Name: "BM Saws Meter 14", WaterType: models.TownsWater, Feed: "East Bank", Source: models.SourceEEmon, Unit: "litres", IsActive: true},
		{ID: "s-croft", SiteID: models.SiteStocksbridge, Department: models.DeptCroft, Name: "Croft Supply", WaterType: models.TownsWater, Feed: "East Bank", Source: models.SourceEEmon, Unit: "litres", IsActive: true},
		// River Water
		{ID: "s-wq1", SiteID: models.SiteStocksbridge, Department: models.DeptWestBank, Name: "Water Quench 1", WaterType: models.RiverWater, Feed: "West Bank", Source: models.SourceEEmon, Unit: "litres", IsActive: true},
		{ID: "s-wq2", SiteID: models.SiteStocksbridge, Department: models.DeptWestBank, Name: "Water Quench 2", WaterType: models.RiverWater, Feed: "West Bank", Source: models.SourceEEmon, Unit: "litres", IsActive: true},
		{ID: "s-sb", SiteID: models.SiteStocksbridge, Department: models.DeptSpringBank, Name: "Spring Bank", WaterType: models.RiverWater, Feed: "Spring Bank", Source: models.SourceEEmon, Unit: "litres", IsActive: true},
		// Manual meters
		{ID: "s-wbwa", SiteID: models.SiteStocksbridge, Department: models.DeptWestBank, Name: "West Bank Water Abstraction Meter", WaterType: models.RiverWater, Feed: "West Bank", Source: models.SourceManual, Unit: "m3", IsActive: true},
		{ID: "s-wbw", SiteID: models.SiteStocksbridge, Department: models.DeptWestBank, Name: "West Bank Water", WaterType: models.RiverWater, Feed: "West Bank", Source: models.SourceManual, Unit: "m3", IsActive: true},
		{ID: "s-wbg", SiteID: models.SiteStocksbridge, Department: models.DeptWestBank, Name: "West Bank Gardens", WaterType: models.RiverWater, Feed: "West Bank", Source: models.SourceManual, Unit: "m3", IsActive: true},
		{ID: "s-g2", SiteID: models.SiteStocksbridge, Department: models.DeptGeneral, Name: "No.2 Gate Meter", WaterType: models.TownsWater, Feed: "East Bank", Source: models.SourceManual, Unit: "m3", IsActive: true},
		{ID: "s-g1", SiteID: models.SiteStocksbridge, Department: models.DeptGeneral, Name: "No.1 Gate (Hawthorne Brook)", WaterType: models.TownsWater, Feed: "East Bank", Source: models.SourceManual, Unit: "m3", IsActive: true},
		{ID: "s-ebte", SiteID: models.SiteStocksbridge, Department: models.DeptEastBank, Name: "EB Trade Effluent", WaterType: models.TownsWater, Feed: "East Bank", Source: models.SourceManual, Unit: "m3", IsActive: true},
		{ID: "s-ebn", SiteID: models.SiteStocksbridge, Department: models.DeptEastBank, Name: "EB Network", WaterType: models.TownsWater, Feed: "East Bank", Source: models.SourceManual, Unit: "m3", IsActive: true},
		{ID: "s-ebt", SiteID: models.SiteStocksbridge, Department: models.DeptEastBank, Name: "EB Tank Meter", WaterType: models.TownsWater, Feed: "East Bank", Source: models.SourceManual, Unit: "m3", IsActive: true},
		// Other EEmon
		{ID: "s-wboq", SiteID: models.SiteStocksbridge, Department: models.DeptWestBank, Name: "West Bank Oil Quench", WaterType: models.TownsWater, Feed: "West Bank", Source: models.SourceEEmon, Unit: "litres", IsActive: true},
		{ID: "s-wbcht", SiteID: models.SiteStocksbridge, Department: models.DeptWestBank, Name: "West Bank CHT", WaterType: models.TownsWater, Feed: "West Bank", Source: models.SourceEEmon, Unit: "litres", IsActive: true},
		{ID: "s-bbr", SiteID: models.SiteStocksbridge, Department: models.DeptBilletMill, Name: "Billet Bank Reeler", WaterType: models.TownsWater, Feed: "East Bank", Source: models.SourceEEmon, Unit: "litres", IsActive: true},
		{ID: "s-esr1", SiteID: models.SiteStocksbridge, Department: models.DeptEastBank, Name: "ESR 1", WaterType: models.TownsWater, Feed: "East Bank", Source: models.SourceEEmon, Unit: "litres", IsActive: true},
		{ID: "s-eb", SiteID: models.SiteStocksbridge, Department: models.DeptEastBank, Name: "East Bank Total", WaterType: models.TownsWater, Feed: "East Bank", Source: models.SourceEEmon, Unit: "litres", IsActive: true},

		// ── BRINSWORTH ─────────────────────────────────────────────────────
		{ID: "b-ttw", SiteID: models.SiteBrinsworth, Department: models.DeptGeneral, Name: "Total Towns Water", WaterType: models.TownsWater, Feed: "Towns", Source: models.SourceManual, Unit: "m3", IsActive: true},
		{ID: "b-glr", SiteID: models.SiteBrinsworth, Department: models.DeptGeneral, Name: "Grange Lane Reservoir (Riverside Pumphouse)", WaterType: models.RiverWater, Feed: "River", Source: models.SourceManual, Unit: "m3", IsActive: true},
		{ID: "b-hmr", SiteID: models.SiteBrinsworth, Department: models.DeptHotMill, Name: "Hot Mill River Water Meter", WaterType: models.RiverWater, Feed: "River", Source: models.SourceTrend, Unit: "m3", IsActive: true},
		{ID: "b-hrp", SiteID: models.SiteBrinsworth, Department: models.DeptHRP, Name: "HRP River Water Meter", WaterType: models.RiverWater, Feed: "River", Source: models.SourceTrend, Unit: "m3", IsActive: true},
		{ID: "b-cmr", SiteID: models.SiteBrinsworth, Department: models.DeptColdMill, Name: "Cold Mill River Water Meter", WaterType: models.RiverWater, Feed: "River", Source: models.SourceTrend, Unit: "m3", IsActive: true},

		// ── WEDNESBURY ─────────────────────────────────────────────────────
		{ID: "w-m1", SiteID: models.SiteWednesbury, Department: models.DeptGeneral, Name: "Main Meter 1", WaterType: models.TownsWater, Feed: "Mains", Source: models.SourceManual, Unit: "m3", IsActive: true},
		{ID: "w-m2", SiteID: models.SiteWednesbury, Department: models.DeptGeneral, Name: "Main Meter 2", WaterType: models.TownsWater, Feed: "Mains", Source: models.SourceManual, Unit: "m3", IsActive: true},
		{ID: "w-pro", SiteID: models.SiteWednesbury, Department: models.DeptProduction, Name: "Production Meter", WaterType: models.TownsWater, Feed: "Mains", Source: models.SourceManual, Unit: "m3", IsActive: true},
		{ID: "w-ame", SiteID: models.SiteWednesbury, Department: models.DeptAmenities, Name: "Amenities Meter", WaterType: models.TownsWater, Feed: "Mains", Source: models.SourceManual, Unit: "m3", IsActive: true},
	}
}

// seedSampleReadings populates sample readings data
func seedSampleReadings(db *DB) {
	baseDate := time.Now().AddDate(0, -6, 0)
	makeID := func() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }

	// Rotherham weekly readings (23 weeks of data)
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
		{123314, 4166, 6549, 56340, 612372, 404781, 887576, 103834, 9306427, 437991},
		{123337, 4168, 6549, 56340, 612372, 404781, 887576, 103861, 9309562, 438272},
		{123351, 4170, 6549, 56340, 612372, 404781, 887576, 103877, 9311927, 438562},
		{123372, 4171, 6549, 56340, 612372, 404781, 887576, 103899, 9314663, 438644},
		{123397, 4171, 6549, 56340, 612372, 404781, 887576, 103921, 9317478, 439399},
		{123421, 4172, 6549, 56341, 612373, 404781, 887576, 103948, 9320648, 439692},
		{123440, 4172, 6549, 56341, 612373, 404781, 887576, 103967, 9323057, 440037},
		{123460, 4172, 6549, 56341, 612373, 404781, 887576, 103989, 9325874, 440922},
		{123490, 4174, 6549, 56341, 612373, 404781, 887576, 104007, 9328657, 441622},
		{123523, 4176, 6549, 56340, 612373, 404781, 887576, 104028, 9331469, 442237},
		{123547, 4176, 6549, 56340, 612373, 404781, 887576, 104047, 9334187, 442589},
		{123574, 4176, 6549, 56341, 612373, 404781, 887576, 104064, 9337073, 442887},
		{123600, 4176, 6549, 56341, 612373, 404781, 887576, 104080, 9339928, 443200},
	}

	meterIDs := []string{"r-acp", "r-abctw", "r-ams", "r-aocc", "r-ks", "r-vd", "r-bbr", "r-tbm", "r-cew", "r-boc"}
	meterMap := make(map[string]models.Meter)
	for _, m := range db.Meters {
		meterMap[m.ID] = m
	}

	for i, row := range rotherhamRows {
		dt := baseDate.AddDate(0, 0, i*7)
		for j, val := range row {
			if j >= len(meterIDs) {
				break
			}
			mID := meterIDs[j]
			m := meterMap[mID]
			db.Readings = append(db.Readings, models.Reading{
				ID:        makeID(),
				MeterID:   mID,
				SiteID:    models.SiteRotherham,
				Value:     val,
				Date:      dt,
				Source:    models.SourceManual,
				WaterType: m.WaterType,
			})
			time.Sleep(time.Nanosecond)
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
		db.Readings = append(db.Readings, models.Reading{
			ID:        makeID(),
			MeterID:   sd.meterID,
			SiteID:    models.SiteStocksbridge,
			Value:     sd.value,
			Date:      sbBase.AddDate(0, 0, sd.offset),
			Source:    models.SourceEEmon,
			WaterType: m.WaterType,
		})
		time.Sleep(time.Nanosecond)
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
		db.Readings = append(db.Readings, models.Reading{
			ID:        makeID(),
			MeterID:   bd.meterID,
			SiteID:    models.SiteBrinsworth,
			Value:     bd.value,
			Date:      brBase.AddDate(0, 0, bd.offset),
			Source:    m.Source,
			WaterType: m.WaterType,
		})
		time.Sleep(time.Nanosecond)
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
		db.Readings = append(db.Readings, models.Reading{
			ID:        makeID(),
			MeterID:   wd.meterID,
			SiteID:    models.SiteWednesbury,
			Value:     wd.value,
			Date:      wdBase.AddDate(0, 0, wd.offset),
			Source:    m.Source,
			WaterType: m.WaterType,
		})
		time.Sleep(time.Nanosecond)
	}

	// Compute usage deltas
	byMeter := make(map[string][]models.Reading)
	for i, rd := range db.Readings {
		byMeter[rd.MeterID] = append(byMeter[rd.MeterID], db.Readings[i])
	}
	db.Readings = []models.Reading{}
	for _, rds := range byMeter {
		sort.Slice(rds, func(i, j int) bool { return rds[i].Date.Before(rds[j].Date) })
		for i := range rds {
			if i > 0 && rds[i].Value > rds[i-1].Value {
				rds[i].Usage = rds[i].Value - rds[i-1].Value
			}
			db.Readings = append(db.Readings, rds[i])
		}
	}

	// Seed tonnes
	m1ago := time.Now().AddDate(0, -1, 0)
	m2ago := time.Now().AddDate(0, -2, 0)
	m3ago := time.Now().AddDate(0, -3, 0)
	db.Tonnes = []models.TonnesEntry{
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
		{ID: makeID(), SiteID: models.SiteWednesbury, Department: models.DeptProduction, Tonnes: 5400, Date: m1ago},
		{ID: makeID(), SiteID: models.SiteWednesbury, Department: models.DeptAmenities, Tonnes: 540, Date: m1ago},
	}
}
