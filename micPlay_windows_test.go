//go:build windows

package main

import (
	"math"
	"testing"
)

// 回归测试：缓冲区长度不能被重采样比率整除时相位会变负，
// 旧实现会以 idx=-1 越界 panic（cubicResample idx out of range [-1]）。
func TestCubicResampleNegativePhase(t *testing.T) {
	const (
		srcRate = 48000
		dstRate = 16000
		chunks  = 50
		chunkSz = 4801 // 不能被 3 整除，触发负相位
	)

	var phase float64
	var tail []int16
	totalOut := 0
	prev := int16(0)
	for c := 0; c < chunks; c++ {
		src := make([]int16, chunkSz)
		for i := range src {
			n := c*chunkSz + i
			src[i] = int16(12000 * math.Sin(2*math.Pi*440*float64(n)/srcRate))
		}
		out := cubicResample(src, srcRate, dstRate, &phase, &tail)
		for _, v := range out {
			// 正弦波连续性检查：相邻输出采样差值不能出现跳变
			if d := int(v) - int(prev); d > 3000 || d < -3000 {
				t.Fatalf("chunk %d: output discontinuity: %d -> %d", c, prev, v)
			}
			prev = v
		}
		totalOut += len(out)
	}

	expected := chunks * chunkSz * dstRate / srcRate
	if diff := totalOut - expected; diff < -chunks || diff > chunks {
		t.Fatalf("total output %d, expected ~%d", totalOut, expected)
	}
}

func TestCubicResampleBoundaries(t *testing.T) {
	var phase float64
	var tail []int16
	// 空输入、小输入不应 panic
	if out := cubicResample(nil, 48000, 16000, &phase, &tail); len(out) != 0 {
		t.Fatalf("empty input produced %d samples", len(out))
	}
	if out := cubicResample([]int16{1, 2}, 48000, 16000, &phase, &tail); len(out) != 0 {
		t.Fatalf("tiny input produced %d samples", len(out))
	}
	if out := cubicResample([]int16{1, 2, 3, 4, 5, 6}, 0, 16000, &phase, &tail); out != nil {
		t.Fatal("zero srcRate should return nil")
	}
}
