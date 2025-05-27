package main

import (
	"strings"
	"testing"
)

func TestProcessHTML(t *testing.T) {
	const commonStyle = "<style>body { font-family: sans-serif; }</style>"
	const commonScript = "<script>console.log('test');</script>"
	const commonNav = "<nav><a href='/home'>Home</a></nav>"
	const commonFooter = "<footer><p>&copy; 2025 Test Inc.</p></footer>"
	const commonImg = "<img src='test.jpg' alt='Test Image'>"
	const commonVideo = "<video><source src='test.mp4' type='video/mp4'></video>"
	const commonLink = "<link rel='stylesheet' href='style.css'>"

	tests := []struct {
		name              string
		pageURL           string
		rawHTML           string
		contentSelector   string
		wantTitle         string
		wantMarkdown      string
		wantArticleHTML   string
		expectError       bool
		checkRawHTML      bool
		checkArticleHTML  bool
		preFilteringCheck func(t *testing.T, pd *PageData, originalRawHTML string)
		selectorCheck     func(t *testing.T, pd *PageData)
	}{
		{
			name:    "Simple content, no selector (pre-filtering applies)",
			pageURL: "http://example.com/simple",
			rawHTML: `<html><head><title>Simple Page</title>` + commonStyle + commonScript + commonLink + `</head><body>` +
				commonNav + commonImg + commonVideo +
				`<h1>Main Title</h1><p>This is simple content.</p>` +
				commonFooter + `</body></html>`,
			contentSelector: "",
			wantTitle:       "Simple Page", // Readability picks <title>
			wantMarkdown:    "Main Title",
			wantArticleHTML: "<h2>Main Title</h2><p>This is simple content.</p>", // go-readability often makes main title H2
			expectError:     false,
			checkRawHTML:    true,
			preFilteringCheck: func(t *testing.T, pd *PageData, originalRawHTML string) {
				if pd.RawHTML != originalRawHTML {
					t.Errorf("RawHTML was modified, expected original")
				}
				// Readability itself might also remove some of these, but pre-filtering ensures it.
				htmlAfterReadability := pd.ArticleHTML
				if strings.Contains(htmlAfterReadability, "<script") ||
					strings.Contains(htmlAfterReadability, "<style") ||
					strings.Contains(htmlAfterReadability, "<img src='test.jpg'") ||
					strings.Contains(htmlAfterReadability, "<video") ||
					strings.Contains(htmlAfterReadability, "<link rel='stylesheet' href='style.css'>") {
					t.Errorf("ArticleHTML seems to contain pre-filtered elements: %s", htmlAfterReadability)
				}
				if !strings.Contains(htmlAfterReadability, "Main Title") {
					t.Errorf("ArticleHTML does not contain expected 'Main Title': %s", htmlAfterReadability)
				}
			},
		},
		{
			name:    "With content selector, selector matches",
			pageURL: "http://example.com/selector",
			rawHTML: `<html><head><title>Selector Test</title></head><body>
                <div class="ignored">Ignored text. ` + commonScript + `</div>
                <article class="main-content">
                    <h2>Article Title</h2>
                    <p>Selected content here. <img src="inline.jpg" alt="Inline"></p>
                </article>
                <div class="ignored-after">More ignored text.</div>
            </body></html>`,
			contentSelector: ".main-content",
			// Title is tricky with snippets. Readability might not find "Article Title" as the main title for the snippet.
			// It might use the page's original title if available, or derive nothing.
			// Actual behavior: readability.FromReader uses the parsedURL to fetch the original page for metadata if the snippet is partial.
			// Since our snippet is from rawHTML, it might not have enough context for title.
			// When processing a snippet, readability might not find a page title.
			wantTitle:       "",
			wantMarkdown:    "Article Title",
			// Also, image URL will be resolved by readability.
			wantArticleHTML: "<h2>Article Title</h2><p>Selected content here. <img src=\"http://example.com/inline.jpg\" alt=\"Inline\"></p>",
			expectError:     false,
			checkRawHTML:    true,
			selectorCheck: func(t *testing.T, pd *PageData) {
				if strings.Contains(pd.ArticleHTML, "Ignored text") {
					t.Errorf("ArticleHTML contains ignored text from outside selector: %s", pd.ArticleHTML)
				}
				if !strings.Contains(pd.ArticleHTML, "src=\"http://example.com/inline.jpg\"") {
					t.Errorf("ArticleHTML does not contain expected absolute image URL from within selector: %s", pd.ArticleHTML)
				}
				if !strings.Contains(pd.ArticleHTML, "Article Title") {
					t.Errorf("ArticleHTML does not contain 'Article Title' from selector: %s", pd.ArticleHTML)
				}
			},
		},
		{
			name:    "With content selector, selector does NOT match (fallback to full rawHTML, no pre-filtering)",
			pageURL: "http://example.com/selector-miss",
			rawHTML: `<html><head><title>Selector Miss</title>` + commonScript + `</head><body>
                <div class="actual-content">
                    <h1>Page Header</h1>
                    <p>Some text. <img src="important.jpg"></p>
                </div>
            </body></html>`,
			contentSelector: ".non-existent-selector",
			wantTitle:       "Selector Miss", // Readability processes full page, gets <title>
			wantMarkdown:    "Page Header",
			wantArticleHTML: "<h2>Page Header</h2><p>Some text. <img src=\"http://example.com/important.jpg\"></p>", // go-readability often makes main title H2
			expectError:     false,
			checkRawHTML:    true,
			selectorCheck: func(t *testing.T, pd *PageData) {
				// Pre-filtering is skipped on selector miss. Readability processes rawHTML.
				// Readability itself will strip script tags.
				if !strings.Contains(pd.ArticleHTML, "src=\"http://example.com/important.jpg\"") {
					t.Errorf("ArticleHTML does not contain 'important.jpg', expected due to selector miss fallback (no pre-filter, readability resolves URL): %s", pd.ArticleHTML)
				}
				if !strings.Contains(pd.ArticleHTML, "Page Header") {
					t.Errorf("ArticleHTML does not contain 'Page Header' from full page: %s", pd.ArticleHTML)
				}
			},
		},
		{
			name:            "Readability with empty rawHTML",
			pageURL:         "http://example.com/fail",
			rawHTML:         ``,
			contentSelector: "",
			wantTitle:       "",
			wantMarkdown:    "",
			wantArticleHTML: "",
			expectError:     false, // readability.FromReader on empty string doesn't error, returns empty Article
			checkRawHTML:    true,
		},
		{
			name:            "Content that results in some markdown after readability (nav link)",
			pageURL:         "http://example.com/emptyish",
			rawHTML:         `<html><head><title>Emptyish</title></head><body>` + commonNav + commonFooter + `</body></html>`,
			contentSelector: "",
			wantTitle:       "Emptyish",
			wantMarkdown:    "[Home](http://example.com/home)", // Pre-filter doesn't remove nav, readability might pick up the link
			wantArticleHTML: "<a href=\"http://example.com/home\">Home</a>",
			expectError:     false,
			checkRawHTML:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: processHTML uses a global logger. For more isolated tests,
			// the logger could be injected. For now, we accept global logger usage.

			pageData, err := processHTML(tt.pageURL, tt.rawHTML, tt.contentSelector)

			if (err != nil) != tt.expectError {
				t.Fatalf("processHTML() error = %v, wantErr %v", err, tt.expectError)
			}
			if tt.expectError {
				return
			}

			if pageData == nil {
				t.Fatalf("processHTML() returned nil PageData unexpectedly")
			}

			if pageData.URL != tt.pageURL {
				t.Errorf("PageData.URL got = %q, want %q", pageData.URL, tt.pageURL)
			}

			if tt.checkRawHTML && pageData.RawHTML != tt.rawHTML {
				t.Errorf("PageData.RawHTML got = %q, want %q", pageData.RawHTML, tt.rawHTML)
			}

			// Readability might find a different title or no title if content is sparse
			if tt.wantTitle != "" && pageData.Title != tt.wantTitle {
				// Allow for cases where readability might prepend site name if available,
				// or if the title tag was inside pre-filtered content.
				if !strings.Contains(pageData.Title, tt.wantTitle) {
					t.Errorf("PageData.Title got = %q, want (or contains) %q", pageData.Title, tt.wantTitle)
				}
			}

			if tt.wantMarkdown != "" && !strings.Contains(pageData.Markdown, tt.wantMarkdown) {
				t.Errorf("PageData.Markdown got = %q, want to contain %q", pageData.Markdown, tt.wantMarkdown)
			} else if tt.wantMarkdown == "" && strings.TrimSpace(pageData.Markdown) != "" {
				t.Errorf("PageData.Markdown got = %q, want empty string", pageData.Markdown)
			}

			if tt.wantArticleHTML != "" {
				if strings.Contains(tt.wantArticleHTML, "<h2>") && !strings.Contains(pageData.ArticleHTML, "<h2>") {
					if !strings.Contains(pageData.ArticleHTML, "<h1>") {
						t.Errorf("PageData.ArticleHTML missing expected H2 (or H1) structure. Got: %q, Expected part: %q", pageData.ArticleHTML, tt.wantArticleHTML)
					}
				}
				if strings.Contains(tt.wantArticleHTML, "<p>") && !strings.Contains(pageData.ArticleHTML, "<p>") {
					t.Errorf("PageData.ArticleHTML missing expected P structure. Got: %q, Expected part: %q", pageData.ArticleHTML, tt.wantArticleHTML)
				}
				if strings.Contains(tt.wantArticleHTML, "<img src=") && !strings.Contains(pageData.ArticleHTML, "<img src=") {
					t.Errorf("PageData.ArticleHTML missing expected IMG structure. Got: %q, Expected part: %q", pageData.ArticleHTML, tt.wantArticleHTML)
				}
				// This is a simplified check; more complex HTML might need smarter stripping.
				var expectedTextContent string
				// Adjust expected text content based on known test cases that had spacing issues.
				if tt.name == "With content selector, selector matches" {
					expectedTextContent = "Article Title Selected content here."
				} else if tt.name == "With content selector, selector does NOT match (fallback to full rawHTML, no pre-filtering)" {
					expectedTextContent = "Page Header Some text."
				} else {
					expectedTextContent = stripTags(tt.wantArticleHTML)
				}

				actualTextContent := stripTags(pageData.ArticleHTML)
				if expectedTextContent != "" && !strings.Contains(actualTextContent, expectedTextContent) {
					t.Errorf("PageData.ArticleHTML's text content = %q, does not contain expected text %q (derived from %q)", actualTextContent, expectedTextContent, tt.wantArticleHTML)
				}
			}

			if tt.preFilteringCheck != nil {
				tt.preFilteringCheck(t, pageData, tt.rawHTML)
			}
			if tt.selectorCheck != nil {
				tt.selectorCheck(t, pageData)
			}
		})
	}
}

// stripTags is a very basic way to remove HTML tags for text comparison.
// For more robust stripping, a proper HTML parser would be better.
func stripTags(html string) string {
	var result strings.Builder
	inTag := false
	for _, r := range html {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				result.WriteRune(r)
			}
		}
	}
	return strings.Join(strings.Fields(result.String()), " ")
}

func TestFormatPageDataAsXML(t *testing.T) {
	tests := []struct {
		name string
		page PageData
		want string
	}{
		{
			name: "simple page",
			page: PageData{
				Title:    "Test Title",
				URL:      "http://example.com/test",
				Markdown: "## Hello\nThis is content.",
			},
			want: "<page>\n  <title>Test Title</title>\n  <url>http://example.com/test</url>\n  <content>\n## Hello\nThis is content.\n  </content>\n</page>",
		},
		{
			name: "empty content",
			page: PageData{
				Title:    "Empty Content Page",
				URL:      "http://example.com/empty",
				Markdown: "",
			},
			want: "<page>\n  <title>Empty Content Page</title>\n  <url>http://example.com/empty</url>\n  <content>\n\n  </content>\n</page>",
		},
		{
			name: "empty title",
			page: PageData{
				Title:    "",
				URL:      "http://example.com/no-title",
				Markdown: "Some markdown.",
			},
			want: "<page>\n  <title></title>\n  <url>http://example.com/no-title</url>\n  <content>\nSome markdown.\n  </content>\n</page>",
		},
		{
			name: "content with XML special characters (should be fine as it's within CDATA-like block)",
			page: PageData{
				Title:    "Special Chars < > &",
				URL:      "http://example.com/special",
				Markdown: "Text with <, >, &, ' and \" should appear as is.",
			},
			// The current Sprintf doesn't escape these for the content block, which is typical for this kind of XML-like format.
			want: "<page>\n  <title>Special Chars < > &</title>\n  <url>http://example.com/special</url>\n  <content>\nText with <, >, &, ' and \" should appear as is.\n  </content>\n</page>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPageDataAsXML(&tt.page)
			normalizedGot := strings.ReplaceAll(got, "\r\n", "\n")
			normalizedWant := strings.ReplaceAll(tt.want, "\r\n", "\n")
			if normalizedGot != normalizedWant {
				t.Errorf("formatPageDataAsXML() =\n%q\nwant\n%q", normalizedGot, normalizedWant)
			}
		})
	}
}
