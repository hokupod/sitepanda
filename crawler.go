package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gobwas/glob"
	"github.com/playwright-community/playwright-go"
)

// JSONOutputPage defines the structure for each page in the JSON output.
type JSONOutputPage struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// Crawler holds the state and configuration for a crawl session.
type Crawler struct {
	startURL        *url.URL
	pageLimit       int
	matchPatterns   []glob.Glob
	contentSelector string
	outfile         string
	silent          bool

	visited map[string]bool
	results []PageData
	rootCtx context.Context
	cancel  context.CancelFunc

	pwBrowser playwright.Browser
	pwContext playwright.BrowserContext
	page      playwright.Page
}

func parseCrawlerArgs(startURLStr string, matchPatternsRaw stringSlice) (*url.URL, []glob.Glob, error) {
	normStartURL, err := normalizeURLtoString(startURLStr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid start URL '%s': %w", startURLStr, err)
	}
	parsedStartURL, err := url.Parse(normStartURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to re-parse normalized start URL '%s': %w", normStartURL, err)
	}
	if parsedStartURL.Scheme != "http" && parsedStartURL.Scheme != "https" {
		return nil, nil, fmt.Errorf("start URL must use http or https scheme, got: %s", parsedStartURL.Scheme)
	}

	var compiledPatterns []glob.Glob
	if len(matchPatternsRaw) > 0 {
		for _, p := range matchPatternsRaw {
			g, compileErr := glob.Compile(p, '/')
			if compileErr != nil {
				return nil, nil, fmt.Errorf("invalid match pattern '%s': %w", p, compileErr)
			}
			compiledPatterns = append(compiledPatterns, g)
		}
	}
	return parsedStartURL, compiledPatterns, nil
}

func newCrawlerCommon(
	parsedStartURL *url.URL,
	pwB playwright.Browser,
	pageLimit int,
	compiledPatterns []glob.Glob,
	contentSelector string,
	outfile string,
	silent bool,
	rootContext context.Context,
	rootCancelFunc context.CancelFunc,
) (*Crawler, error) {

	var browserCtx playwright.BrowserContext
	var p playwright.Page
	var err error

	contexts := pwB.Contexts()
	if len(contexts) > 0 {
		browserCtx = contexts[0]
		logger.Printf("Using existing browser context from browser (Number of contexts: %d)", len(contexts))
	} else {
		browserCtx, err = pwB.NewContext()
		if err != nil {
			rootCancelFunc()
			return nil, fmt.Errorf("failed to create new browser context: %w", err)
		}
		logger.Println("Created new browser context.")
	}

	logger.Println("Creating a new page in the browser context...")
	p, err = browserCtx.NewPage()
	if err != nil {
		_ = browserCtx.Close()
		rootCancelFunc()
		return nil, fmt.Errorf("failed to create new page in browser context: %w", err)
	}
	logger.Printf("Successfully created a new page.")

	if p == nil {
		_ = browserCtx.Close()
		rootCancelFunc()
		return nil, fmt.Errorf("playwright: newly created page object is nil")
	}
	if p.IsClosed() {
		_ = browserCtx.Close()
		rootCancelFunc()
		return nil, fmt.Errorf("playwright: newly created page is already closed")
	}

	logger.Printf("Attempting initial navigation to about:blank with Playwright page...")
	_, err = p.Goto("about:blank", playwright.PageGotoOptions{
		Timeout: playwright.Float(15000),
	})
	if err != nil {
		_ = p.Close()
		_ = browserCtx.Close()
		rootCancelFunc()
		return nil, fmt.Errorf("playwright failed to navigate new page to about:blank: %w", err)
	}
	logger.Println("Successfully navigated new page to about:blank.")

	initialTitle, titleErr := p.Title()
	if titleErr != nil {
		_ = p.Close()
		_ = browserCtx.Close()
		rootCancelFunc()
		return nil, fmt.Errorf("playwright failed to get title of about:blank page: %w", titleErr)
	}
	logger.Printf("Playwright page is responsive (about:blank title: '%s')", initialTitle)

	normStartURLStr := parsedStartURL.String()

	crawler := &Crawler{
		startURL:        parsedStartURL,
		pageLimit:       pageLimit,
		matchPatterns:   compiledPatterns,
		contentSelector: contentSelector,
		outfile:         outfile,
		silent:          silent,
		visited:         map[string]bool{normStartURLStr: true},
		results:         make([]PageData, 0),
		rootCtx:         rootContext,
		cancel:          rootCancelFunc,
		pwBrowser:       pwB,
		pwContext:       browserCtx,
		page:            p,
	}

	return crawler, nil
}

func NewCrawlerForLightpanda(
	startURLStr string,
	wsURL string,
	pwInstance *playwright.Playwright,
	pageLimit int,
	matchPatternsRaw stringSlice,
	contentSelector string,
	outfile string,
	silent bool,
) (*Crawler, error) {
	parsedStartURL, compiledPatterns, err := parseCrawlerArgs(startURLStr, matchPatternsRaw)
	if err != nil {
		return nil, err
	}

	rootCtxForCrawler, rootCrawlerCancel := context.WithCancel(context.Background())

	logger.Printf("Attempting to connect Playwright to Lightpanda browser at %s", wsURL)
	browser, err := pwInstance.Chromium.ConnectOverCDP(wsURL, playwright.BrowserTypeConnectOverCDPOptions{
		Timeout: playwright.Float(30000),
	})
	if err != nil {
		rootCrawlerCancel()
		return nil, fmt.Errorf("playwright could not connect to browser over CDP at %s: %w", wsURL, err)
	}
	logger.Printf("Playwright successfully connected to Lightpanda at %s", wsURL)

	return newCrawlerCommon(parsedStartURL, browser, pageLimit, compiledPatterns, contentSelector, outfile, silent, rootCtxForCrawler, rootCrawlerCancel)
}

func NewCrawlerForPlaywrightBrowser(
	startURLStr string,
	pwB playwright.Browser,
	pageLimit int,
	matchPatternsRaw stringSlice,
	contentSelector string,
	outfile string,
	silent bool,
) (*Crawler, error) {
	parsedStartURL, compiledPatterns, err := parseCrawlerArgs(startURLStr, matchPatternsRaw)
	if err != nil {
		return nil, err
	}
	rootCtxForCrawler, rootCrawlerCancel := context.WithCancel(context.Background())
	return newCrawlerCommon(parsedStartURL, pwB, pageLimit, compiledPatterns, contentSelector, outfile, silent, rootCtxForCrawler, rootCrawlerCancel)
}

func (c *Crawler) Crawl() error {
	defer func() {
		if c.page != nil && !c.page.IsClosed() {
			logger.Println("Crawler: closing Playwright page...")
			if err := c.page.Close(); err != nil {
				logger.Printf("Error closing Playwright page: %v", err)
			}
		}
		if c.pwContext != nil {
			logger.Println("Crawler: closing Playwright browser context...")
			if err := c.pwContext.Close(); err != nil {
				logger.Printf("Error closing Playwright browser context: %v", err)
			}
		}
	}()

	normStartURLForQueue, _ := normalizeURLtoString(c.startURL.String())
	queue := []string{normStartURLForQueue}

	logger.Printf("Starting crawl with URL: %s", c.startURL.String())

	for len(queue) > 0 {
		if c.rootCtx.Err() != nil {
			logger.Printf("Root context canceled. Stopping crawl. Error: %v", c.rootCtx.Err())
			return c.rootCtx.Err()
		}

		currentURLStr := queue[0]
		queue = queue[1:]

		if c.pageLimit > 0 && len(c.results) >= c.pageLimit {
			logger.Printf("Page limit (%d) for saved content reached. Stopping crawl.", c.pageLimit)
			break
		}

		logger.Printf("Processing URL: %s (Queue size: %d, Results: %d)", currentURLStr, len(queue), len(c.results))

		currentURL, err := url.Parse(currentURLStr)
		if err != nil {
			logger.Printf("Warning: failed to re-parse normalized URL from queue %s: %v. Skipping.", currentURLStr, err)
			continue
		}

		var htmlContent string
		var fetchErr error
		const maxRetries = 1

		for attempt := 0; attempt <= maxRetries; attempt++ {
			if c.rootCtx.Err() != nil {
				logger.Printf("Root context canceled before fetching %s, attempt %d. Stopping fetch for this URL.", currentURLStr, attempt)
				fetchErr = c.rootCtx.Err()
				break
			}
			htmlContent, fetchErr = fetchPageHTML(c.page, c.rootCtx, currentURLStr)
			if fetchErr == nil {
				break
			}
			logger.Printf("Error fetching page %s (attempt %d/%d): %v", currentURLStr, attempt+1, maxRetries+1, fetchErr)
			isTimeout := errors.Is(fetchErr, context.DeadlineExceeded)
			isPlaywrightError := strings.Contains(fetchErr.Error(), "Playwright")
			if (isTimeout || isPlaywrightError) && attempt < maxRetries {
				logger.Printf("Retrying fetch for %s...", currentURLStr)
				continue
			}
			break
		}

		if fetchErr != nil {
			errMsgFromFetch := fetchErr.Error()
			isCriticalError := c.rootCtx.Err() != nil ||
				(c.pwBrowser != nil && !c.pwBrowser.IsConnected()) ||
				strings.Contains(errMsgFromFetch, "browser has been closed") ||
				strings.Contains(errMsgFromFetch, "Target page, context or browser has been closed") ||
				strings.Contains(errMsgFromFetch, "Target closed") ||
				strings.Contains(errMsgFromFetch, "net::ERR_CONNECTION_REFUSED")

			if isCriticalError {
				finalErrMsg := fmt.Sprintf("Critical error encountered while fetching %s: %v. Stopping crawl.", currentURLStr, fetchErr)
				if c.rootCtx.Err() != nil {
					finalErrMsg = fmt.Sprintf("Root context done (%v), implies critical error. Original fetch error for %s: %v. Stopping crawl.", c.rootCtx.Err(), currentURLStr, fetchErr)
				} else if c.pwBrowser != nil && !c.pwBrowser.IsConnected() {
					finalErrMsg = fmt.Sprintf("Playwright browser disconnected. Original fetch error for %s: %v. Stopping crawl.", currentURLStr, fetchErr)
				}
				logger.Println(finalErrMsg)
				return errors.New(finalErrMsg)
			}
			logger.Printf("Skipping page %s due to non-critical fetch error after retries: %v", currentURLStr, fetchErr)
			continue
		}

		if c.shouldProcessContent(currentURL) {
			pageData, processErr := processHTML(currentURLStr, htmlContent, c.contentSelector)
			if processErr != nil {
				logger.Printf("Error processing HTML for %s: %v", currentURLStr, processErr)
			} else {
				c.results = append(c.results, *pageData)
				logger.Printf("Content saved for %s. Total saved pages: %d", currentURLStr, len(c.results))
			}
		}

		if currentURL.Hostname() == c.startURL.Hostname() {
			links := c.extractAndFilterLinks(currentURL, htmlContent)
			for _, normalizedLinkStr := range links {
				if _, visited := c.visited[normalizedLinkStr]; !visited {
					if c.rootCtx.Err() != nil {
						logger.Printf("Root context canceled. Not adding more links to queue.")
						break
					}
					c.visited[normalizedLinkStr] = true
					queue = append(queue, normalizedLinkStr)
					logger.Printf("Added to queue: %s", normalizedLinkStr)
				}
			}
		}
	}

	logger.Printf("Crawl finished. Total pages visited (dequeued for processing): %d. Total results saved: %d", len(c.visited), len(c.results))

	if len(c.results) > 0 {
		if c.outfile != "" && strings.HasSuffix(strings.ToLower(c.outfile), ".json") {
			jsonData, err := formatResultsAsJSON(c.results)
			if err != nil {
				logger.Printf("Error marshalling results to JSON: %v", err)
			} else {
				err := os.WriteFile(c.outfile, jsonData, 0644)
				if err != nil {
					logger.Printf("Error writing JSON to outfile %s: %v", c.outfile, err)
				} else {
					logger.Printf("Successfully wrote %d pages to %s in JSON format", len(c.results), c.outfile)
				}
			}
		} else {
			var outputStrings []string
			for _, pd := range c.results {
				outputStrings = append(outputStrings, formatPageDataAsXML(&pd))
			}
			finalOutput := strings.Join(outputStrings, "\n\n")
			if c.outfile != "" {
				err := os.WriteFile(c.outfile, []byte(finalOutput), 0644)
				if err != nil {
					logger.Printf("Error writing to outfile %s: %v", c.outfile, err)
				} else {
					logger.Printf("Successfully wrote %d pages to %s", len(c.results), c.outfile)
				}
			} else {
				fmt.Println(finalOutput)
			}
		}
	} else {
		logger.Println("No results to output.")
	}
	return nil
}

func (c *Crawler) shouldProcessContent(pageURL *url.URL) bool {
	if len(c.matchPatterns) == 0 {
		return true
	}
	pathToMatch := pageURL.Path
	if pathToMatch == "" {
		pathToMatch = "/"
	}

	for _, g := range c.matchPatterns {
		if g.Match(pathToMatch) {
			return true
		}
	}
	logger.Printf("Path '%s' (from URL %s) did not match any patterns. Skipping content processing, but will still crawl for links if on same domain.", pathToMatch, pageURL.String())
	return false
}

func (c *Crawler) extractAndFilterLinks(pageURL *url.URL, htmlBody string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlBody))
	if err != nil {
		logger.Printf("Warning: failed to parse HTML for link extraction from %s: %v", pageURL.String(), err)
		return nil
	}

	uniqueLinks := make(map[string]struct{})
	var validLinks []string

	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		absoluteLinkURL, err := pageURL.Parse(href)
		if err != nil {
			logger.Printf("Warning: could not parse link '%s' on page %s: %v", href, pageURL.String(), err)
			return
		}

		normLinkStr, err := normalizeURLtoString(absoluteLinkURL.String())
		if err != nil {
			return
		}

		resolvedParsedURL, _ := url.Parse(normLinkStr)
		if resolvedParsedURL.Scheme != "http" && resolvedParsedURL.Scheme != "https" {
			return
		}
		if resolvedParsedURL.Hostname() != c.startURL.Hostname() {
			return
		}

		if _, found := uniqueLinks[normLinkStr]; found {
			return
		}
		uniqueLinks[normLinkStr] = struct{}{}
		validLinks = append(validLinks, normLinkStr)
	})
	return validLinks
}

func normalizeURLtoString(urlString string) (string, error) {
	trimmedURLString := strings.TrimSpace(urlString)
	if trimmedURLString == "" {
		return "", fmt.Errorf("input URL string is empty or only whitespace")
	}

	parsed, err := url.Parse(trimmedURLString)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL for normalization '%s': %w", trimmedURLString, err)
	}

	if parsed.Scheme == "" && parsed.Host == "" && parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment != "" {
		return "", fmt.Errorf("input URL '%s' is effectively only a fragment, cannot normalize", trimmedURLString)
	}
	if parsed.Scheme == "" && parsed.Host == "" {
	}

	parsed.Fragment = ""

	if len(parsed.Path) > 1 && strings.HasSuffix(parsed.Path, "/") {
		parsed.Path = parsed.Path[:len(parsed.Path)-1]
	}
	if parsed.Path == "" && parsed.Host != "" {
		parsed.Path = "/"
	}

	return parsed.String(), nil
}

func formatResultsAsJSON(results []PageData) ([]byte, error) {
	if len(results) == 0 {
		return []byte("[]"), nil
	}
	var jsonOutputPages []JSONOutputPage
	for _, pd := range results {
		jsonOutputPages = append(jsonOutputPages, JSONOutputPage{
			Title:   pd.Title,
			URL:     pd.URL,
			Content: pd.Markdown,
		})
	}
	return json.MarshalIndent(jsonOutputPages, "", "  ")
}
