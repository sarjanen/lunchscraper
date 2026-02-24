package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type MonteCarlo struct{}

func (m MonteCarlo) Name() string {
	return "monte_carlo"
}

func (m MonteCarlo) Scrape(ctx context.Context) (RestaurantMenu, error) {
	// mcpizza.se is just an iframe to qopla.com — navigate directly
	url := "https://qopla.com/restaurant/restaurang-monte-carlo/qJNm7kQW9Z/home"

	var rawText string

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),

		// Handle cookie consent if present
		chromedp.ActionFunc(func(ctx context.Context) error {
			var dismissed bool
			_ = chromedp.Evaluate(dismissCookieConsentJS(), &dismissed).Do(ctx)
			log.Printf("Cookie consent dismissed: %v", dismissed)
			return nil
		}),

		chromedp.Sleep(1*time.Second),

		// Try to find and click a lunch menu link/section
		chromedp.ActionFunc(func(ctx context.Context) error {
			var clicked bool
			_ = chromedp.Evaluate(`
				(() => {
					const links = Array.from(document.querySelectorAll('a, button, [role="tab"], li, span, div'));
					const lunchLink = links.find(el => {
						const text = el.textContent.trim().toUpperCase();
						return text === 'LUNCH' || text === 'LUNCHMENY';
					});
					if (lunchLink) {
						lunchLink.scrollIntoView({behavior: 'smooth', block: 'center'});
						lunchLink.click();
						return true;
					}
					return false;
				})()
			`, &clicked).Do(ctx)
			log.Printf("Lunch link click: %v", clicked)
			return nil
		}),

		chromedp.Sleep(2*time.Second),

		// Extract page text
		chromedp.Evaluate(`document.body.innerText`, &rawText),
	)

	if err != nil {
		return RestaurantMenu{}, err
	}

	items := parseMonteCarloMenu(rawText)
	log.Printf("Scraped %d menu items", len(items))

	return RestaurantMenu{
		Restaurant:  "Pizzeria Monte Carlo",
		Location:    "Parkgatan",
		MenuType:    "daily",
		Week:        currentISOWeek(),
		WeekDisplay: currentWeekDisplay(),
		Items:       items,
		Source:      url,
	}, nil
}

func parseMonteCarloMenu(raw string) []MenuItem {
	var cleaned []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 {
			cleaned = append(cleaned, trimmed)
		}
	}

	var items []MenuItem
	var inLunchSection bool
	var currentDay string

	days := map[string]bool{
		"MÅNDAG": true, "TISDAG": true, "ONSDAG": true,
		"TORSDAG": true, "FREDAG": true, "LÖRDAG": true, "SÖNDAG": true,
	}

	for i := 0; i < len(cleaned); i++ {
		line := cleaned[i]
		upper := strings.ToUpper(line)

		// Start at lunch section
		if strings.Contains(upper, "LUNCH") {
			inLunchSection = true
			continue
		}

		// Stop at pizza/other sections
		if strings.Contains(upper, "PIZZOR") || strings.Contains(upper, "KEBAB") ||
			strings.Contains(upper, "HAMBURGARE") || strings.Contains(upper, "SALLADER") {
			break
		}

		if !inLunchSection {
			continue
		}

		// Day headers
		if days[upper] {
			currentDay = upper
			continue
		}

		// Skip price lines like "99 kr"
		if strings.HasSuffix(strings.ToLower(line), "kr") || strings.Contains(line, " kr") {
			continue
		}

		// Skip short/empty lines
		if len(line) < 5 {
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
