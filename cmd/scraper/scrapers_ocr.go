//go:build ocr

package main

import "lunchscraper/internal/scraper"

// extraScrapers returns scrapers that require the "ocr" build tag
// (and the corresponding C libraries: Tesseract + Leptonica).
func extraScrapers() []scraper.Scraper {
	return []scraper.Scraper{
		scraper.DonLuigi{},
	}
}
