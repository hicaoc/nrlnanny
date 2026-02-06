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

func rawAlaw2linear(code byte) int16 {
	code ^= 0x55

	iexp := int16((code & 0x70) >> 4)
	mant := int16(code & 0x0F)

	if iexp > 0 {
		mant += 16
	}

	mant = (mant << 4) + 0x08

	if iexp > 1 {
		mant <<= (iexp - 1)
	}

	if (code & 0x80) != 0 {
		return mant
	}
	return -mant
}

func rawLinear2Alaw(sample int16) byte {
	var sign byte
	var ix int16

	if sample < 0 {
		sign = 0x80
		ix = (^sample) >> 4 // ✅ 按位取反
	} else {
		ix = sample >> 4
	}

	if ix > 15 {
		iexp := byte(1)
		for ix > 31 {
			ix >>= 1
			iexp++
		}
		ix -= 16
		ix += int16(iexp << 4)
	}

	if sign == 0 {
		ix |= 0x80
	}

	return byte(ix) ^ 0x55
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
