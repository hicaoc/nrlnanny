package main

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestValidateRadioURLSupportsMP3AndM3U8(t *testing.T) {
	for _, rawURL := range []string{
		"https://radio.example/live.mp3",
		"https://radio.example/channel/index.m3u8?token=a=b",
		"http://radio.example:8000/stream",
	} {
		if _, err := validateRadioURL(rawURL); err != nil {
			t.Errorf("validateRadioURL(%q) returned %v", rawURL, err)
		}
	}
}

func TestValidateRadioURLRejectsUnsafeSchemes(t *testing.T) {
	for _, rawURL := range []string{"", "file:///tmp/radio.mp3", "javascript:alert(1)", "radio.example/live.mp3"} {
		if _, err := validateRadioURL(rawURL); err == nil {
			t.Errorf("validateRadioURL(%q) unexpectedly succeeded", rawURL)
		}
	}
}

func TestDecodeATPreservesEqualsInRadioURL(t *testing.T) {
	command := decodeAT(append([]byte{0x01}, []byte("AT+RADIO_ADD=News,https://radio.example/live.m3u8?token=a=b\r\n")...))
	if command == nil {
		t.Fatal("decodeAT returned nil")
	}
	if command.value != "News,https://radio.example/live.m3u8?token=a=b" {
		t.Fatalf("value = %q", command.value)
	}
}

func TestIsHLSURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://radio.example/live.m3u8?token=a=b", true},
		{"https://radio.example/LIVE.M3U8", true},
		{"https://radio.example/play?format=m3u8", true},
		{"https://radio.example/live.mp3", false},
	}
	for _, test := range tests {
		if got := isHLSURL(test.url); got != test.want {
			t.Errorf("isHLSURL(%q) = %v, want %v", test.url, got, test.want)
		}
	}
}

func TestRadioPCMWriterProducesNRLFrames(t *testing.T) {
	drainRadioPCM()
	t.Cleanup(drainRadioPCM)
	writer := newRadioPCMWriter(context.Background(), 16000)
	samples := make([]int, opusFrameSamples+64)
	for i := range samples {
		samples[i] = i - 100
	}
	if err := writer.Write(samples); err != nil {
		t.Fatal(err)
	}
	select {
	case frames := <-radioPCM:
		if len(frames) != 1 || len(frames[0]) != opusFrameSamples {
			t.Fatalf("radio frame shape = %d x %d, want 1 x %d", len(frames), len(frames[0]), opusFrameSamples)
		}
	default:
		t.Fatal("radio PCM writer did not emit a complete frame")
	}
}

// TestHLSRadioFixture lets release jobs and field diagnostics validate a live
// station without baking an unstable third-party URL into the test suite.
func TestHLSRadioFixture(t *testing.T) {
	rawURL := os.Getenv("NRLNANNY_TEST_HLS")
	if rawURL == "" {
		t.Skip("NRLNANNY_TEST_HLS is not set")
	}
	drainRadioPCM()
	t.Cleanup(drainRadioPCM)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- streamHLSRadio(ctx, rawURL) }()
	framesSeen := 0
	peak := 0
	for {
		select {
		case frames := <-radioPCM:
			if len(frames) != 1 || len(frames[0]) != opusFrameSamples {
				t.Fatalf("radio frame shape = %d, want one %d-sample frame", len(frames), opusFrameSamples)
			}
			framesSeen++
			for _, sample := range frames[0] {
				if sample < 0 {
					sample = -sample
				}
				if sample > peak {
					peak = sample
				}
			}
			if framesSeen >= 25 {
				if peak < 32 {
					t.Fatalf("HLS stream PCM is effectively silent (peak %d)", peak)
				}
				t.Logf("received %d audible PCM frames, peak %d", framesSeen, peak)
				return
			}
		case err := <-errCh:
			t.Fatalf("HLS stream ended before producing PCM: %v", err)
		case <-ctx.Done():
			t.Fatal("HLS stream produced no audible PCM within 20 seconds")
		}
	}
}
