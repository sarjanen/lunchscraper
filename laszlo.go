package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type LaszloEbbepark struct{}

func (l LaszloEbbepark) Name() string {
	return "laszlo_ebbepark"
}

func (l LaszloEbbepark) Scrape(ctx context.Context) (RestaurantMenu, error) {
	url := "https://www.laszloskrog.se/ebbepark/"

	var rawText string

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),

		// Handle cookie consent dialog
		chromedp.ActionFunc(func(ctx context.Context) error {
			var dismissed bool
			err := chromedp.Evaluate(`
				(() => {
					const buttons = Array.from(document.querySelectorAll("button, a"));
					const acceptBtn = buttons.find(b => 
						b.textContent.trim().toLowerCase().includes('godkänn alla')
					);
					if (acceptBtn) {
						acceptBtn.click();
						return true;
					}
					return false;
				})()
			`, &dismissed).Do(ctx)
			log.Printf("Cookie consent dismissed: %v", dismissed)
			return err
		}),

		chromedp.Sleep(1*time.Second),

		// Scroll to the menu section and click the "LUNCH" tab
		chromedp.ActionFunc(func(ctx context.Context) error {
			var clicked bool
			err := chromedp.Evaluate(`
				(() => {
					// Find the LUNCH tab title in the elementor tabs widget
					const tabTitles = Array.from(document.querySelectorAll(
						'.elementor-tab-title, [role="tab"], .e-n-tab-title'
					));
					const lunchTab = tabTitles.find(t => 
						t.textContent.trim().toUpperCase().includes('LUNCH') &&
						!t.textContent.trim().toUpperCase().includes('AFFÄRS')
					);
					if (lunchTab) {
						lunchTab.scrollIntoView({behavior: 'smooth', block: 'center'});
						lunchTab.click();
						return true;
					}
					// Fallback: click anchor links
					const links = Array.from(document.querySelectorAll("a"));
					const lunchLink = links.find(a => {
						const text = a.textContent.trim().toUpperCase();
						return text === 'LUNCH' || text === 'SE MENYN';
					});
					if (lunchLink) {
						lunchLink.scrollIntoView({behavior: 'smooth', block: 'center'});
						lunchLink.click();
						return true;
					}
					return false;
				})()
			`, &clicked).Do(ctx)
			if err != nil {
				return err
			}
			log.Printf("Lunch tab click successful: %v", clicked)
			return nil
		}),

		chromedp.Sleep(3*time.Second),

		// Extract the lunch menu content from the active tab
		chromedp.Evaluate(`
			(() => {
			  // Strategy 1: Look for active/visible elementor tab content
			  const tabContents = Array.from(document.querySelectorAll(
			    '.elementor-tab-content, .e-n-tabs-content > div, [role="tabpanel"]'
			  ));
			  for (const tab of tabContents) {
			    const text = tab.innerText || '';
			    if (text.toUpperCase().includes('VECKA') && text.length > 100) {
			      return text;
			    }
			  }
			  
			  // Strategy 2: Look for any element containing "VECKA"
			  const allElements = Array.from(document.querySelectorAll('div, section'));
			  for (const el of allElements) {
			    const text = el.innerText || '';
			    if (text.toUpperCase().includes('VECKA') && 
			        text.includes('Serveras') && 
			        text.length > 100 && text.length < 5000) {
			      return text;
			    }
			  }
			  
			  // Strategy 3: Get all visible text
			  return document.body.innerText;
			})()
		`, &rawText),
	)

	if err != nil {
		return RestaurantMenu{}, err
	}

	items := parseLaszloMenu(rawText)
	log.Printf("Scraped %d menu items", len(items))

	return RestaurantMenu{
		Restaurant: "Laszlo's Krog",
		Location:   "Ebbepark",
		MenuType:   "weekly",
		Week:       currentISOWeek(),
		Items:      items,
		Source:     url,
	}, nil
}

func parseLaszloMenu(raw string) []MenuItem {
	// Collect only non-empty trimmed lines
	var cleaned []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 {
			cleaned = append(cleaned, trimmed)
		}
	}

	var items []MenuItem
	var inMenuSection bool

	skipPhrases := []string{
		"till lunchen ingår",
		"ursprungsinformation",
		"har du matallergier",
		"fråga gärna",
		"köp lunchkort",
		"vardagar mellan",
		"ät i restaurangen",
		"avhämtning",
		"pensionärer",
	}

	for i := 0; i < len(cleaned); i++ {
		line := cleaned[i]
		lower := strings.ToLower(line)
		upper := strings.ToUpper(line)

		// Start capturing after the date line (e.g. "23-27 FEBRUARI")
		if containsSwedishMonth(upper) {
			inMenuSection = true
			continue
		}

		if !inMenuSection {
			continue
		}

		// Stop at non-lunch sections
		if strings.Contains(upper, "AFFÄRSLUNCH") || strings.Contains(upper, "À LA CARTE") || strings.Contains(upper, "Á LA CARTE") || strings.Contains(upper, "CATERING") || strings.Contains(upper, "URSPRUNGSINFORMATION") {
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
		if skip || strings.Contains(upper, "PDF") || strings.Contains(upper, "VECKA") {
			continue
		}

		// Dish name + description on next line
		name := line
		desc := ""
		if i+1 < len(cleaned) {
			next := cleaned[i+1]
			nextUpper := strings.ToUpper(next)
			if !strings.Contains(nextUpper, "URSPRUNGSINFORMATION") &&
				!strings.Contains(nextUpper, "PDF") &&
				!strings.Contains(nextUpper, "AFFÄRSLUNCH") {
				desc = next
				i++
			}
		}

		monDate, _ := weekDateRange()
		items = append(items, MenuItem{
			Date:        monDate,
			Name:        name,
			Description: desc,
		})
	}

	return items
}
