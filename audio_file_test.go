package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	waxaudio "github.com/colespringer/waxflow/audio"
	waxcodec "github.com/colespringer/waxflow/codec"
	waxaac "github.com/colespringer/waxflow/codec/aac"
	waxcontainer "github.com/colespringer/waxflow/container"
	"github.com/colespringer/waxflow/container/adts"
	waxmp4 "github.com/colespringer/waxflow/container/mp4"
)

func TestSupportedAudioFiles(t *testing.T) {
	for _, name := range []string{"voice.wav", "music.MP3", "archive.FlAc", "radio.AAC", "raw.adts", "song.m4a", "video.MP4"} {
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

func TestMusicFilenameRegexIncludesAACContainers(t *testing.T) {
	for _, name := range []string{"song-0001.aac", "song-0002.M4A", "song-0003.mp4"} {
		if !MusicfilenameRegex.MatchString(name) {
			t.Errorf("MusicfilenameRegex does not match %q", name)
		}
	}
}

func TestDecodeEmbeddedAACFiles(t *testing.T) {
	format := waxaudio.Format{
		Rate:     44100,
		Channels: 2,
		Layout:   waxaudio.DefaultLayout(2),
		Type:     waxaudio.Float,
		BitDepth: 32,
	}
	encoder, err := waxaac.NewEncoder(format, nil)
	if err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	muxer := adts.NewMuxer(&encoded)
	track := waxcontainer.Track{
		ID:          0,
		Codec:       waxcodec.AACLC,
		CodecConfig: encoder.CodecConfig(),
		Fmt:         format,
		Samples:     4096,
	}
	if err := muxer.Begin([]waxcontainer.Track{track}); err != nil {
		t.Fatal(err)
	}
	var m4a bytes.Buffer
	mp4Muxer := waxmp4.NewMuxer(&m4a, nil)
	mp4Track := track
	mp4Track.Delay = int64(encoder.Delay())
	if err := mp4Muxer.Begin([]waxcontainer.Track{mp4Track}); err != nil {
		t.Fatal(err)
	}
	emit := func(packet waxcodec.Packet) error {
		wrapped := waxcontainer.Packet{Track: 0, Packet: packet}
		if err := muxer.WritePacket(wrapped); err != nil {
			return err
		}
		return mp4Muxer.WritePacket(wrapped)
	}
	for offset := 0; offset < 4096; offset += 1024 {
		buffer := waxaudio.Get(format, 1024)
		buffer.N = 1024
		for i := range 1024 {
			sample := float32(0.3 * math.Sin(2*math.Pi*440*float64(offset+i)/float64(format.Rate)))
			buffer.ChanF(0)[i] = sample
			buffer.ChanF(1)[i] = sample
		}
		if err := encoder.Encode(buffer, emit); err != nil {
			waxaudio.Put(buffer)
			t.Fatal(err)
		}
		waxaudio.Put(buffer)
	}
	trailer, err := encoder.Finish(emit)
	if err != nil {
		t.Fatal(err)
	}
	if err := muxer.End(trailer); err != nil {
		t.Fatal(err)
	}
	if err := mp4Muxer.End(trailer); err != nil {
		t.Fatal(err)
	}

	for _, fixture := range []struct {
		name string
		data []byte
	}{
		{name: "embedded-0001.aac", data: encoded.Bytes()},
		{name: "embedded-0002.m4a", data: m4a.Bytes()},
	} {
		t.Run(filepath.Ext(fixture.name), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), fixture.name)
			if err := os.WriteFile(path, fixture.data, 0600); err != nil {
				t.Fatal(err)
			}
			pcm, err := decodeAudioFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(pcm) < 1000 {
				t.Fatalf("decoded AAC samples = %d, want a non-empty resampled stream", len(pcm))
			}
		})
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
		{name: "AAC", env: "NRLNANNY_TEST_AAC"},
		{name: "M4A", env: "NRLNANNY_TEST_M4A"},
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
