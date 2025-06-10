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

type JSONOutputPage struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

type Crawler struct {
	startURL            *url.URL
	pageLimit           int
	matchPatterns       []glob.Glob
	followMatchPatterns []glob.Glob
	contentSelector     string
	outfile             string
	silent              bool
	waitForNetworkIdle  bool

	isURLListMode       bool
	initialURLs         []string

	visited map[string]bool
	results []PageData
	rootCtx context.Context
	cancel  context.CancelFunc

	pwBrowser playwright.Browser
	pwContext playwright.BrowserContext
	page      playwright.Page
}

func parseCrawlerArgs(startURLStr string, matchPatternsRaw []string, followMatchPatternsRaw []string) (*url.URL, []glob.Glob, []glob.Glob, error) {
	normStartURL, err := normalizeURLtoString(startURLStr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid start URL '%s': %w", startURLStr, err)
	}
	parsedStartURL, err := url.Parse(normStartURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to re-parse normalized start URL '%s': %w", normStartURL, err)
	}
	if parsedStartURL.Scheme != "http" && parsedStartURL.Scheme != "https" {
		return nil, nil, nil, fmt.Errorf("start URL must use http or https scheme, got: %s", parsedStartURL.Scheme)
	}

	var compiledMatchPatterns []glob.Glob
	if len(matchPatternsRaw) > 0 {
		for _, p := range matchPatternsRaw {
			g, compileErr := glob.Compile(p, '/')
			if compileErr != nil {
				return nil, nil, nil, fmt.Errorf("invalid match pattern '%s': %w", p, compileErr)
			}
			compiledMatchPatterns = append(compiledMatchPatterns, g)
		}
	}

	var compiledFollowMatchPatterns []glob.Glob
	if len(followMatchPatternsRaw) > 0 {
		for _, p := range followMatchPatternsRaw {
			g, compileErr := glob.Compile(p, '/')
			if compileErr != nil {
				return nil, nil, nil, fmt.Errorf("invalid follow-match pattern '%s': %w", p, compileErr)
			}
			compiledFollowMatchPatterns = append(compiledFollowMatchPatterns, g)
		}
	}
	return parsedStartURL, compiledMatchPatterns, compiledFollowMatchPatterns, nil
}

func newCrawlerCommon(
	parsedStartURL *url.URL,
	urlListToProcess []string,
	isListMode bool,
	pwB playwright.Browser,
	pageLimit int,
	compiledMatchPatterns []glob.Glob,
	compiledFollowMatchPatterns []glob.Glob,
	contentSelector string,
	outfile string,
	silent bool,
	waitForNetworkIdle bool,
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
		Timeout: playwright.Float(15000), // 15 seconds timeout for about:blank
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

	visitedMap := make(map[string]bool)

	crawler := &Crawler{
		startURL:            parsedStartURL,
		pageLimit:           pageLimit,
		matchPatterns:       compiledMatchPatterns,
		followMatchPatterns: compiledFollowMatchPatterns,
		contentSelector:     contentSelector,
		isURLListMode:       isListMode,
		initialURLs:         urlListToProcess,
		outfile:             outfile,
		silent:              silent,
		waitForNetworkIdle:  waitForNetworkIdle,
		visited:             visitedMap,
		results:             make([]PageData, 0),
		rootCtx:             rootContext,
		cancel:              rootCancelFunc,
		pwBrowser:           pwB,
		pwContext:           browserCtx,
		page:                p,
	}

	return crawler, nil
}

func NewCrawlerForLightpanda(
	startURLStr string,
	urlList []string,
	isListMode bool,
	wsURL string,
	pwInstance *playwright.Playwright,
	pageLimit int,
	matchPatternsRaw []string,
	followMatchPatternsRaw []string,
	contentSelector string,
	outfile string,
	silent bool,
	waitForNetworkIdle bool,
) (*Crawler, error) {
	parsedStartURL, compiledMatchPatterns, compiledFollowPatterns, err := parseCrawlerArgs(startURLStr, matchPatternsRaw, followMatchPatternsRaw)
	if err != nil {
		return nil, err
	}

	rootCtxForCrawler, rootCrawlerCancel := context.WithCancel(context.Background())

	logger.Printf("Attempting to connect Playwright to Lightpanda browser at %s", wsURL)
	browser, err := pwInstance.Chromium.ConnectOverCDP(wsURL, playwright.BrowserTypeConnectOverCDPOptions{
		Timeout: playwright.Float(30000), // 30 seconds timeout for connection
	})
	if err != nil {
		rootCrawlerCancel()
		return nil, fmt.Errorf("playwright could not connect to browser over CDP at %s: %w", wsURL, err)
	}
	logger.Printf("Playwright successfully connected to Lightpanda at %s", wsURL)

	return newCrawlerCommon(parsedStartURL, urlList, isListMode, browser, pageLimit, compiledMatchPatterns, compiledFollowPatterns, contentSelector, outfile, silent, waitForNetworkIdle, rootCtxForCrawler, rootCrawlerCancel)
}

func NewCrawlerForPlaywrightBrowser(
	startURLStr string,
	urlList []string,
	isListMode bool,
	pwB playwright.Browser,
	pageLimit int,
	matchPatternsRaw []string,
	followMatchPatternsRaw []string,
	contentSelector string,
	outfile string,
	silent bool,
	waitForNetworkIdle bool,
) (*Crawler, error) {
	parsedStartURL, compiledMatchPatterns, compiledFollowPatterns, err := parseCrawlerArgs(startURLStr, matchPatternsRaw, followMatchPatternsRaw)
	if err != nil {
		return nil, err
	}
	rootCtxForCrawler, rootCrawlerCancel := context.WithCancel(context.Background())
	return newCrawlerCommon(parsedStartURL, urlList, isListMode, pwB, pageLimit, compiledMatchPatterns, compiledFollowPatterns, contentSelector, outfile, silent, waitForNetworkIdle, rootCtxForCrawler, rootCrawlerCancel)
}

func (c *Crawler) Cancel() {
	if c.cancel != nil {
		c.cancel()
	}
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

	queue := []string{}

	if c.isURLListMode {
		logger.Printf("URL List Mode: Initializing queue with %d URLs from the provided list.", len(c.initialURLs))
		uniqueURLsForQueue := make(map[string]struct{})
		for _, urlStr := range c.initialURLs {
			normalizedURL, err := normalizeURLtoString(urlStr)
			if err != nil {
				logger.Printf("Warning: Skipping invalid URL from list '%s': %v", urlStr, err)
				continue
			}
			if _, exists := uniqueURLsForQueue[normalizedURL]; !exists {
				queue = append(queue, normalizedURL)
				uniqueURLsForQueue[normalizedURL] = struct{}{}
				c.visited[normalizedURL] = true
			}
		}
		logger.Printf("URL List Mode: Effective initial queue size after normalization and deduplication: %d", len(queue))
	} else {
		normStartURLForQueue, err := normalizeURLtoString(c.startURL.String())
		if err != nil {
			return fmt.Errorf("failed to normalize the initial start URL %s: %w", c.startURL.String(), err)
		}
		queue = append(queue, normStartURLForQueue)
		c.visited[normStartURLForQueue] = true
		logger.Printf("Single URL Mode: Initializing queue with start URL: %s", normStartURLForQueue)
	}

	if len(queue) == 0 {
		logger.Println("Initial crawl queue is empty. Nothing to process.")
		return nil
	}

	logger.Printf("Starting crawl. Initial queue size: %d. Start URL for context: %s", len(queue), c.startURL.String())

	for len(queue) > 0 {
		if c.rootCtx.Err() != nil {
			logger.Printf("Root context canceled. Stopping crawl and saving partial results. Error: %v", c.rootCtx.Err())
			break // Don't return error, continue to save partial results
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
				logger.Printf("Root context canceled before fetching %s, attempt %d. Stopping crawl to save partial results.", currentURLStr, attempt+1)
				fetchErr = c.rootCtx.Err()
				break
			}
			htmlContent, fetchErr = fetchPageHTML(c.page, c.rootCtx, currentURLStr, c.waitForNetworkIdle)
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
				if c.rootCtx.Err() != nil {
					logger.Printf("Root context done (%v), stopping crawl to save partial results. Original fetch error for %s: %v", c.rootCtx.Err(), currentURLStr, fetchErr)
				} else if c.pwBrowser != nil && !c.pwBrowser.IsConnected() {
					logger.Printf("Playwright browser disconnected. Stopping crawl to save partial results. Original fetch error for %s: %v", currentURLStr, fetchErr)
				} else {
					logger.Printf("Critical error encountered while fetching %s: %v. Stopping crawl to save partial results.", currentURLStr, fetchErr)
				}
				break // Don't return error, break to save partial results
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

		if !c.isURLListMode {
			if currentURL.Hostname() == c.startURL.Hostname() {
				links := c.extractAndFilterLinks(currentURL, htmlContent)
				for _, normalizedLinkStr := range links {
					if _, visited := c.visited[normalizedLinkStr]; !visited {
						if c.rootCtx.Err() != nil {
							logger.Printf("Root context canceled. Not adding more links to queue. Will save partial results.")
							break
						}
						c.visited[normalizedLinkStr] = true
						queue = append(queue, normalizedLinkStr)
						logger.Printf("Added to queue: %s", normalizedLinkStr)
					}
				}
			}
		}
	}

	if c.rootCtx.Err() != nil {
		logger.Printf("Crawl stopped due to cancellation. Total pages visited: %d. Partial results to save: %d", len(c.visited), len(c.results))
	} else {
		logger.Printf("Crawl finished. Total pages visited (dequeued for processing): %d. Total results saved: %d", len(c.visited), len(c.results))
	}

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
	} else if !strings.HasPrefix(pathToMatch, "/") {
		pathToMatch = "/" + pathToMatch
	}

	for _, g := range c.matchPatterns {
		if g.Match(pathToMatch) {
			return true
		}
	}
	logger.Printf("Path '%s' (from URL %s) did not match any --match patterns. Skipping content processing.", pathToMatch, pageURL.String())
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

		if len(c.followMatchPatterns) > 0 {
			shouldFollow := false
			pathToMatch := resolvedParsedURL.Path
			if pathToMatch == "" {
				pathToMatch = "/"
			} else if !strings.HasPrefix(pathToMatch, "/") {
				pathToMatch = "/" + pathToMatch
			}
			for _, g := range c.followMatchPatterns {
				if g.Match(pathToMatch) {
					shouldFollow = true
					break
				}
			}
			if !shouldFollow {
				return
			}
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
	if parsed.Scheme == "" && parsed.Host != "" {
		if !strings.Contains(parsed.Host, ":") && (strings.HasPrefix(trimmedURLString, "//") || !strings.ContainsAny(trimmedURLString, "/?#")) {
			parsedFromStringWithScheme := "http://" + trimmedURLString
			parsedWithScheme, errParseWithScheme := url.Parse(parsedFromStringWithScheme)
			if errParseWithScheme == nil {
				parsed = parsedWithScheme
			}
		}
	}
	if parsed.Scheme == "" && parsed.Host == "" {
	}

	parsed.Fragment = ""

	if parsed.Host != "" && parsed.Path == "" {
		parsed.Path = "/"
	}

	if len(parsed.Path) > 1 && strings.HasSuffix(parsed.Path, "/") {
		parsed.Path = parsed.Path[:len(parsed.Path)-1]
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
