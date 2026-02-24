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
		// Each scraper gets its own browser tab and timeout so one
		// slow scrape can't steal the time budget from the next one.
		tabCtx, tabCancel := chromedp.NewContext(allocCtx,
			chromedp.WithLogf(func(string, ...interface{}) {}))
		tCtx, tCancel := context.WithTimeout(tabCtx, 45*time.Second)

		menu, err := s.Scrape(tCtx)
		tCancel()
		tabCancel()

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
