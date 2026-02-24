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

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(func(string, ...interface{}) {}))
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	scrapers := []scraper.Scraper{
		scraper.LaszloEbbepark{},
		scraper.LaLuna{},
		scraper.MonteCarlo{},
	}

	var results []scraper.RestaurantMenu

	var failed int
	for _, s := range scrapers {
		menu, err := s.Scrape(ctx)
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
