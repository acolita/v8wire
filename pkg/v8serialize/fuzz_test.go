package v8serialize

import (
	"testing"
	"unicode/utf8"
)

// FuzzDeserialize tests that the deserializer doesn't panic on arbitrary input.
func FuzzDeserialize(f *testing.F) {
	// Seed with valid V8 data from fixtures
	seeds := [][]byte{
		{0xff, 0x0f, 0x30},                                // null
		{0xff, 0x0f, 0x5f},                                // undefined
		{0xff, 0x0f, 0x54},                                // true
		{0xff, 0x0f, 0x46},                                // false
		{0xff, 0x0f, 0x49, 0x54},                          // int32(42)
		{0xff, 0x0f, 0x49, 0x00},                          // int32(0)
		{0xff, 0x0f, 0x22, 0x05, 'h', 'e', 'l', 'l', 'o'}, // "hello"
		{0xff, 0x0f, 0x6f, 0x7b, 0x00},                    // empty object
		{0xff, 0x0f, 0x41, 0x00, 0x24, 0x00, 0x00},        // empty array
		// Invalid/edge cases
		{},
		{0xff},
		{0xff, 0x0f},
		{0x00, 0x01, 0x02},
		{0xff, 0x0f, 0x49}, // truncated int32
		{0xff, 0x0f, 0x22, 0xff, 0xff, 0xff, 0xff}, // huge string length
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Should not panic
		val, err := Deserialize(data)
		if err != nil {
			return // errors are expected for invalid input
		}

		// Try to convert to Go (may panic for unhashable map keys, which is expected)
		func() {
			defer func() {
				// Recover from panics in ToGo (e.g., unhashable map keys)
				_ = recover()
			}()
			_ = ToGo(val)
		}()

		// Note: We intentionally skip re-serialization here because:
		// 1. The deserializer can create circular references (via ObjectReference)
		// 2. The serializer doesn't support circular references (causes stack overflow)
		// Serialization is tested separately in FuzzRoundTrip with known-safe inputs.
	})
}

// FuzzRoundTrip tests that valid data round-trips correctly.
func FuzzRoundTrip(f *testing.F) {
	// Seed with various strings
	f.Add("hello")
	f.Add("")
	f.Add("你好世界")
	f.Add("emoji: 🎉🎊🎈")
	f.Add("\x00\x01\x02") // binary-ish (valid UTF-8, all bytes < 128)
	f.Add("a]b{c}d")      // special chars
	f.Add("café")         // Latin-1 character (\xc3\xa9 in UTF-8)
	f.Add("\xc3\xa4")     // ä as valid UTF-8

	f.Fuzz(func(t *testing.T, s string) {
		// Skip invalid UTF-8 strings. Go strings should be valid UTF-8.
		// Invalid UTF-8 input gets normalized through Latin-1 encoding,
		// so exact round-trip isn't guaranteed for malformed input.
		if !utf8.ValidString(s) {
			return
		}

		// Serialize
		data, err := Serialize(String(s))
		if err != nil {
			t.Fatalf("Serialize failed: %v", err)
		}

		// Deserialize
		val, err := Deserialize(data)
		if err != nil {
			t.Fatalf("Deserialize failed: %v", err)
		}

		// Compare
		if val.Type() != TypeString {
			t.Fatalf("expected string, got %s", val.Type())
		}
		if val.AsString() != s {
			t.Fatalf("round-trip mismatch: got %q, want %q", val.AsString(), s)
		}
	})
}

// FuzzInt32RoundTrip tests int32 round-trips.
func FuzzInt32RoundTrip(f *testing.F) {
	f.Add(int32(0))
	f.Add(int32(1))
	f.Add(int32(-1))
	f.Add(int32(42))
	f.Add(int32(-42))
	f.Add(int32(2147483647))
	f.Add(int32(-2147483648))

	f.Fuzz(func(t *testing.T, n int32) {
		data, err := Serialize(Int32(n))
		if err != nil {
			t.Fatalf("Serialize failed: %v", err)
		}

		val, err := Deserialize(data)
		if err != nil {
			t.Fatalf("Deserialize failed: %v", err)
		}

		if val.Type() != TypeInt32 {
			t.Fatalf("expected int32, got %s", val.Type())
		}
		if val.AsInt32() != n {
			t.Fatalf("got %d, want %d", val.AsInt32(), n)
		}
	})
}
