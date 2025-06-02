package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

func fetchPageHTML(page playwright.Page, parentCtx context.Context, pageURL string) (string, error) {
	opTimeout := 120 * time.Second
	ctx, cancel := context.WithTimeout(parentCtx, opTimeout)
	defer cancel()

	var htmlContent string
	logger.Printf("Fetching HTML for %s (using Playwright page: %p, closed: %t)", pageURL, page, page.IsClosed())

	type result struct {
		content string
		err     error
	}
	resultChan := make(chan result, 1)

	go func() {
		if page.IsClosed() {
			resultChan <- result{err: fmt.Errorf("playwright page for %s is already closed before navigation (Playwright connection issue)", pageURL)}
			return
		}
		browser := page.Context().Browser()
		if browser == nil || !browser.IsConnected() {
			resultChan <- result{err: fmt.Errorf("playwright browser for page %s is not connected (Playwright connection issue)", pageURL)}
			return
		}

		pwTimeoutMs := float64((opTimeout - 5*time.Second).Milliseconds())
		if pwTimeoutMs < 1000 {
			pwTimeoutMs = 1000
		}
		_, err := page.Goto(pageURL, playwright.PageGotoOptions{
			Timeout:   playwright.Float(pwTimeoutMs),
			WaitUntil: playwright.WaitUntilStateLoad,
		})

		if err != nil {
			if strings.Contains(err.Error(), "Target page, context or browser has been closed") || strings.Contains(err.Error(), "Target closed") {
				resultChan <- result{err: fmt.Errorf("playwright page.Goto failed for %s (Playwright connection issue): %w", pageURL, err)}
			} else {
				resultChan <- result{err: fmt.Errorf("playwright page.Goto failed for %s: %w", pageURL, err)}
			}
			return
		}

		if page.IsClosed() {
			resultChan <- result{err: fmt.Errorf("playwright page for %s closed after navigation (Playwright connection issue)", pageURL)}
			return
		}
		if browser := page.Context().Browser(); browser == nil || !browser.IsConnected() {
			resultChan <- result{err: fmt.Errorf("playwright browser for page %s disconnected after navigation (Playwright connection issue)", pageURL)}
			return
		}

		content, err := page.Content()
		if err != nil {
			if strings.Contains(err.Error(), "Target page, context or browser has been closed") || strings.Contains(err.Error(), "Target closed") {
				resultChan <- result{err: fmt.Errorf("playwright page.Content failed for %s (Playwright connection issue): %w", pageURL, err)}
			} else {
				resultChan <- result{err: fmt.Errorf("playwright page.Content failed for %s: %w", pageURL, err)}
			}
			return
		}
		resultChan <- result{content: content, err: nil}
	}()

	select {
	case <-ctx.Done():
		errReason := ctx.Err()
		if parentCtx.Err() == context.Canceled && errors.Is(errReason, context.Canceled) {
			return "", fmt.Errorf("parent context canceled during fetch of %s: %w", pageURL, parentCtx.Err())
		}
		return "", fmt.Errorf("playwright operation for %s %v (overall %s): %w", pageURL, errReason, opTimeout, errReason)
	case res := <-resultChan:
		if res.err != nil {
			return "", res.err
		}
		htmlContent = res.content
	}

	if strings.TrimSpace(htmlContent) == "" {
		return "", fmt.Errorf("fetched HTML content from %s is empty or whitespace", pageURL)
	}

	logger.Printf("Successfully fetched HTML from %s (length: %d)", pageURL, len(htmlContent))
	return htmlContent, nil
}
