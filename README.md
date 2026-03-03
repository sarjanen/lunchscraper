# Lunchscraper

Scrapes weekly lunch menus from restaurants in Linköping and displays them on a static site.

## Stack

- **Scraper** — Go + [chromedp](https://github.com/chromedp/chromedp) (headless Chrome)
- **Frontend** — Vanilla HTML/JS, built with [Vite](https://vitejs.dev)
- **Hosting** — GitHub Pages, deployed via GitHub Actions (weekdays 08:00 UTC)

## Project structure

```
cmd/scraper/          Go entry point
internal/scraper/     Scraper logic & restaurant parsers
web/                  Frontend (index.html)
```

## Prerequisites

- [Go](https://go.dev/) 1.25+
- [Bun](https://bun.sh/) (or Node.js)
- Chrome or Chromium (used headlessly by chromedp)
- **Tesseract OCR** — required by `gosseract` for image-based menu parsing:

```sh
brew install tesseract
```

## Getting started

```sh
# Install frontend dependencies
bun install

# Run the scraper (sets the required CGO flags for Tesseract/Leptonica automatically)
bun run scrape

# Dev server
bun run dev

# Production build
bun run build
```

> **Running the scraper without Bun:**
> If you want to call `go run` directly you need to export the CGO flags so the
> linker can find Tesseract and Leptonica:
>
> ```sh
> export CGO_CPPFLAGS="-I$(brew --prefix leptonica)/include -I$(brew --prefix tesseract)/include"
> export CGO_LDFLAGS="-L$(brew --prefix leptonica)/lib -L$(brew --prefix tesseract)/lib"
> go run ./cmd/scraper
> ```

## Adding a restaurant

1. Create `internal/scraper/<name>.go` implementing the `Scraper` interface
2. Register it in `cmd/scraper/main.go`
3. Add an entry to `restaurants.json`:

```json
{
  "key": "<scraper_name>",
  "name": "Human-Readable Name",
  "location": "Area / Street",
  "latitude": 0.0,
  "longitude": 0.0,
  "apt_packages": []
}
```

That's it — the CI workflow automatically picks up new entries from
`restaurants.json` and runs them as parallel matrix jobs. No workflow
YAML edits needed.

### `restaurants.json` schema

| Field          | Type       | Description                                                     |
|----------------|------------|-----------------------------------------------------------------|
| `key`          | `string`   | Must match the value returned by `Scraper.Name()` / `--site`    |
| `name`        | `string`   | Display name for the restaurant                                 |
| `location`    | `string`   | Area or street name                                             |
| `latitude`    | `number`   | GPS latitude (used for future geolocation features)             |
| `longitude`   | `number`   | GPS longitude                                                   |
| `apt_packages`| `string[]` | Extra apt packages to install in CI (e.g. `tesseract-ocr`)      |

### CI architecture

A single workflow (`.github/workflows/scrape-and-deploy.yml`) with three jobs:

1. **prepare** — reads `restaurants.json` and outputs a dynamic matrix
2. **scrape** — parallel matrix jobs (one per restaurant, `fail-fast: false`)
3. **deploy** — merges all per-site JSONs, builds the Vite site, deploys to GitHub Pages
