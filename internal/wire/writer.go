package wire

import (
	"encoding/binary"
	"math"
)

// Writer writes V8 serialized data to a byte buffer.
type Writer struct {
	buf []byte
}

// NewWriter creates a new Writer with an initial capacity.
func NewWriter(capacity int) *Writer {
	return &Writer{buf: make([]byte, 0, capacity)}
}

// Bytes returns the written bytes.
func (w *Writer) Bytes() []byte {
	return w.buf
}

// Len returns the number of bytes written.
func (w *Writer) Len() int {
	return len(w.buf)
}

// Reset clears the buffer for reuse.
func (w *Writer) Reset() {
	w.buf = w.buf[:0]
}

// WriteByte writes a single byte. Implements io.ByteWriter.
// Always returns nil error for in-memory buffer.
func (w *Writer) WriteByte(b byte) error {
	w.buf = append(w.buf, b)
	return nil
}

// WriteBytes writes a slice of bytes.
func (w *Writer) WriteBytes(b []byte) {
	w.buf = append(w.buf, b...)
}

// WriteVarint writes an unsigned integer as a base-128 varint.
func (w *Writer) WriteVarint(n uint64) {
	for n >= 0x80 {
		w.buf = append(w.buf, byte(n)|0x80)
		n >>= 7
	}
	w.buf = append(w.buf, byte(n))
}

// WriteVarint32 writes a uint32 as a varint.
func (w *Writer) WriteVarint32(n uint32) {
	w.WriteVarint(uint64(n))
}

// ZigZagEncode encodes a signed int64 to unsigned using ZigZag encoding.
// Maps: 0 → 0, -1 → 1, 1 → 2, -2 → 3, 2 → 4, ...
func ZigZagEncode(n int64) uint64 {
	return uint64((n << 1) ^ (n >> 63))
}

// ZigZagEncode32 encodes a signed int32 to unsigned.
func ZigZagEncode32(n int32) uint32 {
	return uint32((n << 1) ^ (n >> 31))
}

// WriteZigZag writes a signed int64 as a ZigZag-encoded varint.
func (w *Writer) WriteZigZag(n int64) {
	w.WriteVarint(ZigZagEncode(n))
}

// WriteZigZag32 writes a signed int32 as a ZigZag-encoded varint.
func (w *Writer) WriteZigZag32(n int32) {
	w.WriteVarint32(ZigZagEncode32(n))
}

// WriteDouble writes an IEEE 754 double in little-endian byte order.
func (w *Writer) WriteDouble(f float64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], math.Float64bits(f))
	w.buf = append(w.buf, buf[:]...)
}

// WriteOneByteString writes a Latin1 string (each rune must be <= 255).
func (w *Writer) WriteOneByteString(s string) {
	for _, r := range s {
		w.buf = append(w.buf, byte(r))
	}
}

// WriteTwoByteString writes a UTF-16LE string.
// Handles alignment by padding if necessary.
func (w *Writer) WriteTwoByteString(s string) {
	// Align to 2-byte boundary
	if len(w.buf)%2 != 0 {
		w.buf = append(w.buf, 0x00)
	}

	// Convert to UTF-16
	for _, r := range s {
		if r <= 0xFFFF {
			// BMP character
			var buf [2]byte
			binary.LittleEndian.PutUint16(buf[:], uint16(r))
			w.buf = append(w.buf, buf[:]...)
		} else {
			// Surrogate pair for characters outside BMP
			r -= 0x10000
			high := uint16(0xD800 + (r >> 10))
			low := uint16(0xDC00 + (r & 0x3FF))
			var buf [4]byte
			binary.LittleEndian.PutUint16(buf[:2], high)
			binary.LittleEndian.PutUint16(buf[2:], low)
			w.buf = append(w.buf, buf[:]...)
		}
	}
}

// UTF16Length returns the number of UTF-16 code units needed for a string.
func UTF16Length(s string) int {
	count := 0
	for _, r := range s {
		if r <= 0xFFFF {
			count++
		} else {
			count += 2 // surrogate pair
		}
	}
	return count
}

// NeedsUTF16 returns true if the string contains characters outside Latin1.
func NeedsUTF16(s string) bool {
	for _, r := range s {
		if r > 255 {
			return true
		}
	}
	return false
}
