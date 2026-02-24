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
