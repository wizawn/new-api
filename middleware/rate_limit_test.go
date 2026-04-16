package middleware

import "testing"

func TestShouldSkipGlobalWebRateLimit(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{path: "/", want: false},
		{path: "/console/chatmock", want: false},
		{path: "/assets/index-abc123.js", want: true},
		{path: "/assets/app.css", want: true},
		{path: "/favicon.ico", want: true},
		{path: "/manifest.webmanifest", want: true},
		{path: "/public/logo.png", want: true},
		{path: "/robots.txt", want: true},
		{path: "/docs/setup", want: false},
		{path: "/some/path/image.SVG", want: true},
	}
	for _, tc := range cases {
		got := shouldSkipGlobalWebRateLimit(tc.path)
		if got != tc.want {
			t.Fatalf("path %q expect %v got %v", tc.path, tc.want, got)
		}
	}
}
