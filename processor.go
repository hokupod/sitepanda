package main

import (
	"fmt"
	"net/url"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/PuerkitoBio/goquery"
	"github.com/go-shiori/go-readability"
)

// PageData represents extracted information from a webpage.
type PageData struct {
	Title       string
	URL         string
	Markdown    string
	RawHTML     string
	ArticleHTML string
}

func processHTML(pageURL string, rawHTML string, contentSelector string) (*PageData, error) {
	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse page URL %s: %w", pageURL, err)
	}

	htmlToProcess := rawHTML

	if contentSelector != "" {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
		if err != nil {
			logger.Printf("Warning: failed to parse HTML for content selector on %s: %v. Falling back to full page for readability.", pageURL, err)
		} else {
			selection := doc.Find(contentSelector).First()
			if selection.Length() > 0 {
				selectedHTML, err := goquery.OuterHtml(selection)
				if err != nil {
					logger.Printf("Warning: failed to get outer HTML for selector '%s' on %s: %v. Falling back to full page for readability.", contentSelector, pageURL, err)
				} else {
					logger.Printf("Successfully applied content selector '%s' on %s. Using selected HTML for readability.", contentSelector, pageURL)
					htmlToProcess = selectedHTML
				}
			} else {
				logger.Printf("Warning: content selector '%s' did not match any elements on %s. Falling back to full page for readability.", contentSelector, pageURL)
			}
		}
	} else {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
		if err != nil {
			logger.Printf("Warning: failed to parse HTML for pre-filtering on %s: %v. Proceeding with raw HTML for readability.", pageURL, err)
		} else {
			selectorsToRemove := []string{
				"script",
				"style",
				"link",
				"img",
				"video",
			}
			var removedElementsLog []string
			for _, selector := range selectorsToRemove {
				foundSelection := doc.Find(selector)
				if foundSelection.Length() > 0 {
					removedElementsLog = append(removedElementsLog, selector)
				}
				foundSelection.Remove()
			}

			modifiedHTML, err := goquery.OuterHtml(doc.Selection)
			if err != nil {
				logger.Printf("Warning: failed to get HTML after pre-filtering on %s: %v. Proceeding with raw HTML for readability.", pageURL, err)
			} else {
				if len(rawHTML) != len(modifiedHTML) && len(removedElementsLog) > 0 {
					htmlToProcess = modifiedHTML
					logger.Printf("Applied pre-filtering on %s (removed: %s). Using modified HTML for readability.", pageURL, strings.Join(removedElementsLog, ", "))
				} else if len(removedElementsLog) == 0 {
					logger.Printf("Pre-filtering attempted on %s, but no targeted elements (%s) were found or removed. Using raw HTML for readability.", pageURL, strings.Join(selectorsToRemove, ", "))
				} else {
					logger.Printf("Pre-filtering on %s: elements (%s) were targeted, but output HTML length is unchanged. Using raw HTML for readability.", pageURL, strings.Join(removedElementsLog, ", "))
				}
			}
		}
	}

	article, err := readability.FromReader(strings.NewReader(htmlToProcess), parsedURL)
	if err != nil {
  // Log this case: If a content selector was used and readability fails, the snippet may be too small or unsuitable.
		if contentSelector != "" && htmlToProcess != rawHTML {
			logger.Printf("Warning: failed to extract readable content from selector-reduced HTML for %s: %v. The selector might be too specific or the content unsuitable for readability.", pageURL, err)
		} else if contentSelector == "" && htmlToProcess != rawHTML {
			logger.Printf("Warning: failed to extract readable content from pre-filtered HTML for %s: %v.", pageURL, err)
		}
		return nil, fmt.Errorf("failed to extract readable content from %s: %w", pageURL, err)
	}

	converter := md.NewConverter("", true, nil)
	converter.Use(plugin.GitHubFlavored())

	markdownContent, err := converter.ConvertString(article.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to convert HTML to Markdown for %s: %w", pageURL, err)
	}

	pageData := &PageData{
		Title:       article.Title,
		URL:         pageURL,
		Markdown:    strings.TrimSpace(markdownContent),
		RawHTML:     rawHTML,
		ArticleHTML: article.Content,
	}

	logger.Printf("Successfully processed content for %s (Title: %s, Markdown length: %d)", pageURL, article.Title, len(pageData.Markdown))
	return pageData, nil
}

func formatPageDataAsXML(page *PageData) string {
	return fmt.Sprintf("<page>\n  <title>%s</title>\n  <url>%s</url>\n  <content>\n%s\n  </content>\n</page>",
		page.Title, page.URL, page.Markdown)
}
