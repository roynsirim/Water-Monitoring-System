package reports

import (
	"encoding/csv"
	"fmt"
	"io"
	"time"

	"water-monitoring-system/internal/models"
)

// WriteReadingsCSV writes a slice of readings to w in CSV format
func WriteReadingsCSV(w io.Writer, results []models.SearchResult) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	// Header
	if err := cw.Write([]string{
		"Date", "Site", "Department", "Meter",
		"Reading", "Usage_m3", "Water_Type", "Source", "Notes",
	}); err != nil {
		return err
	}

	for _, r := range results {
		if err := cw.Write([]string{
			r.Reading.Date.Format("2006-01-02"),
			r.SiteName,
			r.Department,
			r.MeterName,
			fmt.Sprintf("%.2f", r.Reading.Value),
			fmt.Sprintf("%.2f", r.M3),
			string(r.Reading.WaterType),
			r.Reading.Source,
			r.Reading.Notes,
		}); err != nil {
			return err
		}
	}

	return cw.Error()
}

// WriteSiteReportCSV writes a site report summary to w in CSV format
func WriteSiteReportCSV(w io.Writer, report models.Report) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	// Header
	cw.Write([]string{"Water Usage Report"})
	cw.Write([]string{"Period", report.Period})
	cw.Write([]string{"Generated", report.Generated.Format("2006-01-02 15:04:05")})
	cw.Write([]string{})

	for _, site := range report.Sites {
		cw.Write([]string{site.SiteName})
		cw.Write([]string{"Metric", "Value"})
		cw.Write([]string{"Total m³", fmt.Sprintf("%.2f", site.TotalM3)})
		cw.Write([]string{"Towns m³", fmt.Sprintf("%.2f", site.TownsM3)})
		cw.Write([]string{"River m³", fmt.Sprintf("%.2f", site.RiverM3)})
		cw.Write([]string{"Total Tonnes", fmt.Sprintf("%.2f", site.TotalTonnes)})
		cw.Write([]string{"m³/tonne", fmt.Sprintf("%.4f", site.M3PerTonne)})
		cw.Write([]string{})

		if len(site.Departments) > 0 {
			cw.Write([]string{"Department", "m³", "Towns m³", "River m³", "Tonnes", "m³/tonne"})
			for _, d := range site.Departments {
				cw.Write([]string{
					d.Department,
					fmt.Sprintf("%.2f", d.M3),
					fmt.Sprintf("%.2f", d.TownsM3),
					fmt.Sprintf("%.2f", d.RiverM3),
					fmt.Sprintf("%.2f", d.Tonnes),
					fmt.Sprintf("%.4f", d.M3PerTonne),
				})
			}
			cw.Write([]string{})
		}
	}

	return cw.Error()
}

// Filename returns a timestamped CSV filename for a report
func Filename(prefix string) string {
	return fmt.Sprintf("%s_%s.csv", prefix, time.Now().Format("20060102_1504"))
}
