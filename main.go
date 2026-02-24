package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type Scraper interface {
	Name() string
	Scrape(ctx context.Context) (RestaurantMenu, error)
}

type RestaurantMenu struct {
	Restaurant string     `json:"restaurant"`
	Location   string     `json:"location"`
	Week       string     `json:"week"`
	Items      []MenuItem `json:"items"`
	Source     string     `json:"source"`
}

type MenuItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Output struct {
	GeneratedAt string           `json:"generated_at"`
	Restaurants []RestaurantMenu `json:"restaurants"`
}

/*
   =============================
   LASZLO EBEEPARK SCRAPER
   =============================
*/

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
			    // Look for the tab that contains week info (VECKA) and menu items
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

	log.Printf("Scraped %d menu items", len(items))

	items := parseMenuLines(rawText)

	return RestaurantMenu{
		Restaurant: "Laszlo's Krog",
		Location:   "Ebbepark",
		Week:       currentISOWeek(),
		Items:      items,
		Source:     url,
	}, nil
}

/*
   =============================
   MAIN
   =============================
*/

func main() {

	// Create output folder
	_ = os.MkdirAll("data", os.ModePerm)

	// Create Chrome context with headless mode and suppress logs
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

/*
   =============================
   HELPERS
   =============================
*/

func parseMenuLines(raw string) []MenuItem {
	// First, collect only non-empty trimmed lines
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
		if strings.Contains(upper, "FEBRUARI") || strings.Contains(upper, "MARS") || strings.Contains(upper, "APRIL") || strings.Contains(upper, "MAJ") || strings.Contains(upper, "JUNI") || strings.Contains(upper, "JULI") || strings.Contains(upper, "AUGUSTI") || strings.Contains(upper, "SEPTEMBER") || strings.Contains(upper, "OKTOBER") || strings.Contains(upper, "NOVEMBER") || strings.Contains(upper, "DECEMBER") || strings.Contains(upper, "JANUARI") {
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

		// Dish name is short, description is the next line
		name := line
		desc := ""
		if i+1 < len(cleaned) {
			next := cleaned[i+1]
			nextUpper := strings.ToUpper(next)
			// Only consume next line as description if it's not a stop/skip keyword
			if !strings.Contains(nextUpper, "URSPRUNGSINFORMATION") &&
				!strings.Contains(nextUpper, "PDF") &&
				!strings.Contains(nextUpper, "AFFÄRSLUNCH") {
				desc = next
				i++
			}
		}

		items = append(items, MenuItem{
			Name:        name,
			Description: desc,
		})
	}

	return items
}

func writeJSON(output Output) {
	file, err := os.Create("data/lunches.json")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(output); err != nil {
		log.Fatal(err)
	}
}

func currentISOWeek() string {
	year, week := time.Now().ISOWeek()
	return strings.Join([]string{
		strconv.Itoa(year),
		"W",
		strconv.Itoa(week),
	}, "")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}