# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**Project Version**: v0.1.2 - Modern CLI architecture with Cobra framework

## Common Development Commands

```bash
# Build the project
go build .

# Run all tests with coverage
go test ./... -cover

# Run tests in verbose mode
go test ./... -v

# Run specific test packages
go test ./cmd -v                # Test CLI commands
go test . -run TestFunctionName # Run specific test

# Install and run locally
go install . && sitepanda --version

# Test CLI commands manually
./sitepanda --help
./sitepanda init --help
./sitepanda scrape --help

# Format code
go fmt ./...

# Check for common issues
go vet ./...

# Run linting (if golangci-lint is available)
golangci-lint run
```

## Architecture Overview

Sitepanda is a CLI web scraper written in Go that extracts readable content from websites and converts it to Markdown. As of v0.1.0, the architecture has been completely refactored to use Cobra for modern CLI structure, representing a major architectural milestone.

### CLI Architecture (Cobra-based)

- **main.go**: Simple entry point that sets up handlers and executes Cobra commands
- **cmd/**: Cobra command structure
  - **cmd/root.go**: Root command with global flags and version handling
  - **cmd/init.go**: Browser installation subcommand
  - **cmd/scrape.go**: Website scraping subcommand with all scraping flags
  - **cmd/cmd_test.go**: Comprehensive tests for CLI commands

### Core Components

- **crawler.go**: Core crawling logic with queue management, URL filtering, and page processing orchestration
- **browser.go**: Browser abstraction layer supporting Chromium (via Playwright, on Windows/macOS/Linux) and Lightpanda (via CDP, on macOS/Linux only).
- **processor.go**: Content extraction pipeline using go-readability and HTML-to-Markdown conversion
- **lightpanda.go**: Lightpanda-specific server management and process control (relevant for macOS/Linux).
- **paths.go**: Cross-platform path management for browser installations and data directories (supports Windows, macOS, Linux).
- **fetcher.go**: HTTP fetching utilities and URL normalization

### Handler Architecture

- **init_handler.go**: Browser installation logic (called by cmd/init.go)
- **scraping_handler.go**: Main scraping logic (called by cmd/scrape.go)
- **utils.go**: Shared utilities, constants, and logger configuration

### Browser Architecture

The tool supports two browser backends:
- **Chromium**: Managed by Playwright, automatically downloaded and controlled via `playwright-go`. This is the default and recommended browser for all supported platforms (Windows, macOS, Linux).
- **Lightpanda**: External binary, downloaded and launched as a separate process, controlled via CDP. Supported on macOS and Linux only. **Not supported on Windows.**

The `browser.go` file provides a unified interface (`prepareBrowser`, `launchBrowserAndGetConnection`) that abstracts these differences, though Lightpanda-related paths will error out on Windows within `init_handler.go`.

### Content Processing Pipeline

1. **HTML Fetching**: Pages are loaded using the selected browser engine
2. **Content Selection**: Optional CSS selector filtering before processing
3. **Pre-filtering**: Removes script, style, link, img, and video tags (when no content selector is used)
4. **Readability Extraction**: Uses `go-readability` to extract main content
5. **Markdown Conversion**: Converts extracted HTML to Markdown with GitHub Flavored Markdown support

### Graceful Cancellation

The crawler supports graceful shutdown with partial results preservation:
- **Signal Handling**: Listens for SIGINT (Ctrl+C) and SIGTERM signals in `scraping_handler.go`
- **Context Cancellation**: Uses Go's context package to propagate cancellation through the crawler
- **Partial Results**: When cancelled, the crawler:
  - Stops fetching new pages immediately
  - Breaks out of the crawl loop gracefully
  - Saves all successfully scraped pages to the output file
  - Logs the number of partial results saved
- **Implementation**: The `Crawler` struct exposes a `Cancel()` method that cancels its internal context

### URL Management

- URL normalization and validation in `fetcher.go`
- Glob pattern matching for both content filtering (`--match`) and crawl scoping (`--follow-match`)
- Queue-based crawling with visited URL tracking to prevent loops
- Same-domain restriction for discovered links

### Output Formats

- **XML-like text format** (default): Custom structured output with `<page>`, `<title>`, `<url>`, `<content>` tags
- **JSON format**: Array of page objects (triggered by `.json` file extension in `--outfile`)
- **Output streams**: Content goes to stdout, logs go to stderr (allows clean shell redirection)

## Testing Strategy

The codebase includes comprehensive testing with multiple test types:

### Test Structure

- **cmd/cmd_test.go**: CLI command validation, flag parsing, and subcommand behavior
- **handlers_test.go**: Handler function testing with mocking where appropriate
- **integration_test.go**: End-to-end CLI binary execution tests
- **utils_test.go**: Utility function tests (truncateString, logger, constants)
- **[original]_test.go**: Existing tests for core functionality (crawler, processor, paths, browser)

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -cover

# Run specific test packages
go test ./cmd -v                    # CLI commands
go test . -run TestProcessHTML      # Specific functions
go test . -run TestCLI              # Integration tests
```

### Test Coverage

- **cmd package**: ~47% coverage (CLI commands and flags)
- **main package**: ~23% coverage (core functionality)
- Focus areas: Command validation, flag handling, error cases, utility functions

## Browser Management

The `init` subcommand downloads and installs browser dependencies:
- `sitepanda init` or `sitepanda init chromium` (default): Downloads Chromium via Playwright. Works on Windows, macOS, and Linux.
- `sitepanda init lightpanda`: Downloads Lightpanda binary to user data directory. This command is only functional on macOS and Linux. On Windows, it will produce an error message stating that Lightpanda is not supported.

Browser executables and Playwright drivers are stored in platform-specific locations (Windows, macOS, Linux) managed by `paths.go`.

## Platform Support

Sitepanda officially supports the following platforms:
- Windows (x86_64)
- macOS (arm64, and likely x86_64 via Rosetta 2)
- Linux (x86_64)

**Browser Support by Platform:**
- **Windows**: Chromium only.
- **macOS**: Chromium (default), Lightpanda (optional).
- **Linux**: Chromium (default), Lightpanda (optional).

## CLI Command Structure (v0.1.1)

**Important**: This is a breaking change from v0.0.x. The CLI now uses subcommands instead of direct URL arguments.

### Root Command (`sitepanda`)
- Global flags: `--browser`, `--silent`, `--version`
- Shows help when run without subcommands
- No longer accepts URL as direct argument

### Init Subcommand (`sitepanda init [browser]`)
- **Required first step**: Must be run before scraping
- Handles browser installation (chromium/lightpanda)
- Validates browser arguments with proper error messages
- Uses `init_handler.go` for actual installation logic

### Scrape Subcommand (`sitepanda scrape [url]`)
- **Main functionality**: All scraping moved to this subcommand
- URL argument handling with validation
- All original flags preserved: `--outfile`, `--match`, `--url-file`, etc.
- Uses `scraping_handler.go` for scraping logic
- Maintains backward compatibility for all scraping options

## Environment Variables

- `SITEPANDA_BROWSER`: Sets default browser ("chromium" or "lightpanda")
- Detected and used in `cmd/root.go` during flag initialization
- Can be overridden by `--browser` flag

## Development Notes

### v0.1.2 Changes
- **Feature**: Added shell completion support with automatic Homebrew configuration
- **Enhancement**: Improved shell redirection capabilities and output stream separation
- **Documentation**: Updated examples for shell completion setup and usage

### v0.1.1 Changes
- **Bug fix**: Fixed cancellation behavior to properly exit crawl loop and save partial results
- **Improvement**: Added proper loop labeling for graceful shutdown handling
- **Clarification**: Logger outputs to stderr (not stdout) to ensure clean shell redirection
- **Documentation**: Added shell redirection examples and output separation explanation

### v0.1.0 Architectural Changes
- **Major refactor**: Moved from flag-based to Cobra subcommand architecture
- **Breaking changes**: CLI interface completely redesigned for better UX
- **Test coverage**: Added comprehensive test suite for new architecture
- **Handler pattern**: Separated CLI logic from business logic for maintainability

### Adding New Commands
1. Create new file in `cmd/` directory (e.g., `cmd/newcommand.go`)
2. Define cobra.Command with appropriate flags and validation
3. Add to root command in `init()` function
4. Create handler in main package if complex logic is needed
5. Add comprehensive tests in `cmd/cmd_test.go`
6. Update documentation (README.md and this file)

### Modifying Existing Commands
- **CLI logic**: All command definition and flag parsing in `cmd/` package
- **Business logic**: Implementation in handler files (main package)
- **Testing**: Must cover both command parsing and handler execution
- **Validation**: Use Cobra's built-in validation features

### Common Patterns
- Use `cobra.MaximumNArgs(1)` for single URL arguments
- Use `cobra.RunE` instead of `Run` for commands that can return errors
- Global flags are defined in `cmd/root.go`
- Command-specific flags are defined in respective command files
- Handler functions are called from command Run/RunE functions
- Always provide examples in command Long descriptions

### Testing Strategy
- **Unit tests**: Each command's flag parsing and validation
- **Integration tests**: End-to-end command execution
- **Handler tests**: Business logic with appropriate mocking
- **Error cases**: Both CLI validation and handler error scenarios