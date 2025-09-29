package main

import (
	"bytes"
	"io"
	"log"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
	//"github.com/hajimehoshi/oto/v3"
)

var streamReader *PCMStreamReader

func newplay() {

	op := &oto.NewContextOptions{}

	// Usually 44100 or 48000. Other values might cause distortions in Oto
	op.SampleRate = 8000

	// Number of channels (aka locations) to play sounds from. Either 1 or 2.
	// 1 is mono sound, and 2 is stereo (most speakers are stereo).
	op.ChannelCount = 1

	// Format of the source. go-mp3's format is signed 16bit integers.
	op.Format = oto.FormatSignedInt16LE
	// 初始化 oto 音频上下文
	otoCtx, ready, err := oto.NewContext(op)
	if err != nil {
		log.Fatal("创建录音失败:", err)
	}
	<-ready // 等待上下文就绪

	// 创建流式 Reader
	streamReader = NewPCMStreamReader()

	// 创建播放器（传入 Reader）
	player := otoCtx.NewPlayer(streamReader)
	defer player.Close()

	// 启动播放
	log.Println("开始录音和监听...")
	player.Play()

	for player.IsPlaying() {
		//log.Println("等待数据...")
		time.Sleep(time.Microsecond * 100)
	}

}

// PCMStreamReader 实现 io.Reader，用于实时播放 []int16 流
type PCMStreamReader struct {
	mu     sync.Mutex
	cond   *sync.Cond    // 用于等待新数据
	chunks chan []byte   // 接收网络数据的 channel
	buffer *bytes.Buffer // 当前待读取的字节缓冲
	closed bool          // 标记流是否已关闭
}

// NewPCMStreamReader 创建一个新的流式 Reader
func NewPCMStreamReader() *PCMStreamReader {
	reader := &PCMStreamReader{
		chunks: make(chan []byte, 2000), // 缓冲 channel，防止发送阻塞
		buffer: bytes.NewBuffer(nil),
	}
	reader.cond = sync.NewCond(&reader.mu)
	return reader
}

// WriteChunk 从网络接收数据时调用（外部输入）
func (r *PCMStreamReader) WriteChunk(chunk []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return io.ErrClosedPipe
	}

	select {
	case r.chunks <- chunk:
		// 唤醒所有等待的 Read
		r.cond.Broadcast()
	default:
		// channel 满了，丢弃旧数据或阻塞（可优化）
		log.Println("缓冲区满，丢弃一帧")
	}
	return nil
}

// Close 关闭流，通知播放结束
func (r *PCMStreamReader) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed {
		r.closed = true
		close(r.chunks)
		r.cond.Broadcast()
	}
}

// Read 实现 io.Reader，被 oto 调用
func (r *PCMStreamReader) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for {
		// 1. 如果 buffer 中有数据，先返回
		if r.buffer.Len() > 0 {
			return r.buffer.Read(p)
		}

		// 2. 如果流已关闭，且无数据，返回 EOF
		if r.closed && len(r.chunks) == 0 {
			return 0, io.EOF
		}

		// 3. 尝试从 chunks 读取一个新块
		select {
		case chunk, ok := <-r.chunks:

			if !ok {
				if r.buffer.Len() == 0 {
					return 0, io.EOF
				}
				continue // 还有数据在 buffer，继续读
			}

			r.buffer.Write(chunk)
		default:
			// 没有数据，阻塞等待
			r.cond.Wait()
			continue
		}

		// 4. 转换后，从 buffer 读取到 p
		if r.buffer.Len() > 0 {
			return r.buffer.Read(p)
		}
	}
}
