package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestSupportedAudioFiles(t *testing.T) {
	for _, name := range []string{"voice.wav", "music.MP3", "archive.FlAc"} {
		if !isSupportedAudioFile(name) {
			t.Errorf("isSupportedAudioFile(%q) = false", name)
		}
	}
	for _, name := range []string{"voice.ogg", "notes.txt", "wav"} {
		if isSupportedAudioFile(name) {
			t.Errorf("isSupportedAudioFile(%q) = true", name)
		}
	}
}

func TestDownmixAndBitDepthConversion(t *testing.T) {
	got := downmixInterleaved([]int{65536, -65536, 32768, 32768}, 2, 18)
	want := []int{0, 8192}
	if len(got) != len(want) {
		t.Fatalf("downmix length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("downmix[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestResamplePCMTo16K(t *testing.T) {
	input := make([]int, 480)
	for i := range input {
		input[i] = int(math.Sin(2*math.Pi*1000*float64(i)/48000) * 12000)
	}
	output := resamplePCM(input, 48000, playbackSampleRate)
	if len(output) != 160 {
		t.Fatalf("resampled samples = %d, want 160", len(output))
	}
	var peak int
	for _, sample := range output {
		if sample < 0 {
			sample = -sample
		}
		if sample > peak {
			peak = sample
		}
	}
	if peak < 10000 || peak > 13000 {
		t.Fatalf("resampled peak = %d, want approximately 12000", peak)
	}
}

func TestDecodeStereo48KWAV(t *testing.T) {
	const frames = 480
	data := make([]byte, frames*2*2)
	for i := range frames {
		left := int16(math.Sin(2*math.Pi*1000*float64(i)/48000) * 12000)
		binary.LittleEndian.PutUint16(data[i*4:], uint16(left))
		binary.LittleEndian.PutUint16(data[i*4+2:], 0)
	}

	var wav bytes.Buffer
	wav.WriteString("RIFF")
	binary.Write(&wav, binary.LittleEndian, uint32(36+len(data)))
	wav.WriteString("WAVEfmt ")
	binary.Write(&wav, binary.LittleEndian, uint32(16))
	binary.Write(&wav, binary.LittleEndian, uint16(1))
	binary.Write(&wav, binary.LittleEndian, uint16(2))
	binary.Write(&wav, binary.LittleEndian, uint32(48000))
	binary.Write(&wav, binary.LittleEndian, uint32(48000*4))
	binary.Write(&wav, binary.LittleEndian, uint16(4))
	binary.Write(&wav, binary.LittleEndian, uint16(16))
	wav.WriteString("data")
	binary.Write(&wav, binary.LittleEndian, uint32(len(data)))
	wav.Write(data)

	path := filepath.Join(t.TempDir(), "stereo.wav")
	if err := os.WriteFile(path, wav.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	pcm, err := decodeAudioFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(pcm) != 160 {
		t.Fatalf("decoded samples = %d, want 160", len(pcm))
	}
}

// These environment variables let release/packaging jobs validate real media
// fixtures without checking copyrighted audio into this repository.
func TestDecodeCompressedAudioFixtures(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  string
	}{
		{name: "MP3", env: "NRLNANNY_TEST_MP3"},
		{name: "FLAC", env: "NRLNANNY_TEST_FLAC"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := os.Getenv(tc.env)
			if path == "" {
				t.Skip(tc.env + " is not set")
			}
			pcm, err := decodeAudioFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(pcm) == 0 {
				t.Fatal("decoded no samples")
			}
		})
	}
}

func BenchmarkResample48KTo16K(b *testing.B) {
	input := make([]int, 48000)
	for i := range input {
		input[i] = int(math.Sin(2*math.Pi*1000*float64(i)/48000) * 12000)
	}
	b.ResetTimer()
	for range b.N {
		resamplePCM(input, 48000, playbackSampleRate)
	}
}
