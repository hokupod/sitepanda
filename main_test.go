package main

import (
	"testing"
)

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{
			name:   "string shorter than maxLen",
			s:      "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "string equal to maxLen",
			s:      "hello world",
			maxLen: 11,
			want:   "hello world",
		},
		{
			name:   "string longer than maxLen",
			s:      "hello world example",
			maxLen: 11,
			want:   "hello world",
		},
		{
			name:   "maxLen is 0",
			s:      "hello",
			maxLen: 0,
			want:   "",
		},
		{
			name:   "empty string",
			s:      "",
			maxLen: 10,
			want:   "",
		},
		{
			name:   "maxLen is negative",
			s:      "hello",
			maxLen: -1,
			want:   "", // Expect empty string as negative maxLen is treated as 0
		},
		{
			name:   "multibyte characters, shorter",
			s:      "こんにちは",
			maxLen: 10,
			want:   "こんにちは",
		},
		{
			name:   "multibyte characters, truncate",
			s:      "こんにちは世界",
			maxLen: 5,
			want:   "こんにちは",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}
