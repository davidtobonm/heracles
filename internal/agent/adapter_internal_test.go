package agent

import "testing"

func TestPlatformSupported(t *testing.T) {
	t.Parallel()

	if !platformSupported(nil, "windows") {
		t.Error("platformSupported(nil, ...) = false, want true for every platform")
	}
	if !platformSupported([]string{"darwin", "linux"}, "linux") {
		t.Error("platformSupported([darwin linux], linux) = false, want true")
	}
	if platformSupported([]string{"darwin", "linux"}, "windows") {
		t.Error("platformSupported([darwin linux], windows) = true, want false")
	}
}
