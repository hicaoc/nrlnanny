package main

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/kazzmir/opus-go/opus"
)

const (
	opusSampleRate    = 16000
	opusChannels      = 1
	opusFrameSamples  = 320 // 20 ms at 16 kHz
	opusBitrate       = 36000
	opusPacketMax     = 1275
	opusMaxStreams    = 64
	receiveSampleRate = 8000
)

type opusEncoderState struct {
	mu      sync.Mutex
	encoder *opus.Encoder
	packet  []byte
}

var sendOpusEncoder opusEncoderState

func newConfiguredOpusEncoder() (*opus.Encoder, error) {
	encoder, err := opus.NewEncoder(opusSampleRate, opusChannels, opus.ApplicationVoIP)
	if err != nil {
		return nil, err
	}
	if err := encoder.SetBitrate(opusBitrate); err != nil {
		encoder.Close()
		return nil, err
	}
	if err := encoder.SetVBR(true); err != nil {
		encoder.Close()
		return nil, err
	}
	if err := encoder.SetComplexity(10); err != nil {
		encoder.Close()
		return nil, err
	}
	return encoder, nil
}

func encodeOpusVoice(pcm []int16, reset bool) ([]byte, error) {
	if len(pcm) != opusFrameSamples {
		return nil, fmt.Errorf("opus input has %d samples, want %d", len(pcm), opusFrameSamples)
	}

	sendOpusEncoder.mu.Lock()
	defer sendOpusEncoder.mu.Unlock()

	if reset && sendOpusEncoder.encoder != nil {
		_ = sendOpusEncoder.encoder.Close()
		sendOpusEncoder.encoder = nil
	}
	if sendOpusEncoder.encoder == nil {
		encoder, err := newConfiguredOpusEncoder()
		if err != nil {
			return nil, err
		}
		sendOpusEncoder.encoder = encoder
		sendOpusEncoder.packet = make([]byte, opusPacketMax)
	}

	n, err := sendOpusEncoder.encoder.Encode(pcm, opusFrameSamples, sendOpusEncoder.packet)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), sendOpusEncoder.packet[:n]...), nil
}

type opusDecoderState struct {
	mu       sync.Mutex
	decoder  *opus.Decoder
	pcm      []int16
	lastUsed time.Time
}

var receiveOpusDecoders = struct {
	sync.Mutex
	items map[string]*opusDecoderState
}{items: make(map[string]*opusDecoderState)}

func decoderForOpusStream(key string, now time.Time) (*opusDecoderState, error) {
	receiveOpusDecoders.Lock()
	defer receiveOpusDecoders.Unlock()

	if state := receiveOpusDecoders.items[key]; state != nil {
		state.mu.Lock()
		return state, nil
	}
	decoder, err := opus.NewDecoder(receiveSampleRate, opusChannels)
	if err != nil {
		return nil, err
	}
	if len(receiveOpusDecoders.items) >= opusMaxStreams {
		var oldestKey string
		var oldestTime time.Time
		for streamKey, state := range receiveOpusDecoders.items {
			state.mu.Lock()
			lastUsed := state.lastUsed
			state.mu.Unlock()
			if oldestKey == "" || lastUsed.Before(oldestTime) {
				oldestKey = streamKey
				oldestTime = lastUsed
			}
		}
		if old := receiveOpusDecoders.items[oldestKey]; old != nil {
			old.mu.Lock()
			_ = old.decoder.Close()
			old.mu.Unlock()
			delete(receiveOpusDecoders.items, oldestKey)
		}
	}
	state := &opusDecoderState{
		decoder:  decoder,
		pcm:      make([]int16, receiveSampleRate*120/1000),
		lastUsed: now,
	}
	receiveOpusDecoders.items[key] = state
	state.mu.Lock()
	return state, nil
}

func decodeOpusVoice(nrl *NRL21packet) ([]int16, error) {
	if nrl == nil || len(nrl.DATA) == 0 {
		return nil, errors.New("empty opus packet")
	}
	now := time.Now()
	key := fmt.Sprintf("%s-%d", nrl.CallSign, nrl.SSID)
	state, err := decoderForOpusStream(key, now)
	if err != nil {
		return nil, err
	}

	defer state.mu.Unlock()
	if now.Sub(state.lastUsed) > time.Second {
		_ = state.decoder.Close()
		state.decoder, err = opus.NewDecoder(receiveSampleRate, opusChannels)
		if err != nil {
			return nil, err
		}
	}
	state.lastUsed = now
	samples, err := state.decoder.Decode(nrl.DATA, state.pcm, len(state.pcm), false)
	if err != nil {
		return nil, err
	}
	if samples <= 0 || samples > len(state.pcm) {
		return nil, fmt.Errorf("invalid decoded opus sample count %d", samples)
	}
	return append([]int16(nil), state.pcm[:samples]...), nil
}

func upsample8To16(input []int) []int {
	output := make([]int, len(input)*2)
	for i, sample := range input {
		next := sample
		if i+1 < len(input) {
			next = input[i+1]
		}
		output[i*2] = sample
		output[i*2+1] = (sample + next) / 2
	}
	return output
}

func downsample16To8Int(input []int) []int {
	output := make([]int, len(input)/2)
	for i := range output {
		output[i] = (input[i*2] + input[i*2+1]) / 2
	}
	return output
}

func intsToInt16WithVolume(input []int, volume float64) []int16 {
	output := make([]int16, len(input))
	for i, sample := range input {
		output[i] = int16(AdjustVolumeInt(sample, volume))
	}
	return output
}
