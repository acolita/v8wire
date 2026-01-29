// Package wire implements low-level binary primitives for V8 serialization format.
//
// This package handles the mechanical byte manipulation required to read
// V8's wire format: varints, doubles, strings, and alignment.
package wire

import (
	"encoding/binary"
	"errors"
	"math"
	"unicode/utf16"
)

// Common errors returned by Reader methods.
var (
	ErrUnexpectedEOF  = errors.New("wire: unexpected end of input")
	ErrVarintOverflow = errors.New("wire: varint overflow")
	ErrInvalidUTF16   = errors.New("wire: invalid UTF-16 sequence")
)

// Reader reads V8 serialized data from a byte buffer.
// It tracks position for sequential reads and supports alignment.
type Reader struct {
	data []byte
	pos  int
}

// NewReader creates a Reader from the given byte slice.
// The Reader does not copy the data; it reads directly from the slice.
func NewReader(data []byte) *Reader {
	return &Reader{data: data, pos: 0}
}

// Pos returns the current read position.
func (r *Reader) Pos() int {
	return r.pos
}

// Len returns the total length of the underlying data.
func (r *Reader) Len() int {
	return len(r.data)
}

// Remaining returns the number of bytes left to read.
func (r *Reader) Remaining() int {
	return len(r.data) - r.pos
}

// EOF returns true if all bytes have been consumed.
func (r *Reader) EOF() bool {
	return r.pos >= len(r.data)
}

// Peek returns the next byte without advancing the position.
// Returns 0 and ErrUnexpectedEOF if at end of input.
func (r *Reader) Peek() (byte, error) {
	if r.pos >= len(r.data) {
		return 0, ErrUnexpectedEOF
	}
	return r.data[r.pos], nil
}

// ReadByte reads a single byte and advances the position.
func (r *Reader) ReadByte() (byte, error) {
	if r.pos >= len(r.data) {
		return 0, ErrUnexpectedEOF
	}
	b := r.data[r.pos]
	r.pos++
	return b, nil
}

// ReadBytes reads exactly n bytes and advances the position.
// Returns ErrUnexpectedEOF if fewer than n bytes remain.
func (r *Reader) ReadBytes(n int) ([]byte, error) {
	if r.pos+n > len(r.data) {
		return nil, ErrUnexpectedEOF
	}
	result := r.data[r.pos : r.pos+n]
	r.pos += n
	return result, nil
}

// ReadVarint reads a base-128 unsigned varint.
// V8 uses standard protobuf-style varints: 7 bits per byte,
// high bit indicates continuation.
func (r *Reader) ReadVarint() (uint64, error) {
	var result uint64
	var shift uint

	for {
		if r.pos >= len(r.data) {
			return 0, ErrUnexpectedEOF
		}

		b := r.data[r.pos]
		r.pos++

		// Check for overflow before shifting
		if shift >= 64 || (shift == 63 && b > 1) {
			return 0, ErrVarintOverflow
		}

		result |= uint64(b&0x7F) << shift
		if b&0x80 == 0 {
			return result, nil
		}
		shift += 7
	}
}

// ReadVarint32 reads a varint and returns it as uint32.
// Returns error if the value exceeds uint32 range.
func (r *Reader) ReadVarint32() (uint32, error) {
	v, err := r.ReadVarint()
	if err != nil {
		return 0, err
	}
	if v > math.MaxUint32 {
		return 0, ErrVarintOverflow
	}
	return uint32(v), nil
}

// ZigZagDecode decodes a ZigZag-encoded unsigned integer to signed.
// ZigZag encoding maps signed integers to unsigned integers so that
// numbers with small absolute values have small varint encodings:
//
//	0 → 0, -1 → 1, 1 → 2, -2 → 3, 2 → 4, ...
//
// Decoding: (n >> 1) ^ -(n & 1)
func ZigZagDecode(n uint64) int64 {
	return int64(n>>1) ^ -int64(n&1)
}

// ZigZagDecode32 decodes a ZigZag-encoded uint32 to int32.
func ZigZagDecode32(n uint32) int32 {
	return int32(n>>1) ^ -int32(n&1)
}

// ReadZigZag reads a varint and ZigZag-decodes it to a signed int64.
func (r *Reader) ReadZigZag() (int64, error) {
	v, err := r.ReadVarint()
	if err != nil {
		return 0, err
	}
	return ZigZagDecode(v), nil
}

// ReadZigZag32 reads a varint and ZigZag-decodes it to a signed int32.
func (r *Reader) ReadZigZag32() (int32, error) {
	v, err := r.ReadVarint32()
	if err != nil {
		return 0, err
	}
	return ZigZagDecode32(v), nil
}

// ReadDouble reads an IEEE 754 double in little-endian byte order.
func (r *Reader) ReadDouble() (float64, error) {
	if r.pos+8 > len(r.data) {
		return 0, ErrUnexpectedEOF
	}
	bits := binary.LittleEndian.Uint64(r.data[r.pos:])
	r.pos += 8
	return math.Float64frombits(bits), nil
}

// AlignTo ensures the reader position is aligned to the given boundary.
// If not aligned, skips padding bytes until aligned.
// Boundary must be a power of 2.
func (r *Reader) AlignTo(boundary int) {
	if boundary <= 0 || (boundary&(boundary-1)) != 0 {
		return // invalid boundary, do nothing
	}
	remainder := r.pos % boundary
	if remainder != 0 {
		skip := boundary - remainder
		if r.pos+skip <= len(r.data) {
			r.pos += skip
		}
	}
}

// ReadOneByteString reads a Latin1 (one-byte) encoded string.
// The length is provided as the number of bytes/characters.
// Latin-1 bytes are converted to their Unicode equivalents (U+0000-U+00FF),
// and the result is returned as a valid UTF-8 Go string.
func (r *Reader) ReadOneByteString(length int) (string, error) {
	if length < 0 {
		return "", errors.New("wire: negative string length")
	}
	if length == 0 {
		return "", nil
	}
	bytes, err := r.ReadBytes(length)
	if err != nil {
		return "", err
	}
	// Latin-1 maps directly to Unicode code points 0-255.
	// Convert to proper UTF-8 encoding.
	runes := make([]rune, length)
	for i, b := range bytes {
		runes[i] = rune(b)
	}
	return string(runes), nil
}

// ReadTwoByteString reads a UTF-16LE encoded string.
// The length is provided as the number of UTF-16 code units (2 bytes each).
// Automatically handles alignment to 2-byte boundary before reading.
func (r *Reader) ReadTwoByteString(length int) (string, error) {
	if length < 0 {
		return "", errors.New("wire: negative string length")
	}
	if length == 0 {
		return "", nil
	}

	// Align to 2-byte boundary for UTF-16
	r.AlignTo(2)

	byteLen := length * 2
	if r.pos+byteLen > len(r.data) {
		return "", ErrUnexpectedEOF
	}

	// Read UTF-16LE code units
	u16 := make([]uint16, length)
	for i := 0; i < length; i++ {
		u16[i] = binary.LittleEndian.Uint16(r.data[r.pos:])
		r.pos += 2
	}

	// Decode UTF-16 to Go string (UTF-8)
	runes := utf16.Decode(u16)
	return string(runes), nil
}

// Skip advances the position by n bytes without reading.
func (r *Reader) Skip(n int) error {
	if r.pos+n > len(r.data) {
		return ErrUnexpectedEOF
	}
	r.pos += n
	return nil
}

// Reset resets the reader to the beginning of the data.
func (r *Reader) Reset() {
	r.pos = 0
}

// Data returns the underlying byte slice.
func (r *Reader) Data() []byte {
	return r.data
}
