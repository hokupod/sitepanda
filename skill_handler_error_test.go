package main

import (
	"bytes"
	"errors"
	"testing"
)

func TestInstallSkill_GetHomeDirError(t *testing.T) {
	// Mock env and getHomeDir
	mockEnv := func(key string) string { return "" }
	mockGetHomeDir := func() (string, error) {
		return "", errors.New("mock error")
	}

	input := bytes.NewBufferString("3\n~/custom/path\n")
	output := new(bytes.Buffer)

	err := installSkill(input, output, mockEnv, mockGetHomeDir)

	if err == nil {
		t.Error("Expected error, got nil")
	}
	expectedError := "failed to get user home directory: mock error"
	if err.Error() != expectedError {
		t.Errorf("Expected error %q, got %q", expectedError, err.Error())
	}
}
