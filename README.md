# Sitepanda

Sitepanda is a command-line interface (CLI) tool written in Go. It is designed to scrape websites using the Lightpanda headless browser, starting from a user-provided URL. The primary goal is to extract the main readable content from web pages and save it as Markdown. This project is inspired by the functionality of `sitefetch`.

## Features

*   Scrapes websites starting from a given URL.
*   Crawls same-domain links to discover pages.
*   Uses Lightpanda headless browser (controlled via Playwright) for page fetching and JavaScript execution, enabling interaction with dynamic, JavaScript-rich websites.
    *   Lightpanda is managed as an external dependency. Sitepanda provides an `init` command to download and install it.
*   If no specific `--content-selector` is provided, Sitepanda pre-filters the HTML by removing `<script>`, `<style>`, `<link>`, `<img>`, and `<video>` tags before attempting content extraction.
*   Extracts the main "readable" content from each page using `go-readability`.
*   Converts the extracted HTML content to Markdown.
*   Outputs the scraped data (title, URL, Markdown content) in an XML-like text format by default.
*   If the `--outfile` path ends with `.json`, output is in JSON format (an array of page objects).
*   Provides options to filter pages by URL patterns (`--match`) and to stop crawling once a specified number of pages have had their content saved (`--limit`).
*   Allows specifying a CSS selector (`--content-selector`) to target the main content area of a page for more precise extraction (this bypasses the default pre-filtering).

## Technical Stack

*   **Programming Language:** Go
*   **Headless Browser:** [Lightpanda](https://github.com/lightpanda-io/browser)
    *   Lightpanda is installed and managed by Sitepanda's `init` command. It is stored in a user-specific data directory (e.g., `$XDG_DATA_HOME/sitepanda/bin/lightpanda` on Linux, `~/Library/Application Support/Sitepanda/bin/lightpanda` on macOS).
    *   Sitepanda launches and manages Lightpanda processes.
    *   Lightpanda is started in `serve` mode (e.g., `./lightpanda serve --host 127.0.0.1 --port <auto-assigned>`).
*   **Browser Control (CDP Client):** [`playwright-community/playwright-go`](https://github.com/playwright-community/playwright-go)
*   **HTML Parsing (for selector, link extraction, and pre-filtering):** [`PuerkitoBio/goquery`](https://github.com/PuerkitoBio/goquery)
*   **HTML Content Extraction:** [`go-shiori/go-readability`](https://github.com/go-shiori/go-readability)
*   **HTML to Markdown Conversion:** [`JohannesKaufmann/html-to-markdown`](https://github.com/JohannesKaufmann/html-to-markdown) (with GitHub Flavored Markdown plugin)
*   **JSON Handling:** Standard Go `encoding/json` package.
*   **Testing:** Standard Go `testing` package.

## Command-Line Interface

Sitepanda provides the following CLI structure:

```bash
sitepanda [command] [options] <url>
```

**Commands:**

*   `init`: Downloads and installs the Lightpanda browser dependency to the appropriate user-specific data directory. This must be run once before scraping.
*   If no command is specified, Sitepanda assumes a URL is provided and attempts to start scraping.

**Arguments (for scraping):**

*   `url`: The starting URL to fetch.

**Options (for scraping):**

*   `-o, --outfile <path>`: Write the fetched site to a text file. If the path ends with `.json`, the output will be in JSON format. Otherwise, it defaults to an XML-like text format.
*   `-m, --match <pattern>`: Only extract content from matched pages (glob pattern, can be specified multiple times). Non-matching pages on the same domain are still crawled for links until the `--limit` is reached.
*   `--limit <number>`: Stop crawling and fetching new pages once this many pages have had their content successfully saved (0 for no limit).
*   `--content-selector <selector>`: Specify a CSS selector (e.g., `.article-body`) to identify the main content area of a page. If provided, `go-readability` will process only the content of the first matching element; the default HTML pre-filtering (of script, img, etc.) is skipped in this case. If the selector is provided but does not match any elements on the page, Sitepanda will fall back to processing the original, full HTML content without applying the default pre-filtering.
*   `--silent`: Do not print any logs.
*   `--version`: Show version information.

## Crawling Logic

*   A queue manages URLs to be visited.
*   A set (or map) tracks visited URLs to prevent re-fetching and loops.
*   Links are filtered to ensure they are on the same host as the starting URL and use HTTP/HTTPS.
*   If a `--content-selector` is provided, Sitepanda attempts to extract HTML from the first matching element. This specific HTML is then passed to the readability engine.
*   If no `--content-selector` is provided, Sitepanda performs a pre-filtering step on the full HTML: it removes all `<script>`, `<style>`, `<link>`, `<img>`, and `<video>` tags. The resulting modified HTML is then passed to the readability engine.
*   The `--match` option determines if a page's content is extracted and saved.
*   The `--limit` option stops the entire crawl (fetching, processing, and link extraction from new pages) once the specified number of pages have had their content saved.

## Output Format

Sitepanda supports two output formats:

1.  **XML-like Text (Default):**
    If `--outfile` is not specified (output to stdout) or if the `--outfile` path does not end with `.json`.

    ```text
    <page>
      <title>Page Title</title>
      <url>http://example.com/page-url</url>
      <content>
    ## This is the Markdown content

    Extracted from the page...
      </content>
    </page>
    ...
    ```

2.  **JSON:**
    If the `--outfile` path ends with `.json`. The output is a JSON array of page objects.

    ```json
    [
      {
        "title": "Page Title",
        "url": "http://example.com/page-url",
        "content": "## This is the Markdown content\n\nExtracted from the page..."
      },
      ...
    ]
    ```

## Logging and Error Handling

*   A simple logger outputs `INFO` and `WARN` level messages.
*   The `--silent` flag suppresses all log output.
*   Errors encountered during page fetching or processing are logged. Sitepanda attempts to continue processing other pages if the error is page-specific, but will halt the crawl if critical browser connection errors occur or if Lightpanda is not installed (guiding the user to run `sitepanda init`).

## Current Status and Known Issues

**Current Functionality:**
*   CLI with `init` command for Lightpanda setup.
*   CLI flags for URL, outfile (supports `.json` for JSON output), limit, match, content-selector, silent, version.
*   Dynamic download, installation, and launching/termination of Lightpanda browser via `sitepanda init` and subsequent runs.
*   Connection to Lightpanda via Playwright for robust interaction with dynamic web pages.
*   Sequential crawling of same-domain URLs starting from a given URL, using a single browser page instance.
*   URL normalization to handle minor variations.
*   Default HTML pre-filtering (removes script, style, link, img, video tags) when `--content-selector` is not used.
*   Extraction of readable content using `go-readability`, optionally guided by `--content-selector`.
*   Conversion of extracted HTML to Markdown.
*   Output of (Title, URL, Markdown Content) in XML-like text format or JSON format to file, or XML-like text to stdout.
*   `--limit` functionality stops the crawl once the specified number of pages have had their content saved.
*   `--match` functionality filters which pages have their content saved.
*   Graceful shutdown on OS signals.
*   Initial unit tests for URL normalization and other components.

**Known Issues:**
*   (Refer to previous list, ensure they are still relevant or update as needed).

## Installation and Setup

1.  **Install Go:** Ensure you have Go installed (version 1.19 or newer recommended).
    (This is required if you plan to build from source or use `go install`).

2.  **Install Sitepanda:**

    You can install Sitepanda using one of the following methods:

    *   **Using Homebrew (macOS or Linuxbrew):**
        ```bash
        brew install hokupod/tap/sitepanda
        ```
    *   **Using `go install`:**
        ```bash
        go install github.com/hokupod/sitepanda@latest
        ```
        This will install the `sitepanda` binary to your Go bin directory (e.g., `~/go/bin`). Make sure this directory is in your system's `PATH`.

3.  **Initialize Sitepanda:**
    Run the `init` command to download and set up the Lightpanda browser:
    ```bash
    sitepanda init
    ```
    This command will download the appropriate Lightpanda binary for your OS (Linux or macOS) and install it into a user-specific data directory (e.g., `~/.local/share/sitepanda/bin/` on Linux or `~/Library/Application Support/Sitepanda/bin/` on macOS). You only need to run this once, unless you want to re-download Lightpanda.

## Building from Source

If you prefer to build from source:

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/hokupod/sitepanda.git
    cd sitepanda
    ```
2.  **Build the binary:**
    ```bash
    go build .
    ```
    This will create a `sitepanda` executable in the current directory.
3.  **Initialize Sitepanda (if not done already):**
    Even when building from source, you need to run the `init` command to get Lightpanda:
    ```bash
    ./sitepanda init
    ```

## Running

After installation and initialization:

```bash
# Output in XML-like text format to output.txt
sitepanda --outfile output.txt --match "/blog/**" https://example.com

# Output in JSON format to output.json
sitepanda --outfile output.json --match "/blog/**" https://example.com

# To use a specific content selector:
sitepanda --outfile output.json --content-selector ".main-article-body" https://example.com
```

To run the tests:
```bash
go test ./...
```

## License

Sitepanda is licensed under the [MIT License](LICENSE).

## Development TODO / Next Steps

*   **Testing (Unit Tests):**
    *   Ensure comprehensive unit tests for new path management and `init` command logic.
*   **Testing (Integration Tests):**
    *   Develop integration tests for the end-to-end scraping process, including the `init` flow (perhaps by mocking the download or checking for the binary).
*   **Robustness and Error Handling:**
    *   Further refine error handling for `sitepanda init` (network issues, disk space, permissions).
*   **CLI Refinements:**
    *   Consider a more sophisticated CLI parsing library if subcommands become more complex.
*   **Windows Support:**
    *   Currently, Lightpanda is not available for Windows. If it becomes available, extend `sitepanda init` and path logic.
