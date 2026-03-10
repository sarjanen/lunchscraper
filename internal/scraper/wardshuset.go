package scraper

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// Wardshuset scrapes the lunch menu from Wärdshuset, Gamla Linköping (mlemd.se).
type Wardshuset struct{}

func (w Wardshuset) Name() string {
	return "wardshuset"
}

func (w Wardshuset) Scrape(ctx context.Context) (RestaurantMenu, error) {
	url := "https://mlemd.se/"

	var rawText string

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),

		// Wait for body to be ready (up to 15s, non-fatal)
		chromedp.ActionFunc(func(ctx context.Context) error {
			waitCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			_ = chromedp.WaitReady("body", chromedp.ByQuery).Do(waitCtx)
			return nil // non-fatal, keep going
		}),

		chromedp.Sleep(1*time.Second), // Give page time to start rendering

		// Wait for navigation links to render (up to 10s)
		chromedp.ActionFunc(func(ctx context.Context) error {
			waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			_ = chromedp.WaitVisible(`a, button`, chromedp.ByQuery).Do(waitCtx)
			return nil
		}),

		// Handle cookie consent if present
		chromedp.ActionFunc(func(ctx context.Context) error {
			var dismissed bool
			_ = chromedp.Evaluate(dismissCookieConsentJS(), &dismissed).Do(ctx)
			return nil
		}),

		chromedp.Sleep(500*time.Millisecond),

		// Click the LUNCHMENY nav link to scroll to the lunch section
		chromedp.ActionFunc(func(ctx context.Context) error {
			var clicked bool
			_ = chromedp.Evaluate(`
				(() => {
					const links = Array.from(document.querySelectorAll('a, button, [role="tab"]'));
					const lunchLink = links.find(el => {
						const text = el.textContent.trim().toUpperCase();
						return text === 'LUNCHMENY' || text === 'LUNCH';
					});
					if (lunchLink) {
						lunchLink.scrollIntoView({behavior: 'smooth', block: 'center'});
						lunchLink.click();
						return true;
					}
					return false;
				})()
			`, &clicked).Do(ctx)
			return nil
		}),

		chromedp.Sleep(1*time.Second),

		// Wait for lunch content with "VECKA" + day names to appear (up to 15s)
		chromedp.ActionFunc(func(ctx context.Context) error {
			waitCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			for {
				var found bool
				_ = chromedp.Evaluate(`
					(() => {
						const text = document.body.innerText.toUpperCase();
						return text.includes('VECKA') && text.includes('MÅNDAG');
					})()
				`, &found).Do(waitCtx)
				if found {
					return nil
				}
				select {
				case <-waitCtx.Done():
					return nil
				case <-time.After(500 * time.Millisecond):
				}
			}
		}),

		// Extract the lunch menu text from the page
		chromedp.Evaluate(`
			(() => {
				// The site is a single-page layout. Look for the lunch section
				// which contains "VECKA" and day names.
				const sections = Array.from(document.querySelectorAll('section, div'));
				for (const el of sections) {
					const text = el.innerText || '';
					const upper = text.toUpperCase();
					if (upper.includes('VECKA') &&
						upper.includes('MÅNDAG') &&
						upper.includes('FREDAG') &&
						text.length > 100 && text.length < 3000) {
						return text;
					}
				}
				// Fallback: return body text
				return document.body.innerText;
			})()
		`, &rawText),
	)

	if err != nil {
		return RestaurantMenu{}, err
	}

	items := parseWardshusetMenu(rawText)

	return RestaurantMenu{
		Restaurant: "Wärdshuset, Gamla Linköping",
		Location:   "Gamla Linköping",
		MenuType:   "daily",
		Week:       currentISOWeek(),
		Items:      items,
		Source:     url,
	}, nil
}

func parseWardshusetMenu(raw string) []MenuItem {
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
	closedDates := make(map[string]bool)

	days := map[string]bool{
		"MÅNDAG": true, "TISDAG": true, "ONSDAG": true,
		"TORSDAG": true, "FREDAG": true, "LÖRDAG": true, "SÖNDAG": true,
	}

	// Regex to detect price lines like "142:-", "150:-", "145:-"
	priceRe := regexp.MustCompile(`^\d+:-$`)

	for i := 0; i < len(cleaned); i++ {
		line := cleaned[i]
		upper := strings.ToUpper(line)

		// Start at the lunch section (after "Lunchmeny" or "VECKA")
		if strings.Contains(upper, "LUNCHMENY") || strings.Contains(upper, "VECKA ") {
			inMenuSection = true
			continue
		}

		// Stop at helgmeny / à la carte / other sections
		if strings.Contains(upper, "HELGMENY") ||
			strings.Contains(upper, "À LA CARTE") ||
			strings.Contains(upper, "A LA CARTE") ||
			strings.Contains(upper, "OM OSS") ||
			strings.Contains(upper, "UNDERHÅLLNING") {
			break
		}

		if !inMenuSection {
			continue
		}

		// Skip price lines
		if priceRe.MatchString(strings.TrimSpace(line)) {
			continue
		}

		// Skip separator lines
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "___") {
			continue
		}

		// Day headers
		if days[upper] {
			currentDay = upper
			continue
		}

		// Closed days — emit a closed marker and record the date
		if strings.Contains(upper, "STÄNGT") {
			date := weekdayToDate(currentDay)
			closedDates[date] = true
			items = append(items, MenuItem{
				Date:   date,
				Name:   "Stängt",
				Closed: true,
			})
			continue
		}

		// Handle "VECKANS FISK" and "VECKANS VEGETARISKA" as weekly specials.
		// They will be expanded to every open day by expandWeeklySpecials.
		if strings.Contains(upper, "VECKANS FISK") {
			var desc []string
			for j := i + 1; j < len(cleaned); j++ {
				next := cleaned[j]
				nextUpper := strings.ToUpper(next)
				if days[nextUpper] || strings.Contains(nextUpper, "VECKANS") ||
					strings.Contains(nextUpper, "HELGMENY") ||
					strings.HasPrefix(next, "---") || strings.HasPrefix(next, "___") {
					i = j - 1
					break
				}
				if priceRe.MatchString(strings.TrimSpace(next)) {
					continue
				}
				if len(next) > 3 {
					desc = append(desc, next)
				}
				if j == len(cleaned)-1 {
					i = j
				}
			}
			description := strings.Join(desc, ", ")
			items = append(items, MenuItem{
				Name:        "Veckans fisk",
				Description: description,
			})
			continue
		}

		if strings.Contains(upper, "VECKANS VEGETARISKA") {
			var desc []string
			for j := i + 1; j < len(cleaned); j++ {
				next := cleaned[j]
				nextUpper := strings.ToUpper(next)
				if days[nextUpper] || strings.Contains(nextUpper, "VECKANS") ||
					strings.Contains(nextUpper, "HELGMENY") ||
					strings.HasPrefix(next, "---") || strings.HasPrefix(next, "___") {
					i = j - 1
					break
				}
				if priceRe.MatchString(strings.TrimSpace(next)) {
					continue
				}
				if len(next) > 3 {
					desc = append(desc, next)
				}
				if j == len(cleaned)-1 {
					i = j
				}
			}
			description := strings.Join(desc, ", ")
			items = append(items, MenuItem{
				Name:        "Veckans vegetariska",
				Description: description,
			})
			continue
		}

		// Skip very short lines (likely separators or noise)
		if len(line) < 5 {
			continue
		}

		// Each line under a day is one dish (or an alternative).
		items = append(items, MenuItem{
			Date:        weekdayToDate(currentDay),
			Name:        extractDishName(line),
			Description: line,
		})
	}

	return items
}
