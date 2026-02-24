package main

import (
	"context"
	"log"
	"time"

	"github.com/chromedp/chromedp"

	"lunchscraper/internal/scraper"
)

func main() {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	scrapers := []scraper.Scraper{
		scraper.LaszloEbbepark{},
		scraper.LaLuna{},
		scraper.MonteCarlo{},
		scraper.DonLuigi{},
		scraper.Wardshuset{},
	}

	var results []scraper.RestaurantMenu

	var failed int
	for _, s := range scrapers {
		var menu scraper.RestaurantMenu
		var err error

		// Try up to 2 attempts — CI runners can be slow to spin up Chrome.
		for attempt := 1; attempt <= 2; attempt++ {
			tabCtx, tabCancel := chromedp.NewContext(allocCtx,
				chromedp.WithLogf(func(string, ...interface{}) {}))
			tCtx, tCancel := context.WithTimeout(tabCtx, 60*time.Second)

			menu, err = s.Scrape(tCtx)
			tCancel()
			tabCancel()

			if err == nil {
				break
			}
			if attempt == 1 {
				log.Printf("RETRY %s (attempt 1 failed: %v)", s.Name(), err)
			}
		}

		if err != nil {
			log.Printf("FAIL  %s: %v", s.Name(), err)
			failed++
			continue
		}

		log.Printf("OK    %s — %d items", s.Name(), len(menu.Items))
		results = append(results, menu)
	}

	output := scraper.Output{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Restaurants: results,
	}

	scraper.WriteJSON(output, "web/public/data/lunches.json")
	log.Printf("Done: %d/%d restaurants scraped", len(results), len(results)+failed)
}
