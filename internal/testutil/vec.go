package testutil

import (
	"encoding/binary"
	"fmt"
	"math"
)

// DeserializeF32 parses a sqlite-vec blob back into a float32 slice.
func DeserializeF32(buf []byte) ([]float32, error) {
	if len(buf)%4 != 0 {
		return nil, fmt.Errorf("invalid blob length %d", len(buf))
	}
	vec := make([]float32, len(buf)/4)
	for i := range vec {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return vec, nil
}
