package wire

import (
	"bytes"
	"math"
	"testing"
)

func TestWriteVarint(t *testing.T) {
	tests := []struct {
		value    uint64
		expected []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{127, []byte{0x7f}},
		{128, []byte{0x80, 0x01}},
		{255, []byte{0xff, 0x01}},
		{300, []byte{0xac, 0x02}},
		{16384, []byte{0x80, 0x80, 0x01}},
	}

	for _, tt := range tests {
		w := NewWriter(16)
		w.WriteVarint(tt.value)
		if !bytes.Equal(w.Bytes(), tt.expected) {
			t.Errorf("WriteVarint(%d) = %v, want %v", tt.value, w.Bytes(), tt.expected)
		}
	}
}

func TestZigZagEncode(t *testing.T) {
	tests := []struct {
		signed   int64
		unsigned uint64
	}{
		{0, 0},
		{-1, 1},
		{1, 2},
		{-2, 3},
		{2, 4},
		{42, 84},
		{-42, 83},
	}

	for _, tt := range tests {
		got := ZigZagEncode(tt.signed)
		if got != tt.unsigned {
			t.Errorf("ZigZagEncode(%d) = %d, want %d", tt.signed, got, tt.unsigned)
		}

		// Verify it round-trips
		decoded := ZigZagDecode(got)
		if decoded != tt.signed {
			t.Errorf("ZigZagDecode(%d) = %d, want %d", got, decoded, tt.signed)
		}
	}
}

func TestWriteDouble(t *testing.T) {
	tests := []struct {
		value    float64
		expected []byte
	}{
		{0, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
		{1, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf0, 0x3f}},
		{-1, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf0, 0xbf}},
	}

	for _, tt := range tests {
		w := NewWriter(16)
		w.WriteDouble(tt.value)
		if !bytes.Equal(w.Bytes(), tt.expected) {
			t.Errorf("WriteDouble(%v) = %v, want %v", tt.value, w.Bytes(), tt.expected)
		}

		// Verify round-trip
		r := NewReader(w.Bytes())
		got, err := r.ReadDouble()
		if err != nil {
			t.Fatalf("ReadDouble failed: %v", err)
		}
		if got != tt.value {
			t.Errorf("round-trip: got %v, want %v", got, tt.value)
		}
	}
}

func TestWriteOneByteString(t *testing.T) {
	tests := []struct {
		value    string
		expected []byte
	}{
		{"", []byte{}},
		{"hello", []byte{'h', 'e', 'l', 'l', 'o'}},
		{"café", []byte{'c', 'a', 'f', 0xe9}}, // é = 0xe9 in Latin1
	}

	for _, tt := range tests {
		w := NewWriter(16)
		w.WriteOneByteString(tt.value)
		if !bytes.Equal(w.Bytes(), tt.expected) {
			t.Errorf("WriteOneByteString(%q) = %v, want %v", tt.value, w.Bytes(), tt.expected)
		}
	}
}

func TestWriteTwoByteString(t *testing.T) {
	tests := []struct {
		value    string
		utf16Len int
	}{
		{"", 0},
		{"你好", 2},
		{"🌍", 2}, // surrogate pair
	}

	for _, tt := range tests {
		w := NewWriter(16)
		w.WriteTwoByteString(tt.value)

		// UTF-16 length should match
		gotLen := len(w.Bytes()) / 2
		if gotLen != tt.utf16Len {
			t.Errorf("WriteTwoByteString(%q) wrote %d code units, want %d", tt.value, gotLen, tt.utf16Len)
		}

		// Round-trip test
		r := NewReader(w.Bytes())
		got, err := r.ReadTwoByteString(tt.utf16Len)
		if err != nil {
			t.Fatalf("ReadTwoByteString failed: %v", err)
		}
		if got != tt.value {
			t.Errorf("round-trip: got %q, want %q", got, tt.value)
		}
	}
}

func TestUTF16Length(t *testing.T) {
	tests := []struct {
		value    string
		expected int
	}{
		{"", 0},
		{"hello", 5},
		{"你好", 2},
		{"🌍", 2},      // surrogate pair
		{"hello🌍", 7}, // 5 + 2
	}

	for _, tt := range tests {
		got := UTF16Length(tt.value)
		if got != tt.expected {
			t.Errorf("UTF16Length(%q) = %d, want %d", tt.value, got, tt.expected)
		}
	}
}

func TestNeedsUTF16(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"", false},
		{"hello", false},
		{"café", false},   // é is 0xe9, fits in Latin1
		{"你好", true},      // Chinese needs UTF-16
		{"🌍", true},       // emoji needs UTF-16
		{"\u00ff", false}, // 0xff is max Latin1 (using unicode escape)
		{"\u0100", true},  // 0x100 exceeds Latin1
	}

	for _, tt := range tests {
		got := NeedsUTF16(tt.value)
		if got != tt.expected {
			t.Errorf("NeedsUTF16(%q) = %v, want %v", tt.value, got, tt.expected)
		}
	}
}

func TestWriterReset(t *testing.T) {
	w := NewWriter(16)
	w.WriteByte(0x42)
	w.WriteByte(0x43)

	if w.Len() != 2 {
		t.Errorf("expected len 2, got %d", w.Len())
	}

	w.Reset()

	if w.Len() != 0 {
		t.Errorf("after reset, expected len 0, got %d", w.Len())
	}
}

func TestVarintRoundTrip(t *testing.T) {
	values := []uint64{0, 1, 127, 128, 255, 256, 16383, 16384, math.MaxUint32, math.MaxUint64}

	for _, v := range values {
		w := NewWriter(16)
		w.WriteVarint(v)

		r := NewReader(w.Bytes())
		got, err := r.ReadVarint()
		if err != nil {
			t.Fatalf("ReadVarint failed for %d: %v", v, err)
		}
		if got != v {
			t.Errorf("round-trip: got %d, want %d", got, v)
		}
	}
}

func TestZigZagRoundTrip(t *testing.T) {
	values := []int64{0, 1, -1, 42, -42, math.MaxInt32, math.MinInt32, math.MaxInt64, math.MinInt64}

	for _, v := range values {
		w := NewWriter(16)
		w.WriteZigZag(v)

		r := NewReader(w.Bytes())
		got, err := r.ReadZigZag()
		if err != nil {
			t.Fatalf("ReadZigZag failed for %d: %v", v, err)
		}
		if got != v {
			t.Errorf("round-trip: got %d, want %d", got, v)
		}
	}
}
