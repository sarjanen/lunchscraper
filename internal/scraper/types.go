package scraper

import "context"

// Scraper is the interface each restaurant scraper must implement.
type Scraper interface {
	Name() string
	Scrape(ctx context.Context) (RestaurantMenu, error)
}

// RestaurantMenu holds the scraped menu for one restaurant.
type RestaurantMenu struct {
	Restaurant string     `json:"restaurant"`
	Location   string     `json:"location"`
	MenuType   string     `json:"menu_type"`
	Week       string     `json:"week"`
	Items      []MenuItem `json:"items"`
	ImageURL   string     `json:"image_url,omitempty"`
	Source     string     `json:"source"`
}

// MenuItem is a single dish on a menu.
type MenuItem struct {
	Date        string `json:"date"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Closed      bool   `json:"closed,omitempty"`
}

// Output is the top-level JSON structure written to disk.
type Output struct {
	GeneratedAt string           `json:"generated_at"`
	Restaurants []RestaurantMenu `json:"restaurants"`
}
