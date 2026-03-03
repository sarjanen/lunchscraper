//go:build !ocr

package main

import "lunchscraper/internal/scraper"

// extraScrapers returns an empty list when the ocr build tag is not set.
func extraScrapers() []scraper.Scraper {
	return nil
}
