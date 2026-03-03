package main

import "lunchscraper/internal/scraper"

// baseScrapers returns scrapers that compile without any extra build tags or
// CGO dependencies beyond the standard toolchain + chromedp.
func baseScrapers() []scraper.Scraper {
	return []scraper.Scraper{
		scraper.LaszloEbbepark{},
		scraper.LaLuna{},
		scraper.MonteCarlo{},
		scraper.Wardshuset{},
	}
}
