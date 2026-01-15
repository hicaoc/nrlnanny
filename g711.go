package main

const (
	SEG_MASK   = 0x70 // 0b01110000
	QUANT_MASK = 0x0F // 0b00001111
	SEG_SHIFT  = 4
	BIAS       = 0x84 //

)

var (
	alaw2linearTable [256]int16
	linear2alawTable [65536]byte
)

func init() {
	// Initialize Alaw to Linear table
	for i := range 256 {
		alaw2linearTable[i] = rawAlaw2linear(byte(i))
	}

	// Initialize Linear to Alaw table
	for i := range 65536 {
		linear2alawTable[i] = rawLinear2Alaw(int16(i))
	}
}

func alaw2linear(code byte) int16 {
	return alaw2linearTable[code]
}

func Linear2Alaw(sample int16) byte {

	return linear2alawTable[uint16(sample)]
}

// Internal version of alaw2linear for table generation
func rawAlaw2linear(code byte) int16 {
	code ^= 0x55

	sign := int16(code & 0x80)
	seg := (code & 0x70) >> 4
	quant := code & 0x0F

	var sample int16
	if seg == 0 {
		sample = int16(quant<<1) | 0x01
	} else {
		sample = (int16(quant<<1) | 0x21) << (seg - 1)
	}

	if sign != 0 {
		return sample << 3
	}
	return -(sample << 3)
}

// Internal version of Linear2Alaw for table generation
func rawLinear2Alaw(sample int16) byte {
	var sign byte
	if sample < 0 {
		if sample == -32768 {
			sample = -32767
		}
		sample = -sample
		sign = 0x00
	} else {
		sign = 0x80
	}

	// 13-bit absolute value for A-law
	pcm := sample >> 3

	seg := byte(0)
	if pcm >= 32 {
		seg = 1
		t := int16(64)
		for seg < 7 && pcm >= t {
			t <<= 1
			seg++
		}
	}

	var mant byte
	if seg == 0 {
		mant = byte(pcm>>1) & 0x0F
	} else {
		mant = byte(pcm>>seg) & 0x0F
	}

	return (sign | (seg << 4) | mant) ^ 0x55
}

// G711Encode converts a slice of 16-bit linear PCM samples to a slice of 8-bit A-law samples.
func G711Encode(pcmData []int) []byte {
	encoded := make([]byte, len(pcmData))
	for i := range pcmData {
		pcmData[i] = AdjustVolumeInt(pcmData[i], conf.System.Volume)
		encoded[i] = Linear2Alaw(int16(pcmData[i]))
	}
	return encoded
}

func G711Decode(encodedData []byte) []int {
	decoded := make([]int, len(encodedData))
	for i := range encodedData {
		decoded[i] = int(alaw2linear(encodedData[i]))
	}
	return decoded
}

// AdjustVolume 调整 16-bit PCM 音频的音量
// samples: 原始 PCM 采样点（int16）
// volume: 音量缩放因子（0.0 ～ 1.0 为降音量，>1.0 为增益，可能削波）
// 返回调整后的采样点
func AdjustVolume(samples []int16, volume float64) []int16 {
	result := make([]int16, len(samples))
	for i, sample := range samples {
		// 转为 float64 进行缩放
		scaled := float64(sample) * volume

		// 限幅（clipping）到 int16 范围 [-32768, 32767]
		if scaled > 32767 {
			scaled = 32767
		} else if scaled < -32768 {
			scaled = -32768
		}

		result[i] = int16(scaled)
	}
	return result
}

func AdjustVolumeInt(sample int, volume float64) int {
	scaled := float64(sample) * volume

	// 限幅（clipping）到 int16 范围 [-32768, 32767]
	if scaled > 32767 {
		scaled = 32767
	} else if scaled < -32768 {
		scaled = -32768
	}

	return int(scaled)
}

func AdjustVolumeInt16(sample int, volume float64) int {
	scaled := float64(sample) * volume

	// 限幅（clipping）到 int16 范围 [-32768, 32767]
	if scaled > 32767 {
		scaled = 32767
	} else if scaled < -32768 {
		scaled = -32768
	}

	return int(scaled)
}
