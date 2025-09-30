package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

var streamReader *PCMStreamReader

func newplay() {
	op := &oto.NewContextOptions{}
	op.SampleRate = 8000
	op.ChannelCount = 1
	op.Format = oto.FormatSignedInt16LE

	otoCtx, ready, err := oto.NewContext(op)
	if err != nil {
		log.Fatal("创建录音失败:", err)
	}
	<-ready

	streamReader = NewPCMStreamReader()

	player := otoCtx.NewPlayer(streamReader)
	defer player.Close()

	log.Println("开始录音和监听...")
	player.Play()

	for player.IsPlaying() {
		time.Sleep(time.Millisecond * 100)
	}
	log.Println("监听完毕...")
}

// PCMStreamReader 实现 io.Reader，用于实时播放 []int16 流
type PCMStreamReader struct {
	mu            sync.Mutex
	cond          *sync.Cond    // 用于等待新数据
	buffer        *bytes.Buffer // 存储所有待播放的字节
	closed        bool          // 标记流是否已关闭
	maxBufferSize int           // 限制缓冲区的最大大小
}

// NewPCMStreamReader 创建一个新的流式 Reader
func NewPCMStreamReader() *PCMStreamReader {
	reader := &PCMStreamReader{
		buffer:        bytes.NewBuffer(nil),
		maxBufferSize: 8000 * 2 * 300, // 假设 8000 采样率，2 字节/采样，缓冲 300 秒的数据
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

	// 检查缓冲区是否已达到最大容量
	if r.buffer.Len()+len(chunk) > r.maxBufferSize {
		// 缓冲区满，这里可以选择：
		// 1. 丢弃当前 chunk (如果允许丢帧，且希望保持低延迟)
		fmt.Println()
		log.Printf("缓冲区满 (%.2fMB / %.2fMB)，清空缓存，缓冲区大小: %d\n",
			float64(r.buffer.Len())/(1024*1024), float64(r.maxBufferSize)/(1024*1024), r.buffer.Len())
		// return nil // 直接返回，丢弃当前帧

		r.buffer.Reset()

		// 2. 阻塞直到有空间 (如果不能丢帧，且发送方可以阻塞)
		// 等待 Read 消耗一些数据
		// for r.buffer.Len()+len(chunk) > r.maxBufferSize && !r.closed {
		// 	log.Println("缓冲区满，WriteChunk 阻塞等待 Read 消费...")
		// 	r.cond.Wait()
		// }
		// if r.closed { // 再次检查是否在等待期间关闭
		// 	return io.ErrClosedPipe
		// }
	}

	r.buffer.Write(chunk)
	r.cond.Broadcast() // 唤醒所有等待的 Read

	return nil
}

// Close 关闭流，通知播放结束
func (r *PCMStreamReader) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed {
		r.closed = true
		r.cond.Broadcast() // 唤醒所有等待的 Read
	}
}

// Read 实现 io.Reader，被 oto 调用
func (r *PCMStreamReader) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for {
		// 如果 buffer 中有数据，先返回
		if r.buffer.Len() > 0 {
			n, _ := r.buffer.Read(p)
			if n > 0 {
				r.cond.Broadcast() // 读出数据后，通知 WriteChunk 可能有空间了
			}
			return n, nil
		}

		// 如果流已关闭，且无数据，返回 EOF
		if r.closed {
			return 0, io.EOF
		}

		// 没有数据，阻塞等待新数据
		r.cond.Wait()
	}
}
