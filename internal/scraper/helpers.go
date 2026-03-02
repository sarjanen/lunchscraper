package scraper

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

// dismissCookieConsentJS returns JS that clicks a Swedish cookie consent button.
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

// WriteJSON encodes output as pretty JSON and writes it to the given path,
// creating parent directories as needed.
func WriteJSON(output Output, path string) {
	dir := filepath.Dir(path)
	_ = os.MkdirAll(dir, os.ModePerm)

	file, err := os.Create(path)
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

// WriteSingleJSON writes a single RestaurantMenu as pretty JSON to the given path.
func WriteSingleJSON(menu RestaurantMenu, path string) {
	dir := filepath.Dir(path)
	_ = os.MkdirAll(dir, os.ModePerm)

	file, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(menu); err != nil {
		log.Fatal(err)
	}
}

// MergeJSON reads all *.json files in dir (excluding lunches.json), unmarshals
// each as a RestaurantMenu, wraps them in an Output, and writes to outputPath.
func MergeJSON(dir string, outputPath string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading directory %s: %w", dir, err)
	}

	var restaurants []RestaurantMenu
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") || name == "lunches.json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			log.Printf("WARN: skipping %s: %v", name, err)
			continue
		}

		var menu RestaurantMenu
		if err := json.Unmarshal(data, &menu); err != nil {
			log.Printf("WARN: skipping %s: %v", name, err)
			continue
		}

		if menu.Restaurant == "" {
			log.Printf("WARN: skipping %s: no restaurant name", name)
			continue
		}

		restaurants = append(restaurants, menu)
		log.Printf("Merged %s (%d items)", menu.Restaurant, len(menu.Items))
	}

	if len(restaurants) == 0 {
		return fmt.Errorf("no valid restaurant JSON files found in %s", dir)
	}

	output := Output{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Restaurants: restaurants,
	}

	WriteJSON(output, outputPath)
	return nil
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

// weekdayDates returns ISO date strings for Mon–Fri of the current week.
func weekdayDates() []string {
	monday := mondayOfWeek()
	dates := make([]string, 5)
	for i := 0; i < 5; i++ {
		dates[i] = monday.AddDate(0, 0, i).Format("2006-01-02")
	}
	return dates
}

// expandWeeklySpecials replicates weekly items (items whose Date equals the
// sentinel value) onto every weekday that is not in the closedDates set.
// This is useful when a restaurant has e.g. "Veckans fisk" available every
// open day. Pass sentinelDate = "" to match items with an empty date, or a
// specific date string used as a placeholder.
func expandWeeklySpecials(items []MenuItem, closedDates map[string]bool) []MenuItem {
	dates := weekdayDates()

	var daily []MenuItem
	var weekly []MenuItem

	for _, item := range items {
		if item.Closed {
			daily = append(daily, item)
			continue
		}
		// Items flagged with the "weekly" sentinel name prefix are expanded.
		upper := strings.ToUpper(item.Name)
		if strings.HasPrefix(upper, "VECKANS ") {
			weekly = append(weekly, item)
		} else {
			daily = append(daily, item)
		}
	}

	// Expand each weekly special onto every open weekday.
	for _, w := range weekly {
		for _, d := range dates {
			if closedDates[d] {
				continue
			}
			expanded := w
			expanded.Date = d
			daily = append(daily, expanded)
		}
	}

	return daily
}

// extractDishName tries to get just the dish name from a line like
// "Raggmunk med stekt rimmat fläsk samt lingon."
// by taking text up to the first "med", "serveras", or "samt".
func extractDishName(line string) string {
	lower := strings.ToLower(line)

	cutWords := []string{" med ", " serveras ", " samt ", " på tallrik", " toppad "}
	minIdx := len(line)
	for _, w := range cutWords {
		idx := strings.Index(lower, w)
		if idx > 0 && idx < minIdx {
			minIdx = idx
		}
	}

	if minIdx < len(line) && minIdx > 3 {
		return strings.TrimSpace(line[:minIdx])
	}

	if len(line) < 60 {
		return line
	}

	return strings.TrimSpace(line[:50]) + "..."
}
