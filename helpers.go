package main

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// containsSwedishMonth checks if a string contains a Swedish month name.
func containsSwedishMonth(upper string) bool {
	months := []string{
		"JANUARI", "FEBRUARI", "MARS", "APRIL", "MAJ", "JUNI",
		"JULI", "AUGUSTI", "SEPTEMBER", "OKTOBER", "NOVEMBER", "DECEMBER",
	}
	for _, m := range months {
		if strings.Contains(upper, m) {
			return true
		}
	}
	return false
}

// dismissCookieConsent returns JS that clicks a Swedish cookie consent button.
func dismissCookieConsentJS() string {
	return `
		(() => {
			const buttons = Array.from(document.querySelectorAll("button, a"));
			const acceptBtn = buttons.find(b => 
				b.textContent.trim().toLowerCase().includes('godkänn alla') ||
				b.textContent.trim().toLowerCase().includes('acceptera') ||
				b.textContent.trim().toLowerCase().includes('accept all')
			);
			if (acceptBtn) {
				acceptBtn.click();
				return true;
			}
			return false;
		})()
	`
}

func currentISOWeek() string {
	year, week := time.Now().ISOWeek()
	return strconv.Itoa(year) + "W" + strconv.Itoa(week)
}

func writeJSON(output Output) {
	file, err := os.Create("public/data/lunches.json")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(output); err != nil {
		log.Fatal(err)
	}
}

// mondayOfWeek returns the Monday of the current ISO week.
func mondayOfWeek() time.Time {
	now := time.Now()
	offset := int(time.Monday - now.Weekday())
	if offset > 0 {
		offset -= 7
	}
	monday := now.AddDate(0, 0, offset)
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, monday.Location())
}

// weekdayToDate converts a Swedish day name (e.g. "MÅNDAG") to an ISO date
// string for the current week. Returns the Monday date for unknown/special days.
func weekdayToDate(day string) string {
	monday := mondayOfWeek()
	offsets := map[string]int{
		"MÅNDAG":  0,
		"TISDAG":  1,
		"ONSDAG":  2,
		"TORSDAG": 3,
		"FREDAG":  4,
		"LÖRDAG":  5,
		"SÖNDAG":  6,
	}
	if o, ok := offsets[strings.ToUpper(day)]; ok {
		return monday.AddDate(0, 0, o).Format("2006-01-02")
	}
	// For specials / unknown, return Monday (start of week)
	return monday.Format("2006-01-02")
}

// weekDateRange returns Monday and Friday ISO dates for the current week.
func weekDateRange() (string, string) {
	monday := mondayOfWeek()
	friday := monday.AddDate(0, 0, 4)
	return monday.Format("2006-01-02"), friday.Format("2006-01-02")
}
