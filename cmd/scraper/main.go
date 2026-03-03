package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/chromedp/chromedp"

	"lunchscraper/internal/scraper"
)

func main() {
	site := flag.String("site", "", "Scrape a single site by name (e.g. don_luigi, la_luna, laszlo_ebbepark, monte_carlo, wardshuset)")
	merge := flag.Bool("merge", false, "Skip scraping; merge individual JSON files from web/public/data/ into lunches.json")
	configPath := flag.String("config", "restaurants.json", "Path to the restaurants.json config file (used by --merge for coordinate enrichment)")
	flag.Parse()

	// --merge mode: combine per-site JSON files into lunches.json
	if *merge {
		dataDir := "web/public/data"
		outputPath := "web/public/data/lunches.json"
		if err := scraper.MergeJSON(dataDir, outputPath, *configPath); err != nil {
			log.Fatalf("Merge failed: %v", err)
		}
		log.Printf("Merged individual JSONs into %s", outputPath)
		return
	}

	allScrapers := baseScrapers()
	allScrapers = append(allScrapers, extraScrapers()...)

	// Build the list of scrapers to run.
	var targets []scraper.Scraper
	if *site != "" {
		for _, s := range allScrapers {
			if s.Name() == *site {
				targets = append(targets, s)
				break
			}
		}
		if len(targets) == 0 {
			fmt.Fprintf(os.Stderr, "Unknown site %q. Available sites:\n", *site)
			for _, s := range allScrapers {
				fmt.Fprintf(os.Stderr, "  %s\n", s.Name())
			}
			os.Exit(1)
		}
	} else {
		targets = allScrapers
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	var results []scraper.RestaurantMenu

	var failed int
	for _, s := range targets {
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

	// --site mode: write individual JSON per restaurant
	if *site != "" {
		for _, menu := range results {
			path := fmt.Sprintf("web/public/data/%s.json", *site)
			scraper.WriteSingleJSON(menu, path)
			log.Printf("Wrote %s", path)
		}
		return
	}

	// Default: write combined lunches.json
	output := scraper.Output{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Restaurants: results,
	}

	scraper.WriteJSON(output, "web/public/data/lunches.json")
	log.Printf("Done: %d/%d restaurants scraped", len(results), len(results)+failed)
}
