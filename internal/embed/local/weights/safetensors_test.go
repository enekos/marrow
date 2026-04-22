package weights

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// writeSafetensors builds a minimal safetensors file with the given fp32
// tensors for testing.
func writeSafetensors(t *testing.T, tensors map[string][]float32, shapes map[string][]int) string {
	t.Helper()
	type desc struct {
		DType       string `json:"dtype"`
		Shape       []int  `json:"shape"`
		DataOffsets [2]int `json:"data_offsets"`
	}
	hdr := map[string]desc{}
	offset := 0
	var names []string
	for name := range tensors {
		names = append(names, name)
	}
	// Deterministic order.
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	for _, name := range names {
		n := len(tensors[name])
		hdr[name] = desc{
			DType:       "F32",
			Shape:       shapes[name],
			DataOffsets: [2]int{offset, offset + n*4},
		}
		offset += n * 4
	}
	hdrJSON, err := json.Marshal(hdr)
	if err != nil {
		t.Fatal(err)
	}
	// Assemble the file.
	dir := t.TempDir()
	path := filepath.Join(dir, "m.safetensors")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var lenBuf [8]byte
	binary.LittleEndian.PutUint64(lenBuf[:], uint64(len(hdrJSON)))
	if _, err := f.Write(lenBuf[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(hdrJSON); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	for _, name := range names {
		for _, v := range tensors[name] {
			binary.LittleEndian.PutUint32(buf, math.Float32bits(v))
			if _, err := f.Write(buf); err != nil {
				t.Fatal(err)
			}
		}
	}
	return path
}

func TestOpen_ReadsF32(t *testing.T) {
	tensors := map[string][]float32{
		"w": {1.5, -2.25, 3.125, 0, 0.5, -0.5},
	}
	shapes := map[string][]int{"w": {2, 3}}
	p := writeSafetensors(t, tensors, shapes)
	f, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	got, shape, err := f.F32("w")
	if err != nil {
		t.Fatal(err)
	}
	if len(shape) != 2 || shape[0] != 2 || shape[1] != 3 {
		t.Fatalf("shape = %v", shape)
	}
	want := tensors["w"]
	if len(got) != len(want) {
		t.Fatalf("len=%d want=%d", len(got), len(want))
	}
	for i, v := range want {
		if got[i] != v {
			t.Fatalf("got[%d]=%f want %f", i, got[i], v)
		}
	}
}

func TestOpen_UnknownTensor(t *testing.T) {
	p := writeSafetensors(t,
		map[string][]float32{"a": {1}},
		map[string][]int{"a": {1}},
	)
	f, _ := Open(p)
	if _, _, err := f.F32("missing"); err == nil {
		t.Fatal("expected error")
	}
}

func TestF16Conversion(t *testing.T) {
	cases := []struct {
		bits uint16
		want float32
	}{
		{0x0000, 0},
		{0x3C00, 1},              // 1.0
		{0x4000, 2},              // 2.0
		{0xBC00, -1},             // -1.0
		{0x3555, 0.333251953125}, // approx 1/3
	}
	for _, c := range cases {
		got := f16ToF32(c.bits)
		if math.Abs(float64(got-c.want)) > 1e-6 {
			t.Errorf("f16ToF32(%#x) = %v want %v", c.bits, got, c.want)
		}
	}
}
