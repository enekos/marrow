package related

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
)

// rowsIter wraps *sql.Rows so we can call Scan/Next/Err/Close generically.
// This is only here to keep the load functions readable; no real behaviour.
type rowsIter struct {
	rows *sql.Rows
}

func (r *rowsIter) Next() bool                      { return r.rows.Next() }
func (r *rowsIter) Scan(dest ...any) error          { return r.rows.Scan(dest...) }
func (r *rowsIter) Err() error                      { return r.rows.Err() }
func (r *rowsIter) Close() error                    { return r.rows.Close() }

// deserializeVec decodes a sqlite-vec blob (raw little-endian float32) into
// a slice of float32. The blob length must be a multiple of 4.
func deserializeVec(blob []byte) ([]float32, error) {
	if len(blob)%4 != 0 {
		return nil, fmt.Errorf("vec blob length %d not a multiple of 4", len(blob))
	}
	n := len(blob) / 4
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		bits := binary.LittleEndian.Uint32(blob[i*4:])
		out[i] = math.Float32frombits(bits)
	}
	return out, nil
}
