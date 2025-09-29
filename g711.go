package main

const (
	SEG_MASK   = 0x70 // 0b01110000
	QUANT_MASK = 0x0F // 0b00001111
	SEG_SHIFT  = 4
	BIAS       = 0x84 //

)

func alaw2linear(code byte) int16 {
	// XOR with 0x55
	code ^= 0x55

	// Extract segment and quantization index
	seg := (code & SEG_MASK) >> 4
	quant := code & QUANT_MASK

	// Compute base sample value
	sample := int16((quant << 4) | 0x08)

	// Apply segment scaling
	if seg > 0 {
		sample = (sample + 0x100) << (seg - 1)
	}

	// Apply sign: if original code had sign bit (0x80), it's positive; else negative
	if code&0x80 != 0 {
		return sample
	}
	return -sample
}

// Linear2Alaw converts a 16-bit linear PCM sample to an 8-bit A-law sample.
func Linear2Alaw(sample int16) byte {
	// 1. Extract the sign bit
	sign := (sample >> 8) & 0x80

	// 2. Handle negative numbers and avoid overflow
	if sign != 0 {
		if sample == -32768 {
			sample = 32767 // Handle smallest negative number (to avoid overflow)
		} else {
			sample = -sample // Take absolute value for encoding
		}
	}

	// 4. Add bias (A-law encoding bias is 132)
	sample += 132
	if sample < 0 {
		sample = 0 // Ensure sample is not negative after adding bias
	}

	// 5. Calculate segment number (seg)
	seg := 7
	for i := 0x4000; i >= 0x40 && (int(sample)&i) == 0; i >>= 1 {
		seg-- // Determine segment based on higher bits of the sample
	}

	// 6. Calculate mantissa (mant)
	mant := (int(sample) >> (seg + 3)) & 0x0f // Extract mantissa (low 4 bits)

	// 7. Combine segment number and mantissa
	alaw := byte((seg << 4) | mant) // Combine segment and mantissa into an 8-bit value

	// 8. Perform XOR operation based on the sign bit and return the result
	if sign != 0 {
		return (alaw ^ 0xD5) & 0xff // XOR with 0xD5 for negative samples
	}
	return (alaw ^ 0x55) & 0xff // XOR with 0x55 for positive samples
}

// Encode converts a slice of 16-bit linear PCM samples to a slice of 8-bit A-law samples.
func G711Encode(pcmData []int) []byte {
	encoded := make([]byte, len(pcmData))
	for i := range pcmData {
		encoded[i] = Linear2Alaw(int16(pcmData[i]))
	}
	return encoded
}
