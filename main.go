package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/chromedp/chromedp"
)

// Scraper is the interface each restaurant scraper must implement.
type Scraper interface {
	Name() string
	Scrape(ctx context.Context) (RestaurantMenu, error)
}

type RestaurantMenu struct {
	Restaurant  string     `json:"restaurant"`
	Location    string     `json:"location"`
	Week        string     `json:"week"`
	WeekDisplay string     `json:"week_display"`
	Items       []MenuItem `json:"items"`
	Source      string     `json:"source"`
}

type MenuItem struct {
	Day         string `json:"day"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Output struct {
	GeneratedAt string           `json:"generated_at"`
	Restaurants []RestaurantMenu `json:"restaurants"`
}

func main() {
	_ = os.MkdirAll("data", os.ModePerm)

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

	scrapers := []Scraper{
		LaszloEbbepark{},
		LaLuna{},
		MonteCarlo{},
		// Add new scrapers here
	}

	var results []RestaurantMenu

	for _, scraper := range scrapers {
		log.Println("Scraping:", scraper.Name())

		menu, err := scraper.Scrape(ctx)
		if err != nil {
			log.Println("Error scraping", scraper.Name(), ":", err)
			continue
		}

		results = append(results, menu)
	}

	output := Output{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Restaurants: results,
	}

	writeJSON(output)
	log.Println("Done.")
}
