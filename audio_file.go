package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-audio/wav"
	"github.com/hajimehoshi/go-mp3"
	"github.com/mewkiz/flac"

	waxaudio "github.com/colespringer/waxflow/audio"
	waxcodec "github.com/colespringer/waxflow/codec"
	waxaac "github.com/colespringer/waxflow/codec/aac"
	waxcontainer "github.com/colespringer/waxflow/container"
	"github.com/colespringer/waxflow/container/adts"
	"github.com/colespringer/waxflow/container/mp4"
)

const playbackSampleRate = 16000

func isSupportedAudioFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".wav", ".mp3", ".flac", ".aac", ".adts", ".m4a", ".mp4":
		return true
	default:
		return false
	}
}

// decodeAudioFile converts supported media to mono 16-bit PCM at 16 kHz.
// Keeping one internal format means Opus can use the source directly while
// G711 only needs one final 16 kHz -> 8 kHz conversion.
func decodeAudioFile(path string) ([]int, error) {
	var (
		pcm        []int
		sampleRate int
		err        error
	)

	switch strings.ToLower(filepath.Ext(path)) {
	case ".wav":
		pcm, sampleRate, err = decodeWAVFile(path)
	case ".mp3":
		pcm, sampleRate, err = decodeMP3File(path)
	case ".flac":
		pcm, sampleRate, err = decodeFLACFile(path)
	case ".aac", ".adts":
		pcm, sampleRate, err = decodeADTSFile(path)
	case ".m4a", ".mp4":
		pcm, sampleRate, err = decodeMP4AACFile(path)
	default:
		return nil, fmt.Errorf("unsupported audio format %q", filepath.Ext(path))
	}
	if err != nil {
		return nil, err
	}
	if len(pcm) == 0 {
		return nil, fmt.Errorf("audio file contains no samples: %s", path)
	}
	return resamplePCM(pcm, sampleRate, playbackSampleRate), nil
}

func decodeWAVFile(path string) ([]int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	decoder := wav.NewDecoder(f)
	if !decoder.IsValidFile() {
		return nil, 0, fmt.Errorf("invalid WAV file: %s", path)
	}
	buf, err := decoder.FullPCMBuffer()
	if err != nil {
		return nil, 0, fmt.Errorf("decode WAV %s: %w", path, err)
	}
	if buf == nil || buf.Format == nil {
		return nil, 0, fmt.Errorf("decode WAV %s: missing PCM format", path)
	}
	channels := buf.Format.NumChannels
	if channels < 1 || decoder.SampleRate < 1 || decoder.BitDepth < 1 {
		return nil, 0, fmt.Errorf("invalid WAV format: %s", path)
	}
	if decoder.BitDepth == 8 {
		// WAV stores 8-bit PCM unsigned; all wider PCM sample sizes are signed.
		for i := range buf.Data {
			buf.Data[i] -= 128
		}
	}
	return downmixInterleaved(buf.Data, channels, int(decoder.BitDepth)), int(decoder.SampleRate), nil
}

func decodeMP3File(path string) ([]int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	decoder, err := mp3.NewDecoder(f)
	if err != nil {
		return nil, 0, fmt.Errorf("decode MP3 %s: %w", path, err)
	}
	data, err := io.ReadAll(decoder)
	if err != nil {
		return nil, 0, fmt.Errorf("read MP3 %s: %w", path, err)
	}
	// go-mp3 always produces signed 16-bit little-endian stereo PCM.
	frames := len(data) / 4
	mono := make([]int, frames)
	for i := range frames {
		left := int(int16(binary.LittleEndian.Uint16(data[i*4:])))
		right := int(int16(binary.LittleEndian.Uint16(data[i*4+2:])))
		mono[i] = (left + right) / 2
	}
	return mono, decoder.SampleRate(), nil
}

func decodeFLACFile(path string) ([]int, int, error) {
	stream, err := flac.ParseFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("decode FLAC %s: %w", path, err)
	}
	defer stream.Close()

	info := stream.Info
	if info == nil || info.SampleRate == 0 || info.NChannels == 0 || info.BitsPerSample == 0 {
		return nil, 0, fmt.Errorf("invalid FLAC format: %s", path)
	}
	estimatedSamples := info.NSamples
	if oneMinute := uint64(info.SampleRate) * 60; estimatedSamples > oneMinute {
		estimatedSamples = oneMinute
	}
	capacity := int(estimatedSamples)
	mono := make([]int, 0, capacity)
	for {
		frame, err := stream.ParseNext()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("decode FLAC frame %s: %w", path, err)
		}
		if len(frame.Subframes) == 0 {
			continue
		}
		count := len(frame.Subframes[0].Samples)
		for i := 0; i < count; i++ {
			var sum int64
			for _, subframe := range frame.Subframes {
				if i < len(subframe.Samples) {
					sum += int64(subframe.Samples[i])
				}
			}
			value := sum / int64(len(frame.Subframes))
			mono = append(mono, pcmTo16Bit(value, int(info.BitsPerSample)))
		}
	}
	return mono, int(info.SampleRate), nil
}

func decodeADTSFile(path string) ([]int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	source, err := waxcontainer.FileSource(f)
	if err != nil {
		return nil, 0, fmt.Errorf("open AAC %s: %w", path, err)
	}
	demuxer, err := adts.NewDemuxer(source, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("parse AAC/ADTS %s: %w", path, err)
	}
	return decodeAACPackets(demuxer, path)
}

func decodeMP4AACFile(path string) ([]int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	source, err := waxcontainer.FileSource(f)
	if err != nil {
		return nil, 0, fmt.Errorf("open M4A/MP4 %s: %w", path, err)
	}
	demuxer, err := mp4.NewDemuxer(source, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("parse M4A/MP4 %s: %w", path, err)
	}
	return decodeAACPackets(demuxer, path)
}

func decodeAACPackets(demuxer waxcontainer.Demuxer, path string) ([]int, int, error) {
	tracks := demuxer.Tracks()
	if len(tracks) != 1 || tracks[0].Codec != waxcodec.AACLC {
		return nil, 0, fmt.Errorf("M4A/MP4 file does not contain a supported AAC-LC audio track: %s", path)
	}
	track := tracks[0]
	cfg, err := waxaac.ParseASC(track.CodecConfig)
	if err != nil {
		return nil, 0, fmt.Errorf("parse AAC configuration %s: %w", path, err)
	}
	format, err := cfg.Format()
	if err != nil {
		return nil, 0, fmt.Errorf("read AAC format %s: %w", path, err)
	}
	decoder, err := waxaac.NewDecoder(cfg, format)
	if err != nil {
		return nil, 0, fmt.Errorf("initialize embedded AAC decoder %s: %w", path, err)
	}
	defer decoder.Release()

	capacity := 0
	if track.Samples > 0 {
		// Preallocation is only an optimization. Cap it so an untrusted MP4
		// duration cannot force a huge allocation before packets are parsed.
		estimated := track.Samples
		if oneMinute := int64(format.Rate) * 60; estimated > oneMinute {
			estimated = oneMinute
		}
		capacity = int(estimated)
	}
	mono := make([]int, 0, capacity)
	var packet waxcontainer.Packet
	for {
		err := demuxer.ReadPacket(&packet)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("read AAC packet %s: %w", path, err)
		}
		if err := decoder.Decode(packet.Data, func(pcm *waxaudio.Buffer) error {
			mono = append(mono, downmixFloatPCM(pcm)...)
			return nil
		}); err != nil {
			return nil, 0, fmt.Errorf("decode AAC packet %s: %w", path, err)
		}
	}

	// M4A edit lists carry encoder delay and end padding. ADTS has no such
	// signaling, so both values remain zero for raw .aac files.
	start := track.Delay
	if start < 0 {
		start = 0
	}
	if start > int64(len(mono)) {
		start = int64(len(mono))
	}
	padding := track.Padding
	if padding < 0 {
		padding = 0
	}
	if padding > int64(len(mono))-start {
		padding = int64(len(mono)) - start
	}
	end := int64(len(mono)) - padding
	if track.SamplesExact && track.Samples >= 0 && start+track.Samples < end {
		end = start + track.Samples
	}
	return mono[start:end], format.Rate, nil
}

func downmixFloatPCM(pcm *waxaudio.Buffer) []int {
	if pcm == nil || pcm.N <= 0 || pcm.Fmt.Channels <= 0 {
		return nil
	}
	mono := make([]int, pcm.N)
	for i := range mono {
		var sum float64
		for channel := 0; channel < pcm.Fmt.Channels; channel++ {
			sum += float64(pcm.ChanF(channel)[i])
		}
		mono[i] = clampPCM(int(math.Round(sum * 32767 / float64(pcm.Fmt.Channels))))
	}
	return mono
}

func downmixInterleaved(input []int, channels, bits int) []int {
	if channels < 1 {
		return nil
	}
	frames := len(input) / channels
	mono := make([]int, frames)
	for frame := range frames {
		var sum int64
		for channel := 0; channel < channels; channel++ {
			sum += int64(input[frame*channels+channel])
		}
		mono[frame] = pcmTo16Bit(sum/int64(channels), bits)
	}
	return mono
}

func pcmTo16Bit(value int64, bits int) int {
	if bits > 16 {
		value >>= bits - 16
	} else if bits < 16 {
		value <<= 16 - bits
	}
	if value > 32767 {
		return 32767
	}
	if value < -32768 {
		return -32768
	}
	return int(value)
}

// resamplePCM uses a windowed-sinc low-pass filter. This avoids the aliasing
// that a simple sample drop would introduce when 44.1/48 kHz media is reduced
// to the 16 kHz voice pipeline.
func resamplePCM(input []int, sourceRate, targetRate int) []int {
	if len(input) == 0 || sourceRate <= 0 || targetRate <= 0 {
		return nil
	}
	if sourceRate == targetRate {
		return append([]int(nil), input...)
	}

	outputLen := int(math.Round(float64(len(input)) * float64(targetRate) / float64(sourceRate)))
	output := make([]int, outputLen)
	const radius = 16
	const taps = radius * 2
	cutoff := 0.47
	if targetRate < sourceRate {
		cutoff *= float64(targetRate) / float64(sourceRate)
	}

	common := greatestCommonDivisor(sourceRate, targetRate)
	sourceStep := sourceRate / common
	phases := targetRate / common
	kernels := make([][taps]float64, phases)
	for phase := range phases {
		fraction := float64(phase) / float64(phases)
		for tap := range taps {
			distance := float64(tap-radius+1) - fraction
			x := 2 * cutoff * distance
			sinc := 1.0
			if x != 0 {
				sinc = math.Sin(math.Pi*x) / (math.Pi * x)
			}
			window := 0.5 * (1 + math.Cos(math.Pi*distance/float64(radius)))
			kernels[phase][tap] = 2 * cutoff * sinc * window
		}
	}

	for outIndex := range output {
		positionNumerator := outIndex * sourceStep
		center := positionNumerator / phases
		kernel := kernels[positionNumerator%phases]
		var weighted, weightSum float64
		for tap, weight := range kernel {
			sampleIndex := center + tap - radius + 1
			if sampleIndex < 0 || sampleIndex >= len(input) {
				continue
			}
			weighted += float64(input[sampleIndex]) * weight
			weightSum += weight
		}
		if weightSum != 0 {
			output[outIndex] = clampPCM(int(math.Round(weighted / weightSum)))
		}
	}
	return output
}

func greatestCommonDivisor(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func clampPCM(sample int) int {
	if sample > 32767 {
		return 32767
	}
	if sample < -32768 {
		return -32768
	}
	return sample
}
