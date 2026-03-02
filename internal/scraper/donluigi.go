package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/otiai10/gosseract/v2"
)

// DonLuigi scrapes the lunch menu image from Don Luigi and uses OCR
// to extract the text.
type DonLuigi struct{}

func (d DonLuigi) Name() string {
	return "don_luigi"
}

func (d DonLuigi) Scrape(ctx context.Context) (RestaurantMenu, error) {
	url := "https://www.donluigi.se/lunchmeny/"

	var imageURL string

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),

		// Handle cookie consent if present
		chromedp.ActionFunc(func(ctx context.Context) error {
			var dismissed bool
			_ = chromedp.Evaluate(dismissCookieConsentJS(), &dismissed).Do(ctx)
			return nil
		}),

		chromedp.Sleep(1*time.Second),

		// The lunch menu is an image inside .entry-content.
		chromedp.Evaluate(`
			(() => {
				const content = document.querySelector('.entry-content');
				if (content) {
					const img = content.querySelector('img');
					if (img && img.src) return img.src;
				}
				const imgs = Array.from(document.querySelectorAll('img'));
				for (const img of imgs) {
					const src = (img.src || '').toLowerCase();
					if (src.includes('lunch') || src.includes('meny')) return img.src;
				}
				return '';
			})()
		`, &imageURL),
	)

	if err != nil {
		return RestaurantMenu{}, err
	}

	if imageURL == "" {
		return RestaurantMenu{}, fmt.Errorf("donluigi: could not find lunch menu image")
	}

	// Download the image
	imgData, err := downloadImage(imageURL)
	if err != nil {
		return RestaurantMenu{}, fmt.Errorf("donluigi: download image: %w", err)
	}

	// OCR the image with Tesseract (Swedish)
	ocrText, err := ocrImageBytes(imgData)
	if err != nil {
		return RestaurantMenu{}, fmt.Errorf("donluigi: OCR: %w", err)
	}

	items := parseDonLuigiMenu(ocrText)

	return RestaurantMenu{
		Restaurant: "Don Luigi",
		Location:   "Norrköping",
		MenuType:   "weekly",
		Week:       currentISOWeek(),
		Items:      items,
		Source:     url,
	}, nil
}

// downloadImage fetches image bytes from a URL.
func downloadImage(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// ocrImageBytes runs Tesseract OCR on raw image bytes using Swedish language.
func ocrImageBytes(imgData []byte) (string, error) {
	client := gosseract.NewClient()
	defer client.Close()

	// Use Swedish + English for mixed text
	if err := client.SetLanguage("swe", "eng"); err != nil {
		// Fall back to just English if Swedish data isn't available
		if err2 := client.SetLanguage("eng"); err2 != nil {
			return "", fmt.Errorf("set language: %w", err2)
		}
	}

	if err := client.SetImageFromBytes(imgData); err != nil {
		return "", fmt.Errorf("set image: %w", err)
	}

	text, err := client.Text()
	if err != nil {
		return "", fmt.Errorf("recognize: %w", err)
	}

	return text, nil
}

// parseDonLuigiMenu parses OCR text from the Don Luigi lunch menu image.
// The menu is typically a weekly menu with categories (Pinsa, Pasta, Sallad,
// Husman) rather than per-day dishes.
func parseDonLuigiMenu(raw string) []MenuItem {
	var cleaned []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 {
			cleaned = append(cleaned, trimmed)
		}
	}

	var items []MenuItem
	var currentCategory string
	var descLines []string
	var inMenuSection bool

	// Known dish category headers (case-insensitive prefix match)
	categories := []string{
		"PINSA", "PASTA", "SALLAD", "HUSMAN", "HUSMANSKOST",
		"RISOTTO", "FISK", "KÖTT", "SOPPA", "VECKANS",
	}

	skipPhrases := []string{
		"lunchmeny", "lunch meny", "mån-fre", "mån - fre",
		"det går bra", "vi bjuder", "allergisk", "after work",
		"inkl.", "take away", "extra", "hele menyn", "hela menyn",
		"don luigi", "bagarns val", "plocktallrik",
	}

	isCategory := func(line, upper string) (string, bool) {
		for _, cat := range categories {
			if !strings.HasPrefix(upper, cat) {
				continue
			}
			rest := strings.TrimSpace(line[len(cat):])
			// A category header line has either nothing after the name,
			// or a price (digits), or a dash/emdash. If the remainder
			// starts with a letter (like "i krämig…") it's a description
			// line that happens to begin with the category word.
			if len(rest) == 0 {
				return cat, true
			}
			first := rest[0]
			if first >= '0' && first <= '9' {
				return cat, true
			}
			if first == '-' || strings.HasPrefix(rest, "—") || strings.HasPrefix(rest, "–") {
				return cat, true
			}
			// Starts with a letter → treat as description, not header
			return "", false
		}
		return "", false
	}

	// flush saves the accumulated description lines as a MenuItem.
	flush := func() {
		if currentCategory != "" && len(descLines) > 0 {
			desc := strings.Join(descLines, " ")
			items = append(items, MenuItem{
				Date:        mondayOfWeek().Format("2006-01-02"),
				Name:        currentCategory,
				Description: desc,
			})
			descLines = nil
		}
	}

	for i := 0; i < len(cleaned); i++ {
		line := cleaned[i]
		upper := strings.ToUpper(line)
		lower := strings.ToLower(line)

		// Detect the section start (e.g. "LUNCHMENY v9")
		if strings.Contains(upper, "LUNCHMENY") {
			inMenuSection = true
			continue
		}

		// Detect category header (e.g. "Pinsa 124", "Pasta 124")
		if cat, ok := isCategory(line, upper); ok {
			flush() // save previous category's description
			currentCategory = strings.ToUpper(cat[:1]) + strings.ToLower(cat[1:])
			inMenuSection = true
			continue
		}

		if !inMenuSection {
			continue
		}

		// Skip boilerplate
		skip := false
		for _, phrase := range skipPhrases {
			if strings.Contains(lower, phrase) {
				skip = true
				break
			}
		}
		if skip {
			flush()
			currentCategory = ""
			continue
		}

		// Skip very short lines (noise) or price-only lines
		if len(line) < 5 {
			continue
		}
		// Skip lines that are only digits/price patterns
		stripped := strings.TrimRight(strings.TrimRight(lower, " "), "0123456789,.-:kr")
		if len(strings.TrimSpace(stripped)) < 3 {
			continue
		}

		// Accumulate description lines for the current category
		if currentCategory != "" {
			descLines = append(descLines, line)
		}
	}

	// Don't forget the last category
	flush()

	return items
}
