package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bluenviron/gohlslib/v2"
	hlscodecs "github.com/bluenviron/gohlslib/v2/pkg/codecs"
	"github.com/colespringer/waxflow/audio"
	waxaac "github.com/colespringer/waxflow/codec/aac"
	"github.com/hajimehoshi/go-mp3"
)

// RadioStation is a saved internet radio source. MP3 and AAC-LC HLS streams
// are decoded inside this process; no external media program is required.
type RadioStation struct {
	ID   string `yaml:"ID" json:"id"`
	Name string `yaml:"Name" json:"name"`
	URL  string `yaml:"URL" json:"url"`
}

var radioState = struct {
	sync.RWMutex
	cancel context.CancelFunc
	status string
}{status: "stopped"}

func validateRadioURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("radio URL must be a valid http:// or https:// address")
	}
	return parsed.String(), nil
}

func saveRadioStation(id, name, rawURL string) (RadioStation, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return RadioStation{}, fmt.Errorf("radio station name is required")
	}
	if len(name) > 100 {
		return RadioStation{}, fmt.Errorf("radio station name is too long")
	}
	cleanURL, err := validateRadioURL(rawURL)
	if err != nil {
		return RadioStation{}, err
	}

	confMu.Lock()
	defer confMu.Unlock()
	if id == "" {
		id = "radio-" + strconv.FormatInt(time.Now().UnixNano(), 36)
		station := RadioStation{ID: id, Name: name, URL: cleanURL}
		conf.System.RadioStations = append(conf.System.RadioStations, station)
		return station, nil
	}
	for i := range conf.System.RadioStations {
		if conf.System.RadioStations[i].ID == id {
			conf.System.RadioStations[i].Name = name
			conf.System.RadioStations[i].URL = cleanURL
			return conf.System.RadioStations[i], nil
		}
	}
	return RadioStation{}, fmt.Errorf("radio station not found")
}

func deleteRadioStation(id string) error {
	if id == "" {
		return fmt.Errorf("radio station id is required")
	}
	confMu.Lock()
	found := false
	active := conf.System.RadioActiveID == id
	next := make([]RadioStation, 0, len(conf.System.RadioStations))
	for _, station := range conf.System.RadioStations {
		if station.ID == id {
			found = true
			continue
		}
		next = append(next, station)
	}
	if found {
		conf.System.RadioStations = next
		if active {
			conf.System.RadioActiveID = ""
			conf.System.RadioPlaying = false
		}
	}
	confMu.Unlock()
	if !found {
		return fmt.Errorf("radio station not found")
	}
	if active {
		stopRadioProcess()
	}
	return nil
}

func radioSnapshot() (stations []RadioStation, activeID string, playing bool, status string) {
	radioState.RLock()
	status = radioState.status
	radioState.RUnlock()

	confMu.Lock()
	stations = append([]RadioStation(nil), conf.System.RadioStations...)
	activeID = conf.System.RadioActiveID
	playing = conf.System.RadioPlaying
	confMu.Unlock()
	return
}

func isRadioPlaying() bool {
	confMu.Lock()
	playing := conf.System.RadioPlaying
	confMu.Unlock()
	return playing
}

func setRadioStatus(status string) {
	radioState.Lock()
	radioState.status = status
	radioState.Unlock()
}

func startRadioFromConfig() {
	confMu.Lock()
	activeID := conf.System.RadioActiveID
	playing := conf.System.RadioPlaying
	confMu.Unlock()
	if playing && activeID != "" {
		if err := startRadio(activeID); err != nil {
			log.Printf("network radio startup failed: %v", err)
			confMu.Lock()
			conf.System.RadioPlaying = false
			confMu.Unlock()
			saveConfig()
		}
	}
}

func startRadio(id string) error {
	confMu.Lock()
	var station RadioStation
	for _, item := range conf.System.RadioStations {
		if item.ID == id {
			station = item
			break
		}
	}
	confMu.Unlock()
	if station.ID == "" {
		return fmt.Errorf("radio station not found")
	}

	if _, err := validateRadioURL(station.URL); err != nil {
		return err
	}
	drainRadioPCM()

	radioState.Lock()
	if radioState.cancel != nil {
		radioState.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	radioState.cancel = cancel
	radioState.status = "connecting"
	radioState.Unlock()

	confMu.Lock()
	conf.System.RadioActiveID = station.ID
	conf.System.RadioPlaying = true
	// A radio station is an alternative music source, not an additional mix.
	conf.System.MusicPlaying = false
	confMu.Unlock()
	select {
	case stopmusic <- true:
	default:
	}
	saveConfig()

	go runRadio(ctx, station)
	return nil
}

func stopRadio() {
	stopRadioProcess()

	confMu.Lock()
	conf.System.RadioPlaying = false
	confMu.Unlock()
	saveConfig()
}

func stopRadioProcess() {
	radioState.Lock()
	if radioState.cancel != nil {
		radioState.cancel()
		radioState.cancel = nil
	}
	radioState.status = "stopped"
	radioState.Unlock()
	drainRadioPCM()
}

func drainRadioPCM() {
	for {
		select {
		case <-radioPCM:
		default:
			return
		}
	}
}

func switchToLocalMusic() {
	if isRadioPlaying() {
		stopRadio()
	}
	confMu.Lock()
	wasPlaying := conf.System.MusicPlaying
	if !wasPlaying {
		conf.System.MusicPlaying = true
	}
	confMu.Unlock()
	if !wasPlaying {
		select {
		case startmusic <- true:
		default:
		}
		saveConfig()
	}
}

func runRadio(ctx context.Context, station RadioStation) {
	for {
		if ctx.Err() != nil {
			return
		}
		setRadioStatus("connecting")
		err := streamRadio(ctx, station)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			setRadioStatus("reconnecting")
			log.Printf("network radio %q interrupted: %v; retrying", station.Name, err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func streamRadio(ctx context.Context, station RadioStation) error {
	if isHLSURL(station.URL) {
		return streamHLSRadio(ctx, station.URL)
	}
	return streamMP3Radio(ctx, station.URL)
}

var radioHTTPClient = &http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ResponseHeaderTimeout: 15 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	},
}

func isHLSURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(path.Ext(u.Path), ".m3u8") ||
		strings.Contains(strings.ToLower(u.Query().Get("format")), "m3u8")
}

func streamMP3Radio(ctx context.Context, rawURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "nrlnanny/embedded-radio")
	resp, err := radioHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("open MP3 stream: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("open MP3 stream: HTTP %s", resp.Status)
	}

	decoder, err := mp3.NewDecoder(resp.Body)
	if err != nil {
		return fmt.Errorf("decode MP3 stream: %w", err)
	}
	writer := newRadioPCMWriter(ctx, decoder.SampleRate())
	var playbackOnce sync.Once

	// go-mp3 emits signed 16-bit little-endian stereo PCM. Read whole sample
	// groups so an HTTP read boundary can never split a stereo frame.
	buf := make([]byte, 4096)
	for {
		n, readErr := io.ReadFull(decoder, buf)
		if n >= 4 {
			mono := make([]int, n/4)
			for i := range mono {
				left := int(int16(binary.LittleEndian.Uint16(buf[i*4:])))
				right := int(int16(binary.LittleEndian.Uint16(buf[i*4+2:])))
				mono[i] = (left + right) / 2
			}
			if err := writer.Write(mono); err != nil {
				return err
			}
			playbackOnce.Do(func() {
				setRadioStatus("playing")
				log.Printf("network radio MP3 playback started: %d Hz stereo -> 16 kHz mono", decoder.SampleRate())
			})
		}
		if readErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("read MP3 stream: %w", readErr)
		}
	}
}

func streamHLSRadio(ctx context.Context, rawURL string) error {
	// gohlslib's default download callbacks log every segment and playlist
	// refresh. Those are normal HLS activity, not retries, so keep routine
	// downloads quiet and reserve logs for playback state and real errors.
	c := &gohlslib.Client{
		URI:                       rawURL,
		HTTPClient:                radioHTTPClient,
		OnDownloadPrimaryPlaylist: func(string) {},
		OnDownloadStreamPlaylist:  func(string) {},
		OnDownloadSegment:         func(string) {},
		OnDownloadPart:            func(string) {},
	}
	decodeErrors := make(chan error, 1)
	clientDone := make(chan error, 1)
	var decoder *waxaac.Decoder
	var playbackOnce sync.Once

	reportDecodeError := func(err error) {
		select {
		case decodeErrors <- err:
		default:
		}
	}

	c.OnTracks = func(tracks []*gohlslib.Track) error {
		for _, track := range tracks {
			codec, ok := track.Codec.(*hlscodecs.MPEG4Audio)
			if !ok {
				continue
			}
			asc, err := codec.Config.Marshal()
			if err != nil {
				return fmt.Errorf("read HLS AAC configuration: %w", err)
			}
			cfg, err := waxaac.ParseASC(asc)
			if err != nil {
				return fmt.Errorf("unsupported HLS AAC configuration: %w", err)
			}
			format, err := cfg.Format()
			if err != nil {
				return err
			}
			decoder, err = waxaac.NewDecoder(cfg, format)
			if err != nil {
				return fmt.Errorf("initialize embedded AAC decoder: %w", err)
			}
			log.Printf("network radio HLS track detected: AAC-LC, %d Hz, %d channel(s)", format.Rate, format.Channels)
			writer := newRadioPCMWriter(ctx, format.Rate)
			c.OnDataMPEG4Audio(track, func(_ int64, accessUnits [][]byte) {
				for _, accessUnit := range accessUnits {
					err := decoder.Decode(accessUnit, func(pcm *audio.Buffer) error {
						mono := make([]int, pcm.N)
						for i := range mono {
							var sum float64
							for channel := 0; channel < pcm.Fmt.Channels; channel++ {
								sum += float64(pcm.ChanF(channel)[i])
							}
							mono[i] = clampPCM(int(math.Round(sum * 32767 / float64(pcm.Fmt.Channels))))
						}
						if err := writer.Write(mono); err != nil {
							return err
						}
						playbackOnce.Do(func() {
							setRadioStatus("playing")
							log.Printf("network radio HLS playback started: embedded AAC decoder -> 16 kHz mono")
						})
						return nil
					})
					if err != nil {
						reportDecodeError(fmt.Errorf("decode HLS AAC: %w", err))
						return
					}
				}
			})
			return nil
		}
		return fmt.Errorf("HLS stream does not contain a supported AAC-LC audio track")
	}
	c.OnDecodeError = func(err error) {
		log.Printf("HLS container decode warning: %v", err)
	}
	if err := c.Start(); err != nil {
		return fmt.Errorf("start HLS client: %w", err)
	}
	defer func() {
		c.Close()
		if decoder != nil {
			decoder.Release()
		}
	}()
	go func() { clientDone <- c.Wait2() }()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-decodeErrors:
		return err
	case err := <-clientDone:
		if err == nil {
			return io.EOF
		}
		return fmt.Errorf("read HLS stream: %w", err)
	}
}

// radioPCMWriter is a stateful windowed-sinc resampler. Keeping an absolute
// phase and overlap between decoder blocks avoids clicks and timing drift at
// MP3/AAC frame boundaries (especially 44.1 kHz -> 16 kHz).
type radioPCMWriter struct {
	ctx        context.Context
	sourceRate int
	buffer     []int
	base       int64
	nextNum    int64
	pending    []int
}

func newRadioPCMWriter(ctx context.Context, sourceRate int) *radioPCMWriter {
	return &radioPCMWriter{
		ctx:        ctx,
		sourceRate: sourceRate,
		nextNum:    16 * 16000,
	}
}

func (w *radioPCMWriter) Write(samples []int) error {
	if w.sourceRate <= 0 {
		return fmt.Errorf("invalid radio sample rate: %d", w.sourceRate)
	}
	w.buffer = append(w.buffer, samples...)
	const radius = 16
	cutoff := 0.47
	if w.sourceRate > 16000 {
		cutoff *= 16000.0 / float64(w.sourceRate)
	}

	for {
		center := w.nextNum / 16000
		if center+radius >= w.base+int64(len(w.buffer)) {
			break
		}
		fraction := float64(w.nextNum%16000) / 16000
		var weighted, weightSum float64
		for tap := -radius + 1; tap <= radius; tap++ {
			index := center + int64(tap)
			if index < w.base {
				continue
			}
			distance := float64(tap) - fraction
			x := 2 * cutoff * distance
			sinc := 1.0
			if x != 0 {
				sinc = math.Sin(math.Pi*x) / (math.Pi * x)
			}
			window := 0.5 * (1 + math.Cos(math.Pi*distance/radius))
			weight := 2 * cutoff * sinc * window
			weighted += float64(w.buffer[index-w.base]) * weight
			weightSum += weight
		}
		if weightSum != 0 {
			w.pending = append(w.pending, clampPCM(int(math.Round(weighted/weightSum))))
		}
		w.nextNum += int64(w.sourceRate)

		for len(w.pending) >= opusFrameSamples {
			frame := append([]int(nil), w.pending[:opusFrameSamples]...)
			w.pending = w.pending[opusFrameSamples:]
			select {
			case radioPCM <- [][]int{frame}:
			case <-w.ctx.Done():
				return w.ctx.Err()
			}
		}
	}

	keepFrom := w.nextNum/16000 - radius
	if drop := keepFrom - w.base; drop > 0 {
		w.buffer = w.buffer[drop:]
		w.base = keepFrom
	}
	return nil
}
