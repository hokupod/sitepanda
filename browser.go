package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/playwright-community/playwright-go"
)

// prepareBrowser checks if the specified browser's executable is ready.
// For Chromium, Playwright manages its own executable, so this check is relaxed.
// baseInstallDirForChromium is the directory where Playwright installs drivers and browsers.
func prepareBrowser(browserName string, baseInstallDirForChromium string) (executablePath string, cleanupFunc func(), err error) {
	var execPathAttempt string

	switch browserName {
	case "lightpanda":
		execPathAttempt, err = GetBrowserExecutablePath(browserName)
		if err != nil {
			return "", func() {}, fmt.Errorf("could not determine %s executable path: %w", browserName, err)
		}

		info, statErr := os.Stat(execPathAttempt)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				return "", func() {}, fmt.Errorf("%s executable not found at %s. Please run 'sitepanda init %s' to install it", browserName, execPathAttempt, browserName)
			}
			return "", func() {}, fmt.Errorf("error accessing %s executable at %s: %w", browserName, execPathAttempt, statErr)
		}
		if info.IsDir() {
			return "", func() {}, fmt.Errorf("expected %s executable at %s, but found a directory. Please check the installation or run 'sitepanda init %s' again", browserName, execPathAttempt, browserName)
		}
		logger.Printf("Using %s binary from: %s", browserName, execPathAttempt)
		return execPathAttempt, func() {
			logger.Printf("No specific cleanup needed for Lightpanda binary itself from prepareBrowser.")
		}, nil

	case "chromium":
		// For Chromium, Playwright manages the executable. Assume `playwright.Install` has worked.
		// `GetBrowserExecutablePath` for Chromium may return a hint or error if path detection is complex.
		// If Playwright's `Launch` can find the executable, a strict path is not required here.
		logger.Printf("For Chromium, Playwright is expected to find the executable. Path check in prepareBrowser is primarily for confirmation if `GetBrowserExecutablePath` is implemented for Chromium.")
		// Call to check if path hint logic is working as expected
		_, err = GetBrowserExecutablePath(browserName, baseInstallDirForChromium)
		if err != nil {
			logger.Printf("Note: Could not determine a hint for Chromium executable path via GetBrowserExecutablePath: %v. This is often okay as Playwright handles it.", err)
		}
		// No specific executable path is returned since Playwright auto-detects it.
		// No specific cleanup is needed for Chromium from this function.
		return "", func() {}, nil
	default:
		return "", func() {}, fmt.Errorf("unsupported browser for prepare: %s", browserName)
	}
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func launchBrowserAndGetConnection(browserName string, lightpandaExecutablePath string, baseInstallDirForChromium string, verboseBrowser bool) (
	cmd *exec.Cmd, wsURL string, pwInstance *playwright.Playwright, pwBrowser playwright.Browser, lpStdout *bytes.Buffer, lpStderr *bytes.Buffer, err error) {

	switch browserName {
	case "lightpanda":
		port, errPort := getFreePort()
		if errPort != nil {
			return nil, "", nil, nil, nil, nil, fmt.Errorf("failed to get free port for Lightpanda: %w", errPort)
		}
		stdoutBuf := new(bytes.Buffer)
		stderrBuf := new(bytes.Buffer)
		host := "127.0.0.1"
		actualWsURL := fmt.Sprintf("ws://%s:%d", host, port)

		lightpandaCmd := exec.Command(lightpandaExecutablePath, "serve", "--host", host, "--port", fmt.Sprintf("%d", port))
		lightpandaCmd.Stdout = stdoutBuf
		lightpandaCmd.Stderr = stderrBuf

		if errStart := lightpandaCmd.Start(); errStart != nil {
			return nil, "", nil, nil, stdoutBuf, stderrBuf, fmt.Errorf("failed to start Lightpanda server (command: %s serve --host %s --port %d): %w", lightpandaExecutablePath, host, port, errStart)
		}

		logger.Printf("Launched Lightpanda server (PID: %d) on %s:%d", lightpandaCmd.Process.Pid, host, port)
		logger.Printf("Lightpanda WebSocket debugger URL: %s", actualWsURL)

		// Wait for Lightpanda server to accept connections
		logger.Printf("Waiting for Lightpanda server at %s to become ready...", actualWsURL)
		if errWait := waitForPort(host, port, 10*time.Second); errWait != nil {
			_ = lightpandaCmd.Process.Kill()
			_ = lightpandaCmd.Wait()
			return lightpandaCmd, "", nil, nil, stdoutBuf, stderrBuf, fmt.Errorf("Lightpanda did not become ready in time: %w", errWait)
		}

		// Playwright instance is needed to connect even for Lightpanda
		pwRunInstance, errRun := playwright.Run()
		if errRun != nil {
			_ = lightpandaCmd.Process.Kill()
			_ = lightpandaCmd.Wait()
			return lightpandaCmd, "", nil, nil, stdoutBuf, stderrBuf, fmt.Errorf("could not start playwright for Lightpanda connection: %w", errRun)
		}

		return lightpandaCmd, actualWsURL, pwRunInstance, nil, stdoutBuf, stderrBuf, nil

	case "chromium":
		logger.Println("Launching Chromium via playwright-go...")
		// If PLAYWRIGHT_DRIVER_PATH is set, respect it; otherwise, use the managed directory
		runOpts := playwright.RunOptions{DriverDirectory: baseInstallDirForChromium, Verbose: verboseBrowser}
		pwRunInstance, errRun := playwright.Run(&runOpts)
		if errRun != nil {
			return nil, "", nil, nil, nil, nil, fmt.Errorf("could not start playwright for Chromium (DriverDirectory: %s): %w", baseInstallDirForChromium, errRun)
		}

		// Launch Chromium. Playwright will find the executable in DriverDirectory.
		browser, errLaunch := pwRunInstance.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(true),
		})
		if errLaunch != nil {
			// Cleanup Playwright if browser launch fails
			_ = pwRunInstance.Stop()
			return nil, "", pwRunInstance, nil, nil, nil, fmt.Errorf("could not launch Chromium: %w", errLaunch)
		}
		logger.Println("Chromium launched successfully via playwright-go.")
		return nil, "", pwRunInstance, browser, nil, nil, nil
	default:
		return nil, "", nil, nil, nil, nil, fmt.Errorf("unsupported browser for launch: %s", browserName)
	}
}

// waitForPort waits for a TCP port on a given host to become available for connection.
func waitForPort(host string, port int, timeout time.Duration) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", addr)
}
