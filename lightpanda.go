package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
)

func prepareLightpanda() (executablePath string, cleanupFunc func(), err error) {
	lpPath, err := getLightpandaExecutablePath()
	if err != nil {
		return "", func() {}, fmt.Errorf("could not determine Lightpanda executable path: %w", err)
	}

	info, err := os.Stat(lpPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", func() {}, fmt.Errorf("Lightpanda executable not found at %s. Please run 'sitepanda init' to install it", lpPath)
		}
		return "", func() {}, fmt.Errorf("error accessing Lightpanda executable at %s: %w", lpPath, err)
	}

	if info.IsDir() {
		return "", func() {}, fmt.Errorf("expected Lightpanda executable at %s, but found a directory. Please check the installation or run 'sitepanda init' again", lpPath)
	}

	// On Unix-like systems, os.Stat returning successfully for a file implies it's accessible.
	// The execute permission is set during 'sitepanda init'.

	logger.Printf("Using Lightpanda binary from: %s", lpPath)
	// Cleanup function is now a no-op as we are not managing temporary files for Lightpanda.
	return lpPath, func() {
		logger.Printf("No cleanup needed for externally managed Lightpanda binary at %s", lpPath)
	}, nil
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

func launchLightpandaServer(executablePath string) (cmd *exec.Cmd, webSocketURL string, lpStdout *bytes.Buffer, lpStderr *bytes.Buffer, err error) {
	port, err := getFreePort()
	if err != nil {
		return nil, "", nil, nil, fmt.Errorf("failed to get free port for Lightpanda: %w", err)
	}
	lpStdout = new(bytes.Buffer)
	lpStderr = new(bytes.Buffer)

	host := "127.0.0.1"
	webSocketURL = fmt.Sprintf("ws://%s:%d", host, port)

	cmd = exec.Command(executablePath, "serve", "--host", host, "--port", fmt.Sprintf("%d", port))

	cmd.Stdout = lpStdout
	cmd.Stderr = lpStderr

	if err := cmd.Start(); err != nil {
		return nil, "", lpStdout, lpStderr, fmt.Errorf("failed to start Lightpanda server (command: %s serve --host %s --port %d): %w", executablePath, host, port, err)
	}

	logger.Printf("Launched Lightpanda server (PID: %d) on %s:%d", cmd.Process.Pid, host, port)
	logger.Printf("Lightpanda WebSocket debugger URL: %s", webSocketURL)

	// Do not call cmd.Wait() here; the caller (main) will manage the process lifetime.
	return cmd, webSocketURL, lpStdout, lpStderr, nil
}
