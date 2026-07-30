package main

import (
	"math"
	"testing"
)

func TestOpusVoiceRoundTrip(t *testing.T) {
	pcm := make([]int16, opusFrameSamples)
	for i := range pcm {
		pcm[i] = int16(math.Sin(2*math.Pi*1000*float64(i)/opusSampleRate) * 12000)
	}

	packet, err := encodeOpusVoice(pcm, true)
	if err != nil {
		t.Fatalf("encodeOpusVoice() error = %v", err)
	}
	if len(packet) == 0 || len(packet) > opusPacketMax {
		t.Fatalf("encoded packet length = %d", len(packet))
	}

	decoded, err := decodeOpusVoice(&NRL21packet{
		Type:     8,
		CallSign: "N0TEST",
		SSID:     1,
		DATA:     packet,
	})
	if err != nil {
		t.Fatalf("decodeOpusVoice() error = %v", err)
	}
	if len(decoded) != 160 {
		t.Fatalf("decoded playback samples = %d, want 160", len(decoded))
	}
}

func TestVoiceResamplingFrameSizes(t *testing.T) {
	pcm8 := make([]int, 160)
	for i := range pcm8 {
		pcm8[i] = i * 10
	}
	pcm16 := upsample8To16(pcm8)
	if len(pcm16) != opusFrameSamples {
		t.Fatalf("upsampled samples = %d, want %d", len(pcm16), opusFrameSamples)
	}
	if got := downsample16To8Int(pcm16); len(got) != 160 {
		t.Fatalf("downsampled samples = %d, want 160", len(got))
	}
}

func TestEncodeOpusVoiceRejectsWrongFrameSize(t *testing.T) {
	if _, err := encodeOpusVoice(make([]int16, 160), true); err == nil {
		t.Fatal("encodeOpusVoice() accepted an 8 kHz frame")
	}
}

func BenchmarkOpusEncode20ms(b *testing.B) {
	pcm := make([]int16, opusFrameSamples)
	for i := range pcm {
		pcm[i] = int16(math.Sin(2*math.Pi*1000*float64(i)/opusSampleRate) * 12000)
	}
	if _, err := encodeOpusVoice(pcm, true); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		if _, err := encodeOpusVoice(pcm, false); err != nil {
			b.Fatal(err)
		}
	}
}

func TestIntsToInt16WithVolumeClips(t *testing.T) {
	got := intsToInt16WithVolume([]int{30000, -30000}, 2)
	if got[0] != 32767 || got[1] != -32768 {
		t.Fatalf("volume/clipping result = %v", got)
	}
}

func BenchmarkEncodeOpusVoice(b *testing.B) {
	pcm := make([]int16, opusFrameSamples)
	for i := range pcm {
		pcm[i] = int16(math.Sin(2*math.Pi*1000*float64(i)/opusSampleRate) * 12000)
	}
	if _, err := encodeOpusVoice(pcm, true); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := encodeOpusVoice(pcm, false); err != nil {
			b.Fatal(err)
		}
	}
}
