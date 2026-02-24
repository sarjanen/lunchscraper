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

## Getting started

```sh
# Run the scraper (requires Chrome/Chromium)
go run ./cmd/scraper

# Dev server
bun install
bun run dev

# Production build
bun run build
```

## Adding a restaurant

1. Create `internal/scraper/<name>.go` implementing the `Scraper` interface
2. Register it in `cmd/scraper/main.go`
