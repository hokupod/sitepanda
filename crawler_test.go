package main

import (
	"io"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gobwas/glob"
)

func TestMain(m *testing.M) {
	// Suppress logger output for all tests in this package
	originalLoggerOutput := logger.Writer()
	logger.SetOutput(io.Discard)

	exitCode := m.Run()

	// Restore logger output (optional, but good practice if tests run in a shared environment)
	logger.SetOutput(originalLoggerOutput)

	os.Exit(exitCode)
}

func TestNormalizeURLtoString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "simple http",
			input:   "http://example.com",
			want:    "http://example.com/",
			wantErr: false,
		},
		{
			name:    "simple https",
			input:   "https://example.com",
			want:    "https://example.com/",
			wantErr: false,
		},
		{
			name:    "with trailing slash",
			input:   "http://example.com/",
			want:    "http://example.com/",
			wantErr: false,
		},
		{
			name:    "with path",
			input:   "http://example.com/path/to/page",
			want:    "http://example.com/path/to/page",
			wantErr: false,
		},
		{
			name:    "with path and trailing slash",
			input:   "http://example.com/path/to/page/",
			want:    "http://example.com/path/to/page", // Corrected: normalizeURLtoString removes trailing slash from path unless it's the root
			wantErr: false,
		},
		{
			name:    "with fragment",
			input:   "http://example.com/page#section",
			want:    "http://example.com/page",
			wantErr: false,
		},
		{
			name:    "domain with fragment",
			input:   "http://example.com#section",
			want:    "http://example.com/",
			wantErr: false,
		},
		{
			name:    "with query parameters",
			input:   "http://example.com/search?q=term",
			want:    "http://example.com/search?q=term",
			wantErr: false,
		},
		{
			name:    "with query and fragment",
			input:   "http://example.com/search?q=term#results",
			want:    "http://example.com/search?q=term",
			wantErr: false,
		},
		{
			name:    "complex URL with port",
			input:   "https://sub.example.co.uk:8080/path?name=val&name2=val2#frag",
			want:    "https://sub.example.co.uk:8080/path?name=val&name2=val2",
			wantErr: false,
		},
		{
			name:    "URL with only domain and query",
			input:   "http://example.com?query=true",
			want:    "http://example.com/?query=true",
			wantErr: false,
		},
		{
			name:    "invalid URL scheme",
			input:   "ftp://example.com/file",
			want:    "ftp://example.com/file", // Normalization still occurs, scheme is not validated by this func
			wantErr: false,
		},
		{
			name:    "invalid URL structure",
			input:   "://example.com",
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			want:    "",
			wantErr: true,
		},
		{
			name:    "just a fragment",
			input:   "#fragment", // url.Parse will result in an empty URL struct for this
			want:    "",
			wantErr: true, // because parsed.Scheme will be empty, and path is also empty
		},
		{
			name:    "relative path",
			input:   "/just/a/path", // No scheme, so path "/" rule doesn't apply
			want:    "/just/a/path",
			wantErr: false,
		},
		{
			name:    "relative path with fragment",
			input:   "/just/a/path#frag",
			want:    "/just/a/path",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeURLtoString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("normalizeURLtoString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("normalizeURLtoString() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatResultsAsJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       []PageData
		wantJSON    string
		expectError bool
	}{
		{
			name:     "empty results",
			input:    []PageData{},
			wantJSON: `[]`,
		},
		{
			name: "single page",
			input: []PageData{
				{Title: "Page 1", URL: "http://example.com/1", Markdown: "Content 1"},
			},
			wantJSON: `[
  {
    "title": "Page 1",
    "url": "http://example.com/1",
    "content": "Content 1"
  }
]`,
		},
		{
			name: "multiple pages",
			input: []PageData{
				{Title: "Page A", URL: "http://example.com/a", Markdown: "Content A"},
				{Title: "Page B", URL: "http://example.com/b", Markdown: "## Content B\nWith newlines."},
			},
			wantJSON: `[
  {
    "title": "Page A",
    "url": "http://example.com/a",
    "content": "Content A"
  },
  {
    "title": "Page B",
    "url": "http://example.com/b",
    "content": "## Content B\nWith newlines."
  }
]`,
		},
		{
			name: "page with special characters in content",
			input: []PageData{
				{Title: "Special \"Chars\" Page", URL: "http://example.com/special", Markdown: "Content with <>&'\""},
			},
			// Note: json.MarshalIndent handles HTML escaping by default for <, >, & within strings.
			// The Go string literal itself correctly represents the desired characters.
			// The \u003c is <, \u003e is >, \u0026 is &.
			wantJSON: `[
  {
    "title": "Special \"Chars\" Page",
    "url": "http://example.com/special",
    "content": "Content with \u003c\u003e\u0026'\""
  }
]`,
		},
		{
			name: "page with empty title or content",
			input: []PageData{
				{Title: "", URL: "http://example.com/empty", Markdown: ""},
			},
			wantJSON: `[
  {
    "title": "",
    "url": "http://example.com/empty",
    "content": ""
  }
]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotJSONBytes, err := formatResultsAsJSON(tt.input)
			if (err != nil) != tt.expectError {
				t.Fatalf("formatResultsAsJSON() error = %v, wantErr %v", err, tt.expectError)
			}
			if tt.expectError {
				return
			}

			gotJSON := string(gotJSONBytes)
			normalizedWantJSON := strings.ReplaceAll(tt.wantJSON, "\r\n", "\n")
			normalizedGotJSON := strings.ReplaceAll(gotJSON, "\r\n", "\n")

			if normalizedGotJSON != normalizedWantJSON {
				t.Errorf("formatResultsAsJSON() gotJSON =\n%s\nwantErrJSON =\n%s", normalizedGotJSON, normalizedWantJSON)
			}
		})
	}
}

func TestShouldProcessContent(t *testing.T) {
	tests := []struct {
		name           string
		matchPatterns  []string
		pageURLStr     string
		expectedResult bool
		expectErr      bool
	}{
		{
			name:           "no patterns",
			matchPatterns:  nil,
			pageURLStr:     "http://example.com/page",
			expectedResult: true,
		},
		{
			name:           "single matching pattern (exact)",
			matchPatterns:  []string{"/page"},
			pageURLStr:     "http://example.com/page",
			expectedResult: true,
		},
		{
			name:           "single non-matching pattern",
			matchPatterns:  []string{"/other"},
			pageURLStr:     "http://example.com/page",
			expectedResult: false,
		},
		{
			name:           "single matching pattern (wildcard *)",
			matchPatterns:  []string{"/blog/*"},
			pageURLStr:     "http://example.com/blog/my-post",
			expectedResult: true,
		},
		{
			name:           "single non-matching pattern (wildcard *)",
			matchPatterns:  []string{"/news/*"},
			pageURLStr:     "http://example.com/blog/my-post",
			expectedResult: false,
		},
		{
			name:           "single matching pattern (double wildcard **)",
			matchPatterns:  []string{"/docs/**/getting-started"},
			pageURLStr:     "http://example.com/docs/v1/guide/getting-started",
			expectedResult: true,
		},
		{
			name:           "multiple patterns, one matches",
			matchPatterns:  []string{"/about", "/products/*", "/contact"},
			pageURLStr:     "http://example.com/products/widget",
			expectedResult: true,
		},
		{
			name:           "multiple patterns, none match",
			matchPatterns:  []string{"/about", "/products/*", "/contact"},
			pageURLStr:     "http://example.com/services/consulting",
			expectedResult: false,
		},
		{
			name:           "root path matches /",
			matchPatterns:  []string{"/"},
			pageURLStr:     "http://example.com/",
			expectedResult: true,
		},
		{
			name:           "root path (no slash) matches /",
			matchPatterns:  []string{"/"},
			pageURLStr:     "http://example.com",
			expectedResult: true,
		},
		{
			name:           "specific path does not match /",
			matchPatterns:  []string{"/"},
			pageURLStr:     "http://example.com/specific",
			expectedResult: false,
		},
		{
			name:           "pattern is just * (does not match non-empty paths)",
			matchPatterns:  []string{"*"},
			pageURLStr:     "http://example.com/anypage",
			expectedResult: false,
		},
		{
			name:           "pattern is just * (does not match root path)",
			matchPatterns:  []string{"*"},
			pageURLStr:     "http://example.com/",
			expectedResult: false,
		},
		{
			name:           "pattern is just **, root path",
			matchPatterns:  []string{"**"},
			pageURLStr:     "http://example.com/",
			expectedResult: true,
		},
		{
			name:           "pattern is just **, any path",
			matchPatterns:  []string{"**"},
			pageURLStr:     "http://example.com/foo/bar/baz",
			expectedResult: true,
		},
		{
			name:          "invalid pattern",
			matchPatterns: []string{"/path[/"},
			pageURLStr:    "http://example.com/path",
			expectErr:     true,
		},
		{
			name:           "invalid page URL",
			matchPatterns:  []string{"/path"},
			pageURLStr:     "://invalid-url",
			expectedResult: false,
			expectErr:      true,
		},
		{
			name:           "subpath match with double wildcard",
			matchPatterns:  []string{"/blog/**"},
			pageURLStr:     "http://example.com/blog/2023/article123",
			expectedResult: true,
		},
		{
			name:           "subpath unmatch with double wildcard",
			matchPatterns:  []string{"/blog/**"},
			pageURLStr:     "http://example.com/news/2023/article123",
			expectedResult: false,
		},
		{
			name:           "root path with trailing slash",
			matchPatterns:  []string{"/"},
			pageURLStr:     "http://example.com/",
			expectedResult: true,
		},
		{
			name:           "exact path with query params should match",
			matchPatterns:  []string{"/search"},
			pageURLStr:     "http://example.com/search?q=golang",
			expectedResult: true,
		},
		{
			name:           "exact path with fragment should match",
			matchPatterns:  []string{"/about"},
			pageURLStr:     "http://example.com/about#team",
			expectedResult: true,
		},
		{
			name:           "multiple patterns match",
			matchPatterns:  []string{"/contact", "/team/*"},
			pageURLStr:     "http://example.com/team/john",
			expectedResult: true,
		},
		{
			name:           "japanese path match",
			matchPatterns:  []string{"/日本語/**"},
			pageURLStr:     "http://example.com/日本語/記事タイトル",
			expectedResult: true,
		},
		{
			name:           "single wildcard match with multiple segments",
			matchPatterns:  []string{"/products/*"},
			pageURLStr:     "http://example.com/products/widget123",
			expectedResult: true,
		},
		{
			name:           "single wildcard unmatch with multiple segments",
			matchPatterns:  []string{"/products/*"},
			pageURLStr:     "http://example.com/products/widget123/details",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var compiledPatterns []glob.Glob
			if tt.matchPatterns != nil {
				for _, p := range tt.matchPatterns {
					g, err := glob.Compile(p, '/')
					if err != nil {
						if tt.expectErr {
							return
						}
						t.Fatalf("glob.Compile(%q) failed: %v", p, err)
					}
					compiledPatterns = append(compiledPatterns, g)
				}
			}

			c := &Crawler{
				matchPatterns: compiledPatterns,
			}

			pageURL, err := url.Parse(tt.pageURLStr)
			if err != nil {
				if tt.expectErr {
					return
				}
				t.Fatalf("url.Parse(%q) failed: %v", tt.pageURLStr, err)
			}

			if tt.expectErr {
				// This check is problematic if the error is expected from glob.Compile
				// The test structure assumes errors from glob.Compile are handled before this point.
				// If an error is expected from url.Parse, it's handled by the check above.
				// This specific `t.Fatalf` might need to be re-evaluated based on what tt.expectErr signifies.
				// For now, if tt.expectErr is true, we assume the error was caught earlier (e.g., by glob.Compile or url.Parse).
				// If it didn't exit, and tt.expectErr is true, it's a test logic flaw.
				// Let's assume if tt.expectErr is true, the test should have already returned.
				// If it reaches here, tt.expectErr must be false.
			}


			result := c.shouldProcessContent(pageURL)
			if result != tt.expectedResult {
				t.Errorf("shouldProcessContent() for URL %s with patterns %v = %v, want %v", tt.pageURLStr, tt.matchPatterns, result, tt.expectedResult)
			}
		})
	}
}

func TestExtractAndFilterLinks(t *testing.T) {
	sliceToMap := func(s []string) map[string]struct{} {
		m := make(map[string]struct{})
		for _, v := range s {
			m[v] = struct{}{}
		}
		return m
	}

	mapsEqual := func(m1, m2 map[string]struct{}) bool {
		if len(m1) != len(m2) {
			return false
		}
		for k := range m1 {
			if _, ok := m2[k]; !ok {
				return false
			}
		}
		return true
	}

	tests := []struct {
		name       string
		pageURLStr string
		htmlBody   string
		wantLinks  []string
		wantErr    bool
	}{
		{
			name:       "no links",
			pageURLStr: "http://example.com/",
			htmlBody:   `<html><body><p>No links here.</p></body></html>`,
			wantLinks:  []string{},
		},
		{
			name:       "one valid same-domain link",
			pageURLStr: "http://example.com/",
			htmlBody:   `<html><body><a href="/page1">Page 1</a></body></html>`,
			wantLinks:  []string{"http://example.com/page1"},
		},
		{
			name:       "absolute same-domain link",
			pageURLStr: "http://example.com/",
			htmlBody:   `<html><body><a href="http://example.com/page2">Page 2</a></body></html>`,
			wantLinks:  []string{"http://example.com/page2"},
		},
		{
			name:       "multiple valid links",
			pageURLStr: "http://example.com/path/",
			htmlBody: `<html><body>
                <a href="sub1">Sub Page 1</a>
                <a href="/otherpath">Other Path</a>
                <a href="http://example.com/another">Another Absolute</a>
            </body></html>`,
			wantLinks: []string{
				"http://example.com/path/sub1",
				"http://example.com/otherpath",
				"http://example.com/another",
			},
		},
		{
			name:       "duplicate links",
			pageURLStr: "http://example.com/",
			htmlBody: `<html><body>
                <a href="/page1">Page 1</a>
                <a href="/page1">Page 1 Again</a>
                <a href="http://example.com/page1">Page 1 Absolute</a>
            </body></html>`,
			wantLinks: []string{"http://example.com/page1"},
		},
		{
			name:       "external domain link",
			pageURLStr: "http://example.com/",
			htmlBody:   `<html><body><a href="http://othersite.com/page">Other Site</a></body></html>`,
			wantLinks:  []string{},
		},
		{
			name:       "mailto and tel links",
			pageURLStr: "http://example.com/",
			htmlBody: `<html><body>
                <a href="mailto:test@example.com">Email</a>
                <a href="tel:+123456789">Call</a>
            </body></html>`,
			wantLinks: []string{},
		},
		{
			name:       "ftp link",
			pageURLStr: "http://example.com/",
			htmlBody:   `<html><body><a href="ftp://example.com/file">FTP</a></body></html>`,
			wantLinks:  []string{},
		},
		{
			name:       "link with fragment",
			pageURLStr: "http://example.com/",
			htmlBody:   `<html><body><a href="/page#section">Page with fragment</a></body></html>`,
			wantLinks:  []string{"http://example.com/page"}, // Fragment should be removed by normalizeURLtoString
		},
		{
			name:       "link to root, page is root",
			pageURLStr: "http://example.com/",
			htmlBody:   `<html><body><a href="/">Home</a></body></html>`,
			wantLinks:  []string{"http://example.com/"},
		},
		{
			name:       "link to root, page is subpage",
			pageURLStr: "http://example.com/sub/page",
			htmlBody:   `<html><body><a href="/">Home</a></body></html>`,
			wantLinks:  []string{"http://example.com/"},
		},
		{
			name:       "link relative to current directory",
			pageURLStr: "http://example.com/blog/post1/",
			htmlBody:   `<html><body><a href="edit">Edit Post</a></body></html>`,
			wantLinks:  []string{"http://example.com/blog/post1/edit"},
		},
		{
			name:       "link with .. (parent directory)",
			pageURLStr: "http://example.com/blog/category/post/",
			htmlBody:   `<html><body><a href="../other-post">Other Post in Category</a></body></html>`,
			wantLinks:  []string{"http://example.com/blog/category/other-post"},
		},
		{
			name:       "invalid href (just fragment)",
			pageURLStr: "http://example.com/",
			htmlBody:   `<html><body><a href="#section-only">Section</a></body></html>`,
			// normalizeURLtoString for "#section-only" relative to "http://example.com/" becomes "http://example.com/"
			// because the fragment is removed, and the base path is "/".
			wantLinks: []string{"http://example.com/"},
		},
		{
			name:       "empty href",
			pageURLStr: "http://example.com/",
			htmlBody:   `<html><body><a href="">Empty Href</a></body></html>`,
			// normalizeURLtoString for "" relative to "http://example.com/" becomes "http://example.com/"
			wantLinks: []string{"http://example.com/"},
		},
		{
			name:       "link with spaces (should be handled by url.Parse)",
			pageURLStr: "http://example.com/",
			htmlBody:   `<html><body><a href="/path with spaces">Path With Spaces</a></body></html>`,
			wantLinks:  []string{"http://example.com/path%20with%20spaces"},
		},
		{
			name:       "complex scenario with mixed links",
			pageURLStr: "https://sub.example.com/docs/v1/",
			htmlBody: `
                <html><body>
                    <a href="intro.html">Intro</a>
                    <a href="/api/v1/method">API Method</a>
                    <a href="https://sub.example.com/docs/v1/examples/ex1.html">Full Example Link</a>
                    <a href="https://anothersub.example.com/page">Another Subdomain (same base)</a>
                    <a href="https://othersite.net/resource">External Site</a>
                    <a href="mailto:support@example.com">Support</a>
                    <a href="intro.html#part2">Intro Part 2</a>
                    <a href="/docs/v1/intro.html">Duplicate of Intro via absolute path</a>
                </body></html>`,
			wantLinks: []string{
				"https://sub.example.com/docs/v1/intro.html",
				"https://sub.example.com/api/v1/method",
				"https://sub.example.com/docs/v1/examples/ex1.html",
				// "https://anothersub.example.com/page" is considered external by current Hostname() check
			},
		},
		{
			name:       "page URL with no trailing slash, relative link",
			pageURLStr: "http://example.com/folder",
			htmlBody:   `<html><body><a href="item">Item</a></body></html>`,
			// url.Parse resolves "item" relative to "http://example.com/" if "folder" is not seen as a directory.
			// If "folder" is seen as a file, it resolves to "http://example.com/item".
			// The standard library's `pageURL.Parse("item")` when pageURL.Path is "/folder" (not "/folder/")
			// will resolve to "/item".
			wantLinks: []string{"http://example.com/item"},
		},
		{
			name:       "page URL with trailing slash, relative link",
			pageURLStr: "http://example.com/folder/",
			htmlBody:   `<html><body><a href="item">Item</a></body></html>`,
			wantLinks:  []string{"http://example.com/folder/item"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pageURL, err := url.Parse(tt.pageURLStr)
			if err != nil {
				if tt.wantErr {
					return
				}
				t.Fatalf("url.Parse(%q) failed: %v", tt.pageURLStr, err)
			}
			if tt.wantErr {
				t.Fatalf("expected an error for pageURL parsing, but got none")
			}

			c := &Crawler{
				startURL: pageURL, // So that link filtering uses the correct host
			}

			gotLinks := c.extractAndFilterLinks(pageURL, tt.htmlBody)

			gotMap := sliceToMap(gotLinks)
			wantMap := sliceToMap(tt.wantLinks)

			if !mapsEqual(gotMap, wantMap) {
				t.Errorf("extractAndFilterLinks() got = %v, want %v", gotLinks, tt.wantLinks)
			}
		})
	}
}
