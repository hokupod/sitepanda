# Sitepanda

Sitepanda is a command-line interface (CLI) tool written in Go, now with support for Windows, macOS, and Linux. It is designed to scrape websites using a headless browser (Chromium by default, or Lightpanda on macOS/Linux, controlled via Playwright), starting from a user-provided URL or a list of URLs from a file. The primary goal is to extract the main readable content from web pages and save it as Markdown. This project is inspired by the functionality of [sitefetch](https://github.com/egoist/sitefetch).

**Note on Default Browser and Platform Support:**
*   Chromium is the default browser due to its broad compatibility and stability across all supported platforms (Windows, macOS, Linux).
*   Lightpanda is an optional browser for macOS and Linux users. It is currently **not supported on Windows**. If you encounter issues with Lightpanda, we recommend using Chromium (`--browser chromium` or by default). On Windows, Sitepanda will exclusively use Chromium.

## Features

*   Scrapes websites starting from a given URL.
*   Processes a list of URLs from a file (`--url-file`), in addition to a single starting URL.
*   Crawls same-domain links to discover pages (when not using `--url-file`).
*   Uses a headless browser (Chromium by default across all platforms; Lightpanda is an option for macOS/Linux) controlled via Playwright for page fetching and JavaScript execution.
    *   Browser dependencies (Chromium for all platforms, Lightpanda for macOS/Linux) are managed by Sitepanda. The `init` command downloads and installs the chosen browser.
    *   Chromium is the recommended default due to its stability and wide compatibility. On Windows, only Chromium is supported.
*   If no specific `--content-selector` is provided, Sitepanda pre-filters the HTML by removing `<script>`, `<style>`, `<link>`, `<img>`, and `<video>` tags before attempting content extraction.
*   Extracts the main "readable" content from each page using `go-readability`.
*   Converts the extracted HTML content to Markdown.
*   Outputs the scraped data (title, URL, Markdown content) in multiple formats, controllable via the `--output-format` flag. Supported formats: `xml-like` (default), `json`, and `jsonl`.
*   Provides options to filter pages by URL patterns (`--match`) and to stop crawling once a specified number of pages have had their content saved (`--limit`).
*   Allows specifying URL patterns (`--follow-match`) to restrict which discovered links are added to the crawl queue, preventing crawls from expanding into unwanted areas (e.g., other user profiles on a social media site). This is not applicable when using `--url-file`.
*   Allows specifying a CSS selector (`--content-selector`) to target the main content area of a page for more precise extraction (this bypasses the default pre-filtering).
*   Allows switching page load waiting strategy between `load` (default) and `networkidle` using the `--wait-for-network-idle` or `-wni` flag.

## Technical Stack

*   **Programming Language:** Go
*   **Headless Browser:**
    *   Chromium (via Playwright, **default on all platforms**):
        *   Installed and managed by Playwright when `sitepanda init chromium` is run. Playwright installs Chromium into a Sitepanda-managed directory.
            *   Linux: e.g., `$XDG_DATA_HOME/sitepanda/playwright_driver/`
            *   macOS: e.g., `~/Library/Application Support/Sitepanda/playwright_driver/`
            *   Windows: e.g., `%LOCALAPPDATA%\Sitepanda\playwright_driver\`
        *   Sitepanda uses Playwright to launch and control Chromium.
    *   [Lightpanda](https://github.com/lightpanda-io/browser) (optional, **macOS and Linux only**):
        *   Lightpanda can be installed by `sitepanda init lightpanda` on macOS and Linux. It is stored in a user-specific data directory:
            *   Linux: e.g., `$XDG_DATA_HOME/sitepanda/bin/lightpanda`
            *   macOS: e.g., `~/Library/Application Support/Sitepanda/bin/lightpanda`
        *   **Windows Support:** Lightpanda is currently not supported on Windows. Attempting to run `sitepanda init lightpanda` on Windows will result in an error.
        *   Sitepanda launches and manages Lightpanda processes if selected on a compatible OS.
        *   Lightpanda is started in `serve` mode (e.g., `./lightpanda serve --host 127.0.0.1 --port <auto-assigned>`). Sitepanda connects to it via CDP.
*   **Browser Control (CDP Client / Browser Automation):** [`playwright-community/playwright-go`](https://github.com/playwright-community/playwright-go)
    *   Used for launching/controlling Chromium (all platforms) and for CDP connection to Lightpanda (macOS/Linux).
*   **HTML Parsing (for selector, link extraction, and pre-filtering):** [`PuerkitoBio/goquery`](https://github.com/PuerkitoBio/goquery)
*   **HTML Content Extraction:** [`go-shiori/go-readability`](https://github.com/go-shiori/go-readability)
*   **HTML to Markdown Conversion:** [`JohannesKaufmann/html-to-markdown`](https://github.com/JohannesKaufmann/html-to-markdown) (with GitHub Flavored Markdown plugin)
*   **CLI Framework:** [`spf13/cobra`](https://github.com/spf13/cobra) for modern command-line interface with subcommands and auto-generated help.
*   **JSON Handling:** Standard Go `encoding/json` package.
*   **Testing:** Standard Go `testing` package with comprehensive unit and integration tests.

## Command-Line Interface

Sitepanda uses a modern CLI structure powered by [Cobra](https://github.com/spf13/cobra) with clear subcommands:

```bash
sitepanda [command] [flags]
```

### Commands

#### `init` - Browser Setup
Downloads and installs browser dependencies:

```bash
sitepanda init [browser]        # Install browser (default: chromium).
sitepanda init chromium        # Install Chromium via Playwright (works on Windows, macOS, Linux).
sitepanda init lightpanda      # Install Lightpanda binary (macOS and Linux only).
                               # On Windows, this command will show an error as Lightpanda is not supported.
```

#### `scrape` - Website Scraping
Scrapes websites and extracts content:

```bash
sitepanda scrape [url] [flags]
```

### Global Flags

These flags work with all commands:

*   `--browser <name>, -b <name>`: Specify the browser to use for scraping (`chromium` or `lightpanda`). Default: `chromium` (or the value of the `SITEPANDA_BROWSER` environment variable if set).
*   `--silent`: Do not print any logs.
*   `--version`: Show version information.

### Scrape Command Flags

*   `--url-file <path>`: Path to a file containing a list of URLs to process (one URL per line). If specified, Sitepanda will process each URL from this file individually. This option overrides the `<url>` argument. When `--url-file` is used, the `--follow-match` option is ignored as crawling beyond the provided URLs is not applicable.
*   `-o, --outfile <path>`: Write the fetched site to a text file. The format is determined by the `--output-format` flag.
*   `-f, --output-format <format>`: Specifies the output format. Supported values are `xml-like` (default), `json`, and `jsonl`.
*   `-m, --match <pattern>`: Only extract content from matched pages (glob pattern, can be specified multiple times). Non-matching pages on the same domain are still crawled for links until the `--limit` is reached (this crawling behavior does not apply when `--url-file` is used).
*   `--follow-match <pattern>`: Only add links matching this glob pattern to the crawl queue (can be specified multiple times). This helps control the scope of the crawl. For example, on a social media site, you might use `--follow-match "/username/**"` to only crawl links related to a specific user. This option is ignored if `--url-file` is used.
*   `--limit <number>`: Stop processing/fetching new pages once this many pages have had their content successfully saved (0 for no limit). If the process is interrupted (Ctrl+C), partial results will be saved.
*   `--content-selector <selector>`: Specify a CSS selector (e.g., `.article-body`) to identify the main content area of a page. If provided, `go-readability` will process only the content of the first matching element; the default HTML pre-filtering (of script, img, etc.) is skipped in this case. If the selector is provided but does not match any elements on the page, Sitepanda will fall back to processing the original, full HTML content without applying the default pre-filtering.
*   `--wait-for-network-idle, -wni`: Wait for network to be idle instead of just `load` (default) when fetching pages. This can be useful for pages that load content dynamically after the initial `load` event.

### Environment Variables

*   `SITEPANDA_BROWSER`: Specifies the default browser to use (`chromium` or `lightpanda`). This can be overridden by the `--browser` or `-b` command-line options.

## Crawling Logic

*   A queue manages URLs to be visited.
*   A set (or map) tracks visited URLs to prevent re-fetching and loops.
*   Links are filtered to ensure they are on the same host as the starting URL and use HTTP/HTTPS.
*   If `--follow-match` patterns are provided, discovered links are further filtered. Only links whose paths match one of these patterns will be added to the queue for crawling.
*   When `--url-file` is used, Sitepanda processes each URL from the file directly. It does not crawl for new links from these pages, and thus the `--follow-match` option is not applied in this mode.
*   Connection to the browser (Chromium via Playwright launch, or Lightpanda via CDP) for robust interaction with dynamic web pages.
*   Page fetching waits for the `load` event by default. If `--wait-for-network-idle` or `-wni` is specified, it waits for the network to become idle.
*   If a `--content-selector` is provided, Sitepanda attempts to extract HTML from the first matching element. This specific HTML is then passed to the readability engine.
*   If no `--content-selector` is provided, Sitepanda performs a pre-filtering step on the full HTML: it removes all `<script>`, `<style>`, `<link>`, `<img>`, and `<video>` tags. The resulting modified HTML is then passed to the readability engine.
*   The `--match` option determines if a page's content is extracted and saved.
*   The `--limit` option stops the entire crawl (fetching, processing, and link extraction from new pages) once the specified number of pages have had their content saved.

## Output Format

Sitepanda supports multiple output formats, controlled by the `--output-format` flag.

1.  **`xml-like` (Default):**
    This is the default format if `--output-format` is not specified.

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

2.  **`json`:**
    A single JSON array containing all page objects. Useful for standard JSON parsing.

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

3.  **`jsonl` (JSON Lines):**
    Each page object is a separate, newline-delimited JSON object. This format is useful for streaming results, as each line can be parsed independently.

    ```json
    {"title":"Page Title 1","url":"http://example.com/page-1","content":"Content for page 1..."}
    {"title":"Page Title 2","url":"http://example.com/page-2","content":"Content for page 2..."}
    ...
    ```

### Shell Redirection

When not using `--outfile`, Sitepanda outputs scraped content to stdout and logs to stderr, allowing clean shell redirection:

```bash
# Redirect only scraped content to file (logs appear in terminal)
sitepanda scrape https://example.com > output.txt

# Redirect content and logs to separate files
sitepanda scrape https://example.com > output.txt 2> logs.txt

# Redirect both to the same file
sitepanda scrape https://example.com > all_output.txt 2>&1

# Suppress logs while redirecting content
sitepanda scrape https://example.com --silent > output.txt
# or
sitepanda scrape https://example.com > output.txt 2>/dev/null
```

## Logging and Error Handling

*   A simple logger outputs `INFO` and `WARN` level messages to **stderr** (standard error stream).
*   The `--silent` flag suppresses all log output.
*   Errors encountered during page fetching or processing are logged. Sitepanda attempts to continue processing other pages if the error is page-specific, but will halt the crawl if critical browser connection errors occur or if the required browser is not installed (guiding the user to run `sitepanda init [browser]`).
*   **Graceful Shutdown**: When the process receives an interrupt signal (Ctrl+C/SIGINT) or termination signal (SIGTERM), Sitepanda will stop crawling new pages and save all successfully scraped content up to that point. This ensures that partial results are not lost during long-running scrapes.
*   **Output Separation**: Scraped content is written to **stdout** (standard output), while logs are written to **stderr**. This allows for clean shell redirection of scraped content without log messages.

## Current Status and Known Issues

**Current Functionality:**
*   CLI with `init [browser]` command for Lightpanda or Chromium setup.
*   CLI flags for URL, url-file, outfile (supports `.json` for JSON output), limit, match, follow-match, content-selector, silent, version, and `browser` (with `-b` shorthand, defaulting to Chromium).
*   CLI flag `--wait-for-network-idle` (with `-wni` shorthand) to control page load waiting strategy.
*   Support for `SITEPANDA_BROWSER` environment variable to set the default browser (Chromium or Lightpanda).
*   Dynamic download, installation, and launching/termination of browser dependencies (Lightpanda or Chromium via Playwright).
*   Launch/control of Chromium via Playwright (default) or connection to Lightpanda via CDP for robust interaction with dynamic web pages.
*   Sequential crawling of same-domain URLs starting from a given URL, using a single browser page instance (when not using `--url-file`).
*   Processing of a list of URLs from a file (when using `--url-file`).
*   URL normalization to handle minor variations.
*   Default HTML pre-filtering (removes script, style, link, img, video tags) when `--content-selector` is not used.
*   Extraction of readable content using `go-readability`, optionally guided by `--content-selector`.
*   Conversion of extracted HTML to Markdown.
*   Output of (Title, URL, Markdown Content) in XML-like text format or JSON format to file, or XML-like text to stdout.
*   `--limit` functionality stops the process once the specified number of pages have had their content saved. 
*   `--match` functionality filters which pages have their content saved.
*   `--follow-match` functionality filters which links are added to the crawl queue (ignored when `--url-file` is used).
*   Graceful shutdown on OS signals with partial results saving.
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
        This command will use Playwright to download and install Chromium. Playwright will install it into a Sitepanda-managed directory:
        *   Linux: e.g., `~/.local/share/sitepanda/playwright_driver/`
        *   macOS: e.g., `~/Library/Application Support/Sitepanda/playwright_driver/`
        *   Windows: e.g., `%LOCALAPPDATA%\Sitepanda\playwright_driver\` (typically `C:\Users\<username>\AppData\Local\Sitepanda\playwright_driver\`)
        This directory is used by Playwright to find its browser binaries. You only need to run this once for Chromium.

    *   **For Lightpanda (optional, macOS and Linux only):**
        ```bash
        sitepanda init lightpanda
        ```
        This command will download the appropriate Lightpanda binary for your OS (Linux or macOS) and install it into a user-specific data directory:
        *   Linux: e.g., `~/.local/share/sitepanda/bin/`
        *   macOS: e.g., `~/Library/Application Support/Sitepanda/bin/`
        You only need to run this once for Lightpanda on a compatible OS, unless you want to re-download it.
        **On Windows, this command will result in an error as Lightpanda is not supported.**

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

## Usage Examples

After installation and initialization:

### Basic Scraping

```bash
# Initialize browser (first time only)
sitepanda init

# Scrape a single URL with Chromium (default)
sitepanda scrape https://example.com

# Scrape with output to file (XML-like format)
sitepanda scrape --outfile output.txt https://example.com

# Scrape with JSON output
sitepanda scrape --output-format json --outfile output.json https://example.com

# Scrape with JSON Lines output
sitepanda scrape --output-format jsonl --outfile output.jsonl https://example.com
```

### Advanced Scraping Options

```bash
# Scrape with content filtering by URL patterns
sitepanda scrape --match "/blog/**" --outfile output.json https://example.com

# Scrape multiple URLs from a file
sitepanda scrape --url-file urls.txt --outfile output.json

# Control crawling scope with follow-match patterns
sitepanda scrape --follow-match "/docs/**" --follow-match "/api/**" \
  --outfile docs.json https://example.com

# Use specific content selector for precise extraction
sitepanda scrape --content-selector ".main-article-body" \
  --outfile output.json https://example.com

# Wait for network idle for dynamic content
sitepanda scrape --wait-for-network-idle --outfile output.json https://example.com
# or use the short form:
sitepanda scrape -wni --outfile output.json https://example.com
```

### Graceful Cancellation

```bash
# Start a large scraping job
sitepanda scrape --limit 100 --outfile results.json https://example.com

# Press Ctrl+C to cancel - partial results will be saved
# The output file will contain all pages successfully scraped before cancellation
```

### Browser Selection

```bash
# Use Lightpanda browser
sitepanda init lightpanda
sitepanda scrape --browser lightpanda --outfile output.json https://example.com

# Use environment variable for browser selection
SITEPANDA_BROWSER=lightpanda sitepanda scrape https://example.com

# Global browser flag (works with all commands)
sitepanda --browser chromium scrape --outfile output.json https://example.com
```

### Help and Information

```bash
# Show general help
sitepanda --help

# Show help for specific commands
sitepanda init --help
sitepanda scrape --help

# Show version
sitepanda --version
```

## Testing

Sitepanda includes comprehensive tests for reliability:

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test ./... -cover

# Run tests in verbose mode
go test ./... -v

# Run specific test packages
go test ./cmd -v                    # Test CLI commands
go test . -run TestProcessHTML      # Test specific functions
```

### Test Structure

- **Unit Tests**: Core functionality (path management, content processing, URL handling)
- **Command Tests**: CLI command validation and flag parsing
- **Integration Tests**: End-to-end command execution
- **Handler Tests**: Browser initialization and scraping logic

## License

Sitepanda is licensed under the [MIT License](LICENSE).

## Development Status

### v0.2.0 - Feature Release

This release introduces explicit output format control.

#### ✨ New Features
*   **Output Format Flag**: Added `--output-format` (or `-f`) flag to `scrape` command to specify output format.
*   **JSONL Format**: Added support for `jsonl` (JSON Lines) as an output format.
*   **Explicit Formatting**: The output format is now explicitly controlled by the new flag, not inferred from the output file extension.

### v0.1.3 - Maintenance Release

This release updates the version number and confirms Windows compatibility.

#### Chore
*   Bump version to 0.1.3
*   Verified Windows compatibility and release configuration.

### v0.1.1, 0.1.2 - Bug Fix Release

This release fixes the cancellation behavior and improves output handling.

#### 🐛 Bug Fixes
*   **Fixed cancellation behavior**: Ctrl+C now properly exits the crawl loop and saves partial results
*   **Improved signal handling**: Added proper loop labeling for graceful shutdown

#### 📖 Documentation
*   **Output redirection**: Clarified that logs go to stderr and content to stdout for clean shell redirection
*   **Shell redirection examples**: Added comprehensive examples for different redirection scenarios

### v0.1.0 - Major Release 🎉

This release represents a significant architectural improvement with breaking changes for better CLI experience.

#### ✨ New Features
*   **Modern CLI with Cobra**: Complete refactor to use `spf13/cobra` for professional command structure
*   **Subcommand Architecture**: Clear separation between `init` and `scrape` commands with proper flag organization
*   **Auto-Generated Help**: Rich help system with examples and shell completion support
*   **Enhanced Testing**: Full test suite including unit tests, integration tests, and command validation

#### 🔄 Breaking Changes
*   **Command Structure**: Changed from `sitepanda [flags] <url>` to `sitepanda scrape [flags] <url>`
*   **Browser Initialization**: Now requires explicit `sitepanda init [browser]` command
*   **Help System**: New structured help with `--help` for each subcommand

#### 📖 Migration Guide
```bash
# Old (v0.0.x)
sitepanda --outfile output.json https://example.com

# New (v0.1.0)
sitepanda init                    # One-time setup
sitepanda scrape --outfile output.json https://example.com
```

### Current Status (v0.1.0)

*   🚀 **Stable CLI**: Production-ready `init [browser]` and `scrape [url]` subcommands
*   🌐 **Dual Browser Support**: Full support for both Chromium (default) and Lightpanda browsers
*   ⚡ **Automated Setup**: Dynamic browser installation and management
*   🛡️ **Robust Validation**: Comprehensive flag support with proper validation and error handling
*   📚 **Auto-Generated Help**: Built-in help system and shell completion support via Cobra
*   🔧 **Flexible Configuration**: Environment variable configuration with CLI override capability
*   ✅ **High Quality**: Comprehensive test coverage across all major functionality

### Future Improvements

*   **Enhanced Error Handling**: More detailed error messages for network and installation issues.
*   **Performance Optimization**: Parallel processing for multiple URLs.
*   **Windows Support**: Initial support for Windows (Chromium only) has been added. Ongoing efforts will focus on comprehensive testing and addressing any platform-specific issues.
*   **Plugin System**: Extensible architecture for custom content processors.
