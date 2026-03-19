package scraper

import (
	"context"
	"encoding/json"
	"log"
	"regexp"
	"strconv"
	"time"

	"github.com/chromedp/chromedp"
)

// LaszloEbbepark scrapes the lunch menu from Laszlo's Krog (Ebbepark).
type LaszloEbbepark struct{}

func (l LaszloEbbepark) Name() string {
	return "laszlo_ebbepark"
}

// laszloSection represents one week-section extracted from the DOM.
type laszloSection struct {
	WeekClass string       `json:"weekClass"`
	DateRange string       `json:"dateRange"`
	Items     []laszloItem `json:"items"`
}

// laszloItem represents a single dish extracted from the DOM.
type laszloItem struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (l LaszloEbbepark) Scrape(ctx context.Context) (RestaurantMenu, error) {
	url := "https://www.laszloskrog.se/ebbepark/"

	var sectionsJSON string

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),

		// Wait for body to be ready (up to 15s, non-fatal)
		chromedp.ActionFunc(func(ctx context.Context) error {
			waitCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			_ = chromedp.WaitReady("body", chromedp.ByQuery).Do(waitCtx)
			return nil
		}),

		chromedp.Sleep(1*time.Second),

		// Wait for the Elementor tab widget to render
		chromedp.ActionFunc(func(ctx context.Context) error {
			waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			_ = chromedp.WaitVisible(`.elementor-tab-title, [role="tab"], .e-n-tab-title`, chromedp.ByQuery).Do(waitCtx)
			return nil
		}),

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
			return err
		}),

		chromedp.Sleep(500*time.Millisecond),

		// Scroll to the menu section and click the "LUNCH" tab
		chromedp.ActionFunc(func(ctx context.Context) error {
			var clicked bool
			err := chromedp.Evaluate(`
				(() => {
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
			return err
		}),

		chromedp.Sleep(1*time.Second),

		// Wait for fdm-section elements to appear (up to 10s)
		chromedp.ActionFunc(func(ctx context.Context) error {
			waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			_ = chromedp.WaitVisible(`ul.fdm-section`, chromedp.ByQuery).Do(waitCtx)
			return nil
		}),

		// Extract structured menu data — only from lunch sections (those with a vecka class)
		chromedp.Evaluate(`
			(() => {
				const sections = document.querySelectorAll('ul.fdm-section[class*="fdm-section-vecka-"]');
				const result = [];
				for (const section of sections) {
					const classList = Array.from(section.classList);
					const weekClass = classList.find(c => /^fdm-section-vecka-\d+-\d+$/.test(c)) || '';
					if (!weekClass) continue;
					const header = section.querySelector('.fdm-section-header h3');
					const dateRange = header ? header.textContent.trim() : '';
					const items = [];
					const itemEls = section.querySelectorAll('.fdm-item');
					for (const item of itemEls) {
						const titleEl = item.querySelector('.fdm-item-title');
						const contentEl = item.querySelector('.fdm-item-content');
						items.push({
							title: titleEl ? titleEl.textContent.trim() : '',
							content: contentEl ? contentEl.textContent.trim() : ''
						});
					}
					result.push({ weekClass, dateRange, items });
				}
				return JSON.stringify(result);
			})()
		`, &sectionsJSON),
	)

	if err != nil {
		return RestaurantMenu{}, err
	}

	items := parseLaszloSections(sectionsJSON)

	return RestaurantMenu{
		Restaurant: "Laszlo's Krog",
		Location:   "Ebbepark",
		MenuType:   "weekly",
		Week:       currentISOWeek(),
		Items:      items,
		Source:     url,
	}, nil
}

// weekClassRe matches "fdm-section-vecka-13-2026" -> week=13, year=2026
var weekClassRe = regexp.MustCompile(`^fdm-section-vecka-(\d+)-(\d+)$`)

func parseLaszloSections(sectionsJSON string) []MenuItem {
	var sections []laszloSection
	if err := json.Unmarshal([]byte(sectionsJSON), &sections); err != nil {
		log.Printf("laszlo: failed to parse sections JSON: %v", err)
		return nil
	}

	_, currentWeekNum := time.Now().ISOWeek()

	var items []MenuItem
	for _, sec := range sections {
		// Match on week number only — the year in the class can be wrong
		// (e.g. "fdm-section-vecka-12-2023" when it should be 2026).
		sectionWeekNum := laszloSectionWeekNum(sec.WeekClass)
		if sectionWeekNum == 0 || sectionWeekNum != currentWeekNum {
			continue
		}

		monDate, _ := weekDateRange()
		for _, dish := range sec.Items {
			if dish.Title == "" {
				continue
			}
			items = append(items, MenuItem{
				Date:        monDate,
				Name:        dish.Title,
				Description: dish.Content,
			})
		}
	}

	return items
}

// laszloSectionWeekNum extracts the week number from a class like
// "fdm-section-vecka-13-2026" -> 13. Returns 0 if not matched.
// We ignore the year because the site sometimes has incorrect years in the class.
func laszloSectionWeekNum(class string) int {
	m := weekClassRe.FindStringSubmatch(class)
	if m == nil {
		return 0
	}
	week, _ := strconv.Atoi(m[1])
	return week
}
