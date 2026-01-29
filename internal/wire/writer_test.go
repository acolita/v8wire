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
		name     string
		value    string
		expected []byte
	}{
		{"empty", "", []byte{}},
		{"ascii", "hello", []byte{'h', 'e', 'l', 'l', 'o'}},
		{"café-valid-utf8", "café", []byte{'c', 'a', 'f', 0xe9}}, // é (U+00E9) = 0xe9 in Latin1

		// Valid UTF-8 Latin-1 characters
		{"latin1-ä", "\u00E4", []byte{0xe4}}, // ä
		{"latin1-ö", "\u00F6", []byte{0xf6}}, // ö
		{"latin1-ü", "\u00FC", []byte{0xfc}}, // ü
		{"latin1-ÿ", "\u00FF", []byte{0xff}}, // ÿ (last Latin-1)
		{"latin1-mixed", "a\u00E4b", []byte{'a', 0xe4, 'b'}},

		// Invalid UTF-8 (raw bytes) - written directly
		{"raw-0xe4", "\xe4", []byte{0xe4}},
		{"raw-0xff", "\xff", []byte{0xff}},
		{"raw-mixed", "a\xe4b", []byte{'a', 0xe4, 'b'}},
		{"raw-high-bytes", "\x80\x81\x82", []byte{0x80, 0x81, 0x82}},

		// Edge cases
		{"null-byte", "a\x00b", []byte{'a', 0x00, 'b'}},
		{"ascii-127", "\x7f", []byte{0x7f}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewWriter(16)
			w.WriteOneByteString(tt.value)
			if !bytes.Equal(w.Bytes(), tt.expected) {
				t.Errorf("WriteOneByteString(%q) = %v, want %v", tt.value, w.Bytes(), tt.expected)
			}
		})
	}
}

func TestOneByteStringRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string // expected output (may differ for raw bytes due to UTF-8 normalization)
	}{
		{"empty", "", ""},
		{"ascii", "hello world", "hello world"},
		{"café", "café", "café"},
		{"all-latin1", "\u00C0\u00E4\u00FF", "\u00C0\u00E4\u00FF"}, // À ä ÿ

		// Invalid UTF-8 gets normalized to proper UTF-8:
		// Raw byte 0xe4 → Latin-1 char U+00E4 (ä) → UTF-8 "\xc3\xa4"
		{"raw-single", "\xe4", "\u00E4"},                // raw 0xe4 → ä
		{"raw-mixed", "a\xe4b\xf6c", "a\u00E4b\u00F6c"}, // raw bytes → proper UTF-8
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewWriter(16)
			w.WriteOneByteString(tt.input)

			length := OneByteStringLength(tt.input)
			r := NewReader(w.Bytes())
			got, err := r.ReadOneByteString(length)
			if err != nil {
				t.Fatalf("ReadOneByteString failed: %v", err)
			}

			// Reader converts Latin-1 bytes to proper UTF-8
			if got != tt.expected {
				t.Errorf("round-trip mismatch: got %q (%x), want %q (%x)",
					got, []byte(got), tt.expected, []byte(tt.expected))
			}
		})
	}
}

func TestOneByteStringInvalidUTF8Handling(t *testing.T) {
	// Test that invalid UTF-8 input is written as raw bytes (Latin-1)
	// and read back as proper UTF-8
	tests := []struct {
		name      string
		input     string // may contain invalid UTF-8
		wantBytes []byte // expected raw bytes written
		wantUTF8  string // expected UTF-8 after reading
	}{
		{
			name:      "raw-0xe4",
			input:     "\xe4",
			wantBytes: []byte{0xe4},
			wantUTF8:  "\u00E4", // ä
		},
		{
			name:      "raw-0xff",
			input:     "\xff",
			wantBytes: []byte{0xff},
			wantUTF8:  "\u00FF", // ÿ
		},
		{
			name:      "raw-0x80",
			input:     "\x80",
			wantBytes: []byte{0x80},
			wantUTF8:  "\u0080", // control char
		},
		{
			name:      "mixed-invalid",
			input:     "a\xe4b",
			wantBytes: []byte{'a', 0xe4, 'b'},
			wantUTF8:  "a\u00E4b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write
			w := NewWriter(16)
			w.WriteOneByteString(tt.input)

			if !bytes.Equal(w.Bytes(), tt.wantBytes) {
				t.Errorf("write: got %x, want %x", w.Bytes(), tt.wantBytes)
			}

			// Read
			r := NewReader(w.Bytes())
			got, err := r.ReadOneByteString(len(tt.wantBytes))
			if err != nil {
				t.Fatalf("read failed: %v", err)
			}

			if got != tt.wantUTF8 {
				t.Errorf("read: got %q (%x), want %q (%x)",
					got, []byte(got), tt.wantUTF8, []byte(tt.wantUTF8))
			}
		})
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
		name     string
		value    string
		expected bool
	}{
		{"empty", "", false},
		{"ascii", "hello", false},
		{"café", "café", false},            // é is U+00E9, fits in Latin1
		{"chinese", "你好", true},            // Chinese needs UTF-16
		{"emoji", "🌍", true},               // emoji needs UTF-16
		{"max-latin1", "\u00ff", false},    // U+00FF is max Latin1
		{"min-non-latin1", "\u0100", true}, // U+0100 exceeds Latin1

		// Edge cases for Latin-1 boundaries
		{"ascii-max", "\x7f", false},      // DEL character, ASCII max
		{"latin1-start", "\u0080", false}, // First Latin-1 extended char
		{"latin1-0xA0", "\u00A0", false},  // Non-breaking space
		{"latin1-0xBF", "\u00BF", false},  // Inverted question mark
		{"latin1-0xC0", "\u00C0", false},  // À
		{"latin1-0xE4", "\u00E4", false},  // ä
		{"latin1-0xFF", "\u00FF", false},  // ÿ (last Latin-1)

		// Invalid UTF-8 strings (raw bytes) - should use Latin-1
		{"raw-0x80", "\x80", false},    // Invalid UTF-8, valid Latin-1
		{"raw-0xE4", "\xe4", false},    // Invalid UTF-8, valid Latin-1
		{"raw-0xFF", "\xff", false},    // Invalid UTF-8, valid Latin-1
		{"raw-mixed", "a\xe4b", false}, // Mixed ASCII and raw byte

		// Strings that need UTF-16
		{"cyrillic", "Привет", true},     // Russian
		{"japanese", "こんにちは", true},      // Japanese
		{"mixed-emoji", "Hello 👋", true}, // ASCII + emoji
		{"math-symbols", "∑∏∫", true},    // Math symbols (> U+00FF)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedsUTF16(tt.value)
			if got != tt.expected {
				t.Errorf("NeedsUTF16(%q) = %v, want %v", tt.value, got, tt.expected)
			}
		})
	}
}

func TestOneByteStringLength(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected int
	}{
		{"empty", "", 0},
		{"ascii", "hello", 5},
		{"café", "café", 4},                          // 5 UTF-8 bytes, 4 runes
		{"latin1-single", "\u00E4", 1},               // ä is 1 rune (2 UTF-8 bytes)
		{"latin1-multiple", "\u00E4\u00F6\u00FC", 3}, // äöü = 3 runes

		// Invalid UTF-8 - should use byte count
		{"raw-single", "\xe4", 1},           // 1 raw byte
		{"raw-multiple", "\xe4\xf6\xfc", 3}, // 3 raw bytes
		{"raw-mixed", "a\xe4b", 3},          // 3 bytes (a + raw + b)

		// Edge cases
		{"null-byte", "a\x00b", 3},            // Null in middle
		{"all-high-bytes", "\x80\x81\x82", 3}, // All > 127, invalid UTF-8
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OneByteStringLength(tt.value)
			if got != tt.expected {
				t.Errorf("OneByteStringLength(%q) = %d, want %d", tt.value, got, tt.expected)
			}
		})
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
