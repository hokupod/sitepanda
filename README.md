# Sitepanda

Sitepanda is a command-line interface (CLI) tool written in Go. It is designed to scrape websites using a headless browser (Chromium by default, or Lightpanda, controlled via Playwright), starting from a user-provided URL. The primary goal is to extract the main readable content from web pages and save it as Markdown. This project is inspired by the functionality of [sitefetch](https://github.com/egoist/sitefetch).

**Note on Default Browser:** While Sitepanda initially defaulted to Lightpanda, Chromium is now the default browser due to its broader compatibility and stability. Lightpanda remains an option for users who prefer it, but if you encounter issues with Lightpanda, we recommend using Chromium (`--browser chromium` or by default).

## Features

*   Scrapes websites starting from a given URL.
*   Crawls same-domain links to discover pages.
*   Uses a headless browser (Chromium by default, or Lightpanda, controlled via Playwright) for page fetching and JavaScript execution, enabling interaction with dynamic, JavaScript-rich websites.
    *   Browser dependencies (Chromium or Lightpanda) are managed by Sitepanda. The `init` command downloads and installs the chosen browser.
    *   Chromium is the recommended default due to its stability and wide compatibility.
*   If no specific `--content-selector` is provided, Sitepanda pre-filters the HTML by removing `<script>`, `<style>`, `<link>`, `<img>`, and `<video>` tags before attempting content extraction.
*   Extracts the main "readable" content from each page using `go-readability`.
*   Converts the extracted HTML content to Markdown.
*   Outputs the scraped data (title, URL, Markdown content) in an XML-like text format by default.
*   If the `--outfile` path ends with `.json`, output is in JSON format (an array of page objects).
*   Provides options to filter pages by URL patterns (`--match`) and to stop crawling once a specified number of pages have had their content saved (`--limit`).
*   Allows specifying URL patterns (`--follow-match`) to restrict which discovered links are added to the crawl queue, preventing crawls from expanding into unwanted areas (e.g., other user profiles on a social media site).
*   Allows specifying a CSS selector (`--content-selector`) to target the main content area of a page for more precise extraction (this bypasses the default pre-filtering).
*   Allows switching page load waiting strategy between `load` (default) and `networkidle` using the `--wait-for-network-idle` or `-wni` flag.

## Technical Stack

*   **Programming Language:** Go
*   **Headless Browser:**
    *   Chromium (via Playwright, **default**):
        *   Installed and managed by Playwright when `sitepanda init chromium` is run. Playwright installs Chromium into a Sitepanda-managed directory (e.g., `$XDG_DATA_HOME/sitepanda/playwright_driver/` on Linux).
        *   Sitepanda uses Playwright to launch and control Chromium.
    *   [Lightpanda](https://github.com/lightpanda-io/browser) (optional):
        *   Lightpanda can be installed and managed by Sitepanda's `init lightpanda` command. It is stored in a user-specific data directory (e.g., `$XDG_DATA_HOME/sitepanda/bin/lightpanda` on Linux, `~/Library/Application Support/Sitepanda/bin/lightpanda` on macOS).
        *   Sitepanda launches and manages Lightpanda processes if selected.
        *   Lightpanda is started in `serve` mode (e.g., `./lightpanda serve --host 127.0.0.1 --port <auto-assigned>`). Sitepanda connects to it via CDP.
*   **Browser Control (CDP Client / Browser Automation):** [`playwright-community/playwright-go`](https://github.com/playwright-community/playwright-go)
    *   Used for launching/controlling Chromium and for CDP connection to Lightpanda.
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

*   `init [chromium|lightpanda]`: Downloads and installs the specified browser dependency (default: `chromium`). `chromium` is installed by Playwright into a Sitepanda-managed directory. `lightpanda` is installed to a user-specific data directory. This must be run once before scraping with the chosen browser.
*   If no command is specified, Sitepanda assumes a URL is provided and attempts to start scraping.

**Arguments (for scraping):**

*   `url`: The starting URL to fetch.

**Options (for scraping):** 

*   `--browser <name>, -b <name>`: Specify the browser to use for scraping (`chromium` or `lightpanda`). Default: `chromium` (or the value of the `SITEPANDA_BROWSER` environment variable if set).
*   `-o, --outfile <path>`: Write the fetched site to a text file. If the path ends with `.json`, the output will be in JSON format. Otherwise, it defaults to an XML-like text format.
*   `-m, --match <pattern>`: Only extract content from matched pages (glob pattern, can be specified multiple times). Non-matching pages on the same domain are still crawled for links until the `--limit` is reached.
*   `--follow-match <pattern>`: Only add links matching this glob pattern to the crawl queue (can be specified multiple times). This helps control the scope of the crawl. For example, on a social media site, you might use `--follow-match "/username/**"` to only crawl links related to a specific user.
*   `--limit <number>`: Stop crawling and fetching new pages once this many pages have had their content successfully saved (0 for no limit).
*   `--content-selector <selector>`: Specify a CSS selector (e.g., `.article-body`) to identify the main content area of a page. If provided, `go-readability` will process only the content of the first matching element; the default HTML pre-filtering (of script, img, etc.) is skipped in this case. If the selector is provided but does not match any elements on the page, Sitepanda will fall back to processing the original, full HTML content without applying the default pre-filtering.
*   `--wait-for-network-idle, -wni`: Wait for network to be idle instead of just `load` (default) when fetching pages. This can be useful for pages that load content dynamically after the initial `load` event.
*   `--silent`: Do not print any logs.
*   `--version`: Show version information.

**Environment Variables:**

*   `SITEPANDA_BROWSER`: Specifies the default browser to use (`chromium` or `lightpanda`). This can be overridden by the `--browser` or `-b` command-line options.

## Crawling Logic

*   A queue manages URLs to be visited.
*   A set (or map) tracks visited URLs to prevent re-fetching and loops.
*   Links are filtered to ensure they are on the same host as the starting URL and use HTTP/HTTPS.
*   If `--follow-match` patterns are provided, discovered links are further filtered. Only links whose paths match one of these patterns will be added to the queue for crawling.
*   Connection to the browser (Chromium via Playwright launch, or Lightpanda via CDP) for robust interaction with dynamic web pages.
*   Page fetching waits for the `load` event by default. If `--wait-for-network-idle` or `-wni` is specified, it waits for the network to become idle.
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
*   Errors encountered during page fetching or processing are logged. Sitepanda attempts to continue processing other pages if the error is page-specific, but will halt the crawl if critical browser connection errors occur or if the required browser is not installed (guiding the user to run `sitepanda init [browser]`).

## Current Status and Known Issues

**Current Functionality:**
*   CLI with `init [browser]` command for Lightpanda or Chromium setup.
*   CLI flags for URL, outfile (supports `.json` for JSON output), limit, match, follow-match, content-selector, silent, version, and `browser` (with `-b` shorthand, defaulting to Chromium).
*   CLI flag `--wait-for-network-idle` (with `-wni` shorthand) to control page load waiting strategy.
*   Support for `SITEPANDA_BROWSER` environment variable to set the default browser (Chromium or Lightpanda).
*   Dynamic download, installation, and launching/termination of browser dependencies (Lightpanda or Chromium via Playwright).
*   Launch/control of Chromium via Playwright (default) or connection to Lightpanda via CDP for robust interaction with dynamic web pages.
*   Sequential crawling of same-domain URLs starting from a given URL, using a single browser page instance.
*   URL normalization to handle minor variations.
*   Default HTML pre-filtering (removes script, style, link, img, video tags) when `--content-selector` is not used.
*   Extraction of readable content using `go-readability`, optionally guided by `--content-selector`.
*   Conversion of extracted HTML to Markdown.
*   Output of (Title, URL, Markdown Content) in XML-like text format or JSON format to file, or XML-like text to stdout.
*   `--limit` functionality stops the crawl once the specified number of pages have had their content saved. 
*   `--match` functionality filters which pages have their content saved.
*   `--follow-match` functionality filters which links are added to the crawl queue.
*   Graceful shutdown on OS signals.
*   Initial unit tests for URL normalization and other components.

**Known Issues:**
*   While Lightpanda is efficient for many websites, some users have reported occasional instability with specific site structures or technologies. To enhance compatibility across a broader range of websites and offer a consistently stable alternative, Sitepanda now includes support for the Chromium browser. If you encounter a website where Lightpanda does not perform as expected, we recommend trying to scrape it using Chromium with the `--browser chromium` option.
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
    Run the `init` command to download and set up your chosen browser:

    *   **For Chromium (default):**
        ```bash
        sitepanda init
        # or
        sitepanda init chromium
        ```
        This command will use Playwright to download and install Chromium. Playwright will install it into a Sitepanda-managed directory (e.g., `~/.local/share/sitepanda/playwright_driver/` on Linux, `~/Library/Application Support/Sitepanda/playwright_driver/` on macOS). This directory is used by Playwright to find its browser binaries. You only need to run this once for Chromium.

    *   **For Lightpanda (optional):**
        ```bash
        sitepanda init lightpanda
        ```
        This command will download the appropriate Lightpanda binary for your OS (Linux or macOS) and install it into a user-specific data directory (e.g., `~/.local/share/sitepanda/bin/` on Linux or `~/Library/Application Support/Sitepanda/bin/` on macOS). You only need to run this once for Lightpanda, unless you want to re-download it.

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
    Even when building from source, you need to run the `init` command to get your chosen browser:
    ```bash
    # For Chromium (default)
    ./sitepanda init

    # For Lightpanda (optional)
    ./sitepanda init lightpanda
    ```

## Running

After installation and initialization:

```bash
# Scrape using Chromium (default) and output in XML-like text format to output.txt
sitepanda --outfile output.txt --match "/blog/**" https://example.com

# Scrape using Chromium (default) and output in JSON format to output.json
sitepanda --outfile output.json --match "/blog/**" https://example.com

# Scrape using Lightpanda (short option) and output in JSON format to output.json
sitepanda -b lightpanda --outfile output.json --match "/blog/**" https://example.com

# Scrape using Lightpanda (long option) and output in JSON format to output.json
sitepanda --browser lightpanda --outfile output.json --match "/blog/**" https://example.com

# Scrape using Chromium with a specific content selector
sitepanda --browser chromium --outfile output.json --content-selector ".main-article-body" https://example.com

# Scrape using Chromium (specified via environment variable) and output to stdout
SITEPANDA_BROWSER=chromium sitepanda https://example.com

# Scrape using Chromium (default), wait for network idle (long option), and output to JSON
sitepanda --wait-for-network-idle --outfile output.json https://example.com

# Scrape using Chromium (default), wait for network idle (shorthand option), and output to JSON
sitepanda -wni --outfile output.json https://example.com

# Scrape a specific user's profile on a social media site, avoiding links to other profiles, and save to JSON
sitepanda --outfile user_posts.json --follow-match "/username/posts/**" --follow-match "/username/details" https://example.com/username
```

To run the tests:
```bash
go test ./...
```

## License

Sitepanda is licensed under the [MIT License](LICENSE).

## Development TODO / Next Steps

*   **Testing (Unit Tests):**
    *   Ensure comprehensive unit tests for new path management and `init` command logic for both browsers.
    *   Add unit tests for the `--wait-for-network-idle` / `-wni` flag logic.
*   **Testing (Integration Tests):**
    *   Develop integration tests for the end-to-end scraping process with both Lightpanda and Chromium, including the `init` flow.
    *   Include test cases for different page load waiting strategies.
*   **Robustness and Error Handling:**
    *   Further refine error handling for `sitepanda init [browser]` (network issues, disk space, permissions).
*   **CLI Refinements:**
    *   Consider a more sophisticated CLI parsing library if subcommands become more complex.
*   **Windows Support:**
    *   Lightpanda is not currently available for Windows. However, Chromium (via Playwright) works on Windows. `sitepanda init chromium` and scraping with `--browser chromium` should be tested and confirmed on Windows. Path logic for Playwright's driver directory on Windows needs to be ensured.
