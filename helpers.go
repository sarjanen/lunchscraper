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

// currentWeekDisplay returns a display-friendly week string like "Vecka 9 (24-28 feb)"
func currentWeekDisplay() string {
	now := time.Now()
	_, week := now.ISOWeek()

	// Find Monday of the current week
	offset := int(time.Monday - now.Weekday())
	if offset > 0 {
		offset -= 7
	}
	monday := now.AddDate(0, 0, offset)
	friday := monday.AddDate(0, 0, 4)

	months := []string{"", "jan", "feb", "mar", "apr", "maj", "jun", "jul", "aug", "sep", "okt", "nov", "dec"}
	monthStr := months[monday.Month()]

	if monday.Month() == friday.Month() {
		return "Vecka " + strconv.Itoa(week) + " (" + strconv.Itoa(monday.Day()) + "-" + strconv.Itoa(friday.Day()) + " " + monthStr + ")"
	}
	return "Vecka " + strconv.Itoa(week) + " (" + strconv.Itoa(monday.Day()) + " " + monthStr + " - " + strconv.Itoa(friday.Day()) + " " + months[friday.Month()] + ")"
}

func writeJSON(output Output) {
	file, err := os.Create("data/lunches.json")
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
