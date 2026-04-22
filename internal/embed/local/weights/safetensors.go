// Package weights reads safetensors files.
//
// The format: 8-byte little-endian u64 header length N, followed by N bytes
// of UTF-8 JSON metadata, followed by the raw tensor blob. Each JSON entry
// maps a tensor name to {dtype, shape, data_offsets:[start,end]} where
// offsets are relative to the start of the blob.
//
// See https://huggingface.co/docs/safetensors for the spec.
package weights

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

type header struct {
	Metadata map[string]string           `json:"__metadata__,omitempty"`
	Tensors  map[string]tensorDescriptor `json:"-"`
}

type tensorDescriptor struct {
	DType       string `json:"dtype"`
	Shape       []int  `json:"shape"`
	DataOffsets [2]int `json:"data_offsets"`
}

// File is a loaded safetensors file. The underlying blob is kept in memory
// as a single byte slice. For the ~90 MB MiniLM checkpoint this is fine;
// for much larger models we could switch to mmap.
type File struct {
	tensors map[string]tensorDescriptor
	blob    []byte
}

// Open loads a safetensors file fully into memory and parses its header.
func Open(path string) (*File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read safetensors: %w", err)
	}
	if len(raw) < 8 {
		return nil, fmt.Errorf("safetensors too short: %d bytes", len(raw))
	}
	hdrLen := binary.LittleEndian.Uint64(raw[:8])
	if 8+hdrLen > uint64(len(raw)) {
		return nil, fmt.Errorf("safetensors header length %d exceeds file size", hdrLen)
	}
	hdrBytes := raw[8 : 8+hdrLen]
	blob := raw[8+hdrLen:]

	// The header is a flat JSON object; we unmarshal into a map then extract
	// the reserved "__metadata__" key ourselves.
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(hdrBytes, &generic); err != nil {
		return nil, fmt.Errorf("parse safetensors header: %w", err)
	}
	tensors := make(map[string]tensorDescriptor, len(generic))
	for k, v := range generic {
		if k == "__metadata__" {
			continue
		}
		var d tensorDescriptor
		if err := json.Unmarshal(v, &d); err != nil {
			return nil, fmt.Errorf("parse tensor %q: %w", k, err)
		}
		tensors[k] = d
	}
	return &File{tensors: tensors, blob: blob}, nil
}

// Names returns all tensor names. Useful for debugging.
func (f *File) Names() []string {
	out := make([]string, 0, len(f.tensors))
	for k := range f.tensors {
		out = append(out, k)
	}
	return out
}

// Shape returns the shape of a named tensor.
func (f *File) Shape(name string) ([]int, bool) {
	d, ok := f.tensors[name]
	if !ok {
		return nil, false
	}
	return d.Shape, true
}

// F32 returns a tensor as a freshly-allocated []float32 slice, converting
// from the stored dtype. Supported dtypes: F32, F16, BF16.
func (f *File) F32(name string) ([]float32, []int, error) {
	d, ok := f.tensors[name]
	if !ok {
		return nil, nil, fmt.Errorf("tensor not found: %s", name)
	}
	start, end := d.DataOffsets[0], d.DataOffsets[1]
	if start < 0 || end > len(f.blob) || start > end {
		return nil, nil, fmt.Errorf("tensor %s: invalid offsets [%d,%d] in blob of %d",
			name, start, end, len(f.blob))
	}
	raw := f.blob[start:end]

	n := numel(d.Shape)
	out := make([]float32, n)

	switch d.DType {
	case "F32":
		if len(raw) != n*4 {
			return nil, nil, fmt.Errorf("tensor %s: F32 size mismatch: %d bytes for %d elements", name, len(raw), n)
		}
		for i := 0; i < n; i++ {
			bits := binary.LittleEndian.Uint32(raw[i*4:])
			out[i] = math.Float32frombits(bits)
		}
	case "F16":
		if len(raw) != n*2 {
			return nil, nil, fmt.Errorf("tensor %s: F16 size mismatch", name)
		}
		for i := 0; i < n; i++ {
			bits := binary.LittleEndian.Uint16(raw[i*2:])
			out[i] = f16ToF32(bits)
		}
	case "BF16":
		if len(raw) != n*2 {
			return nil, nil, fmt.Errorf("tensor %s: BF16 size mismatch", name)
		}
		for i := 0; i < n; i++ {
			bits := uint32(binary.LittleEndian.Uint16(raw[i*2:])) << 16
			out[i] = math.Float32frombits(bits)
		}
	default:
		return nil, nil, fmt.Errorf("tensor %s: unsupported dtype %s", name, d.DType)
	}
	return out, d.Shape, nil
}

func numel(shape []int) int {
	n := 1
	for _, s := range shape {
		n *= s
	}
	return n
}

// f16ToF32 converts an IEEE-754 binary16 value to float32.
func f16ToF32(h uint16) float32 {
	sign := uint32(h>>15) & 0x1
	exp := uint32(h>>10) & 0x1F
	mant := uint32(h) & 0x3FF

	var f32 uint32
	switch {
	case exp == 0 && mant == 0:
		f32 = sign << 31
	case exp == 0:
		// Subnormal: renormalize.
		e := int32(-1)
		m := mant
		for m&0x400 == 0 {
			m <<= 1
			e--
		}
		m &= 0x3FF
		exp32 := uint32(127 + e + 1)
		f32 = (sign << 31) | (exp32 << 23) | (m << 13)
	case exp == 0x1F:
		// Inf / NaN.
		f32 = (sign << 31) | (0xFF << 23) | (mant << 13)
	default:
		exp32 := exp - 15 + 127
		f32 = (sign << 31) | (exp32 << 23) | (mant << 13)
	}
	return math.Float32frombits(f32)
}
