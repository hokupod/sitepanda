package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"errors"
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
	wsURL           string
	pageLimit       int
	matchPatterns   []glob.Glob
	contentSelector string
	outfile         string
	silent          bool

	visited map[string]bool
	results []PageData
	rootCtx context.Context
	cancel  context.CancelFunc

	pw         *playwright.Playwright
	browser    playwright.Browser
	browserCtx playwright.BrowserContext
	page       playwright.Page
}

func NewCrawler(startURLStr string, wsURL string, pageLimit int, matchPatternsRaw stringSlice, contentSelector string, outfile string, silent bool) (*Crawler, error) {
	normStartURL, err := normalizeURLtoString(startURLStr)
	if err != nil {
		return nil, fmt.Errorf("invalid start URL '%s': %w", startURLStr, err)
	}

	parsedStartURL, err := url.Parse(normStartURL)
	if err != nil {
		return nil, fmt.Errorf("failed to re-parse normalized start URL '%s': %w", normStartURL, err)
	}
	if parsedStartURL.Scheme != "http" && parsedStartURL.Scheme != "https" {
		return nil, fmt.Errorf("start URL must use http or https scheme, got: %s", parsedStartURL.Scheme)
	}

	var compiledPatterns []glob.Glob
	if len(matchPatternsRaw) > 0 {
		for _, p := range matchPatternsRaw {
			g, err := glob.Compile(p)
			if err != nil {
				return nil, fmt.Errorf("invalid match pattern '%s': %w", p, err)
			}
			compiledPatterns = append(compiledPatterns, g)
		}
	}

	rootCtxForCrawler, rootCrawlerCancel := context.WithCancel(context.Background())

	pw, err := playwright.Run()
	if err != nil {
		rootCrawlerCancel()
		return nil, fmt.Errorf("could not start playwright: %w", err)
	}

	logger.Printf("Attempting to connect Playwright to Lightpanda browser at %s", wsURL)
	browser, err := pw.Chromium.ConnectOverCDP(wsURL, playwright.BrowserTypeConnectOverCDPOptions{
		Timeout: playwright.Float(30000),
	})
	if err != nil {
		_ = pw.Stop()
		rootCrawlerCancel()
		return nil, fmt.Errorf("playwright could not connect to browser over CDP at %s: %w", wsURL, err)
	}
	logger.Printf("Playwright successfully connected to browser at %s", wsURL)

	contexts := browser.Contexts()
	if len(contexts) == 0 {
		_ = browser.Close()
		_ = pw.Stop()
		rootCrawlerCancel()
		return nil, fmt.Errorf("no browser contexts found in Lightpanda after CDP connect; Lightpanda might not be fully initialized or is in an unexpected state")
	}
	browserCtx := contexts[0]
	logger.Printf("Using existing browser context from Lightpanda (Number of contexts: %d)", len(contexts))

	logger.Println("Creating a new page in the Lightpanda browser context...")
	page, err := browserCtx.NewPage()
	if err != nil {
		_ = browser.Close()
		_ = pw.Stop()
		rootCrawlerCancel()
		return nil, fmt.Errorf("failed to create new page in browser context: %w", err)
	}
	logger.Printf("Successfully created a new page.")

	logger.Println("Performing immediate checks on the newly created page...")
	if page == nil {
		_ = pw.Stop()
		rootCrawlerCancel()
		return nil, fmt.Errorf("playwright: newly created page object is nil")
	}
	if page.IsClosed() {
		_ = pw.Stop()
		rootCrawlerCancel()
		return nil, fmt.Errorf("playwright: newly created page is already closed (IsClosed() check)")
	}
	logger.Println("Newly created page is not nil and not reported as closed by IsClosed().")

	var initialPageURL string
	urlCheckCtx, urlCheckCancel := context.WithTimeout(rootCtxForCrawler, 5*time.Second)
	defer urlCheckCancel()
	err = func() error {
		done := make(chan error, 1)
		go func() {
			urlStr := page.URL()
			initialPageURL = urlStr
			done <- nil
		}()
		select {
		case <-urlCheckCtx.Done():
			return fmt.Errorf("timeout getting initial page URL: %w", urlCheckCtx.Err())
		case <-done:
			return nil
		}
	}()
	if err != nil {
		logger.Printf("Error getting URL of newly created page: %v", err)
		if strings.Contains(err.Error(), "target closed") || strings.Contains(err.Error(), "frame detached") {
			_ = pw.Stop()
			rootCrawlerCancel()
			return nil, fmt.Errorf("playwright: failed to get URL of new page, target likely closed: %w", err)
		}
	} else {
		logger.Printf("Newly created page URL: '%s'", initialPageURL)
	}

	if page.IsClosed() {
		_ = pw.Stop()
		rootCrawlerCancel()
		return nil, fmt.Errorf("playwright: page became closed after trying to get its URL")
	}
	logger.Println("Page still not reported as closed after URL check.")

	if initialPageURL == "about:blank" {
		logger.Println("Newly created page URL is already 'about:blank'. Skipping explicit Goto.")
	} else {
		logger.Printf("Attempting initial navigation from '%s' to about:blank with Playwright...", initialPageURL)
		_, err = page.Goto("about:blank", playwright.PageGotoOptions{
			Timeout: playwright.Float(15000),
		})
		if err != nil {
			_ = pw.Stop()
			rootCrawlerCancel()
			return nil, fmt.Errorf("playwright failed to navigate to about:blank: %w", err)
		}
		logger.Println("Successfully navigated to about:blank.")
	}

	titleCtx, titleCancel := context.WithTimeout(rootCtxForCrawler, 5*time.Second)
	defer titleCancel()
	var initialTitle string
	err = func() error {
		done := make(chan error, 1)
		go func() {
			title, titleErr := page.Title()
			if titleErr != nil {
				done <- titleErr
				return
			}
			initialTitle = title
			done <- nil
		}()
		select {
		case <-titleCtx.Done():
			return titleCtx.Err()
		case err := <-done:
			return err
		}
	}()
	if err != nil {
		_ = pw.Stop()
		rootCrawlerCancel()
		return nil, fmt.Errorf("playwright failed to get title of about:blank: %w", err)
	}
	logger.Printf("Playwright page is responsive (about:blank title: '%s')", initialTitle)

	crawler := &Crawler{
		startURL:        parsedStartURL,
		wsURL:           wsURL,
		pageLimit:       pageLimit,
		matchPatterns:   compiledPatterns,
		contentSelector: contentSelector,
		outfile:         outfile,
		silent:          silent,
		visited:         map[string]bool{normStartURL: true},
		results:         make([]PageData, 0),
		rootCtx:         rootCtxForCrawler,
		pw:              pw,
		browser:         browser,
		browserCtx:      browserCtx,
		page:            page,
	}

	crawler.cancel = func() {
		if crawler.page != nil && !crawler.page.IsClosed() {
			logger.Println("Crawler: closing Playwright page...")
			if err := crawler.page.Close(); err != nil {
				logger.Printf("Error closing Playwright page (might be okay if browser disconnected): %v", err)
			}
		}
		if crawler.browser != nil && crawler.browser.IsConnected() {
			logger.Println("Crawler: disconnecting Playwright browser (CDP)...")
			if err := crawler.browser.Close(); err != nil {
				logger.Printf("Error disconnecting Playwright browser: %v", err)
			}
		}
		if crawler.pw != nil {
			logger.Println("Crawler: stopping Playwright...")
			if err := crawler.pw.Stop(); err != nil {
				logger.Printf("Error stopping Playwright: %v", err)
			}
		}
		logger.Println("Crawler: canceling main root context...")
		rootCrawlerCancel()
	}

	return crawler, nil
}

func (c *Crawler) Crawl() error {
	defer c.cancel()

	normStartURLForQueue, _ := normalizeURLtoString(c.startURL.String())
	queue := []string{normStartURLForQueue}

	logger.Printf("Starting crawl with URL: %s (Normalized: %s)", c.startURL.String(), normStartURLForQueue)

	for len(queue) > 0 {
		currentURLStr := queue[0]
		queue = queue[1:]

		if c.pageLimit > 0 && len(c.results) >= c.pageLimit {
			logger.Printf("Page limit (%d) for saved content reached. Stopping crawl.", c.pageLimit)
			break
		}

		logger.Printf("Processing URL: %s (Queue size: %d, Results: %d)", currentURLStr, len(queue), len(c.results))

		currentURL, err := url.Parse(currentURLStr)
		if err != nil {
			logger.Printf("Warning: failed to re-parse normalized URL from queue %s: %v", currentURLStr, err)
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
			if isTimeout && attempt < maxRetries {
				logger.Printf("Retrying fetch for %s due to timeout...", currentURLStr)
				continue
			}
			break
		}

		if fetchErr != nil {
			isPlaywrightConnectionError := false
			errMsgFromFetch := fetchErr.Error()
			if strings.Contains(errMsgFromFetch, "browser has been closed") ||
				strings.Contains(errMsgFromFetch, "Target page, context or browser has been closed") ||
				strings.Contains(errMsgFromFetch, "Target closed") ||
				strings.Contains(errMsgFromFetch, "net::ERR_CONNECTION_REFUSED") ||
				strings.Contains(errMsgFromFetch, "(Playwright connection issue)") ||
				(c.browser != nil && !c.browser.IsConnected()) {
				isPlaywrightConnectionError = true
			}
			isRootContextDone := c.rootCtx.Err() != nil
			if isPlaywrightConnectionError || isRootContextDone {
				errMsg := "Critical error encountered: "
				if isPlaywrightConnectionError {
					errMsg += "Playwright connection to Lightpanda browser lost or Lightpanda unresponsive. "
				}
				if isRootContextDone && c.rootCtx.Err() != context.Canceled {
					errMsg += fmt.Sprintf("Root crawler context is done (%v). ", c.rootCtx.Err())
				} else if isRootContextDone {
					errMsg += "Root crawler context canceled. "
				}
				errMsg += "Stopping crawl."
				logger.Println(errMsg)
				return fmt.Errorf("%s Original error: %w", errMsg, fetchErr)
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
	path := pageURL.Path
	if path == "" {
		path = "/"
	}
	for _, g := range c.matchPatterns {
		if g.Match(path) {
			return true
		}
	}
	logger.Printf("URL %s (path: %s) did not match any patterns. Skipping content processing, but will still crawl for links.", pageURL.String(), path)
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
			logger.Printf("Warning: could not normalize link URL '%s' on page %s: %v", absoluteLinkURL.String(), pageURL.String(), err)
			return
		}
		if absoluteLinkURL.Scheme != "http" && absoluteLinkURL.Scheme != "https" {
			return
		}
		if absoluteLinkURL.Hostname() != c.startURL.Hostname() {
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

// normalizeURLtoString parses a URL string, removes its fragment, and returns the canonical string form.
// This helps in treating URLs like http://example.com and http://example.com/ as the same
// if they resolve identically after parsing and fragment removal.
func normalizeURLtoString(urlString string) (string, error) {
	trimmedURLString := strings.TrimSpace(urlString)
	if trimmedURLString == "" {
		return "", fmt.Errorf("input URL string is empty or only whitespace")
	}

	parsed, err := url.Parse(trimmedURLString)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL for normalization '%s': %w", trimmedURLString, err)
	}

	// Specifically, an input like "#fragment" results in Scheme="", Host="", Path="".
	if parsed.Scheme == "" && parsed.Host == "" && parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment != "" {
		// This was likely just a fragment string. After clearing it, the URL would be empty.
		return "", fmt.Errorf("input URL '%s' is effectively a fragment or invalid for normalization", trimmedURLString)
	}

	parsed.Fragment = ""

	// This makes "http://example.com" and "http://example.com/" normalize to the same string.
	if parsed.Scheme != "" && parsed.Host != "" && parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String(), nil
}

func formatResultsAsJSON(results []PageData) ([]byte, error) {
	if len(results) == 0 {
		// Return an empty JSON array if there are no results, to be consistent.
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
