package scraper

import (
	"context"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// LaLuna scrapes the lunch menu from La Luna (Drabantgatan).
type LaLuna struct{}

func (l LaLuna) Name() string {
	return "la_luna"
}

func (l LaLuna) Scrape(ctx context.Context) (RestaurantMenu, error) {
	url := "https://lalunat1.se/lunch"

	var rawText string

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),

		// Handle cookie consent if present
		chromedp.ActionFunc(func(ctx context.Context) error {
			var dismissed bool
			_ = chromedp.Evaluate(dismissCookieConsentJS(), &dismissed).Do(ctx)
			return nil
		}),

		chromedp.Sleep(1*time.Second),

		// Extract the lunch menu text
		chromedp.Evaluate(`
			(() => {
			  // Look for the main content area containing the menu
			  const body = document.body.innerText || '';
			  return body;
			})()
		`, &rawText),
	)

	if err != nil {
		return RestaurantMenu{}, err
	}

	items := parseLaLunaMenu(rawText)

	return RestaurantMenu{
		Restaurant: "La Luna",
		Location:   "Drabantgatan",
		MenuType:   "daily",
		Week:       currentISOWeek(),
		Items:      items,
		Source:     url,
	}, nil
}

func parseLaLunaMenu(raw string) []MenuItem {
	var cleaned []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 {
			cleaned = append(cleaned, trimmed)
		}
	}

	var items []MenuItem
	var inMenuSection bool
	var currentDay string

	days := []string{"MÅNDAG", "TISDAG", "ONSDAG", "TORSDAG", "FREDAG", "LÖRDAG", "SÖNDAG"}
	specials := []string{"VECKANS SPECIAL", "VECKANS VEGETARISKA"}

	isDay := func(s string) (string, bool) {
		upper := strings.ToUpper(s)
		for _, d := range days {
			if upper == d {
				return d, true
			}
		}
		for _, sp := range specials {
			if strings.Contains(upper, sp) {
				return sp, true
			}
		}
		return "", false
	}

	skipPhrases := []string{
		"salladsbuffé",
		"meny vecka",
		"alla ordinarie",
		"pris vid",
		"lunchkupong",
		"mån-fre",
		"copyright",
		"la luna restaurang",
		"dagens lunch",
		"kvarterskrog",
	}

	for i := 0; i < len(cleaned); i++ {
		line := cleaned[i]
		lower := strings.ToLower(line)
		upper := strings.ToUpper(line)

		// Start after seeing "MENY VECKA" or first day
		if strings.Contains(upper, "MENY VECKA") {
			inMenuSection = true
			continue
		}

		// Check if this is a day header
		if day, ok := isDay(line); ok {
			inMenuSection = true
			currentDay = day
			continue
		}

		if !inMenuSection || currentDay == "" {
			continue
		}

		// Stop at footer
		if strings.Contains(lower, "copyright") {
			break
		}

		// Skip boilerplate
		skip := false
		for _, phrase := range skipPhrases {
			if strings.Contains(lower, phrase) {
				skip = true
				break
			}
		}
		if skip || len(line) < 5 {
			continue
		}

		items = append(items, MenuItem{
			Date:        weekdayToDate(currentDay),
			Name:        extractDishName(line),
			Description: line,
		})
	}

	return items
}
