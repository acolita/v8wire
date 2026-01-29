package v8serialize

import (
	"bytes"
	"math"
	"math/big"
	"testing"
	"time"
)

func TestSerializePrimitives(t *testing.T) {
	tests := []struct {
		name    string
		value   Value
		wantHex string
	}{
		{"null", Null(), "ff0f30"},
		{"undefined", Undefined(), "ff0f5f"},
		{"true", Bool(true), "ff0f54"},
		{"false", Bool(false), "ff0f46"},
		{"int32-zero", Int32(0), "ff0f4900"},
		{"int32-42", Int32(42), "ff0f4954"},
		{"int32-neg42", Int32(-42), "ff0f4953"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Serialize(tt.value)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}
			gotHex := bytesToHex(data)
			if gotHex != tt.wantHex {
				t.Errorf("got %s, want %s", gotHex, tt.wantHex)
			}
		})
	}
}

func TestSerializeRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		value Value
	}{
		{"null", Null()},
		{"undefined", Undefined()},
		{"true", Bool(true)},
		{"false", Bool(false)},
		{"int32-0", Int32(0)},
		{"int32-42", Int32(42)},
		{"int32-neg", Int32(-12345)},
		{"int32-max", Int32(math.MaxInt32)},
		{"int32-min", Int32(math.MinInt32)},
		{"double-pi", Double(math.Pi)},
		{"double-neg-zero", Double(math.Copysign(0, -1))},
		{"double-inf", Double(math.Inf(1))},
		{"string-empty", String("")},
		{"string-ascii", String("hello")},
		{"string-unicode", String("你好🌍")},
		{"hole", Hole()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			data, err := Serialize(tt.value)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			// Deserialize
			got, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			// Compare
			if !valuesEqual(got, tt.value) {
				t.Errorf("round-trip mismatch: got %#v, want %#v", got, tt.value)
			}
		})
	}
}

func TestSerializeBigInt(t *testing.T) {
	tests := []struct {
		name  string
		value *big.Int
	}{
		{"zero", big.NewInt(0)},
		{"42", big.NewInt(42)},
		{"neg42", big.NewInt(-42)},
		{"large", new(big.Int).SetUint64(math.MaxUint64)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Serialize(BigInt(tt.value))
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			got, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			if got.Type() != TypeBigInt {
				t.Fatalf("expected BigInt, got %s", got.Type())
			}
			if got.AsBigInt().Cmp(tt.value) != 0 {
				t.Errorf("got %s, want %s", got.AsBigInt(), tt.value)
			}
		})
	}
}

func TestSerializeDate(t *testing.T) {
	tests := []time.Time{
		time.Unix(0, 0).UTC(),
		time.Date(2024, 1, 15, 12, 30, 45, 123000000, time.UTC),
		time.Unix(-86400, 0).UTC(),
	}

	for _, tt := range tests {
		t.Run(tt.Format(time.RFC3339), func(t *testing.T) {
			data, err := Serialize(Date(tt))
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			got, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			if got.Type() != TypeDate {
				t.Fatalf("expected Date, got %s", got.Type())
			}

			// Compare milliseconds (V8 Date precision)
			wantMs := tt.UnixMilli()
			gotMs := got.AsDate().UnixMilli()
			if gotMs != wantMs {
				t.Errorf("got %d ms, want %d ms", gotMs, wantMs)
			}
		})
	}
}

func TestSerializeObjectRoundTrip(t *testing.T) {
	obj := map[string]Value{
		"a": Int32(1),
		"b": String("two"),
		"c": Bool(true),
	}
	v := Value{typ: TypeObject, data: obj}

	data, err := Serialize(v)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	got, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if got.Type() != TypeObject {
		t.Fatalf("expected Object, got %s", got.Type())
	}

	gotObj := got.AsObject()
	if gotObj["a"].AsInt32() != 1 {
		t.Errorf("a: expected 1, got %v", gotObj["a"])
	}
	if gotObj["b"].AsString() != "two" {
		t.Errorf("b: expected 'two', got %v", gotObj["b"])
	}
	if !gotObj["c"].AsBool() {
		t.Errorf("c: expected true")
	}
}

func TestSerializeArrayRoundTrip(t *testing.T) {
	arr := []Value{Int32(1), Int32(2), Int32(3)}
	v := Value{typ: TypeArray, data: arr}

	data, err := Serialize(v)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	got, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if got.Type() != TypeArray {
		t.Fatalf("expected Array, got %s", got.Type())
	}

	gotArr := got.AsArray()
	if len(gotArr) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(gotArr))
	}
	for i, expected := range []int32{1, 2, 3} {
		if gotArr[i].AsInt32() != expected {
			t.Errorf("arr[%d]: expected %d, got %v", i, expected, gotArr[i])
		}
	}
}

func TestSerializeGoValues(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
	}{
		{"nil", nil},
		{"bool-true", true},
		{"bool-false", false},
		{"int", 42},
		{"int32", int32(-100)},
		{"int64", int64(12345)},
		{"float64", 3.14159},
		{"string", "hello world"},
		{"bytes", []byte{1, 2, 3}},
		{"array", []interface{}{1, "two", true}},
		{"object", map[string]interface{}{"key": "value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := SerializeGo(tt.val)
			if err != nil {
				t.Fatalf("SerializeGo failed: %v", err)
			}

			_, err = Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}
		})
	}
}

func TestSerializeRegExp(t *testing.T) {
	re := &RegExp{Pattern: "test.*pattern", Flags: "gi"}
	v := Value{typ: TypeRegExp, data: re}

	data, err := Serialize(v)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	got, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if got.Type() != TypeRegExp {
		t.Fatalf("expected RegExp, got %s", got.Type())
	}

	gotRe := got.Interface().(*RegExp)
	if gotRe.Pattern != re.Pattern {
		t.Errorf("pattern: got %q, want %q", gotRe.Pattern, re.Pattern)
	}
	if gotRe.Flags != re.Flags {
		t.Errorf("flags: got %q, want %q", gotRe.Flags, re.Flags)
	}
}

func TestSerializeArrayBuffer(t *testing.T) {
	buf := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	v := Value{typ: TypeArrayBuffer, data: buf}

	data, err := Serialize(v)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	got, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if got.Type() != TypeArrayBuffer {
		t.Fatalf("expected ArrayBuffer, got %s", got.Type())
	}

	gotBuf := got.Interface().([]byte)
	if !bytes.Equal(gotBuf, buf) {
		t.Errorf("got %v, want %v", gotBuf, buf)
	}
}

// TestSerializeMatchesNodeJS verifies our output matches Node.js v8.serialize()
func TestSerializeMatchesNodeJS(t *testing.T) {
	tests := []struct {
		name    string
		value   Value
		fixture string // fixture name without extension
	}{
		{"null", Null(), "null"},
		{"undefined", Undefined(), "undefined"},
		{"true", Bool(true), "true"},
		{"false", Bool(false), "false"},
		{"int32-zero", Int32(0), "int32-zero"},
		{"int32-42", Int32(42), "int32-positive"},
		{"int32-neg42", Int32(-42), "int32-negative"},
		{"int32-max", Int32(2147483647), "int32-max"},
		{"int32-min", Int32(-2147483648), "int32-min"},
		{"string-empty", String(""), "string-empty"},
		{"string-hello", String("hello"), "string-onebyte"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load Node.js fixture
			nodeBin, meta := loadFixture(t, tt.fixture)

			// Serialize with our library
			goBin, err := Serialize(tt.value)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			// Compare
			if !bytes.Equal(goBin, nodeBin) {
				t.Errorf("output mismatch:\n  Go:   %s\n  Node: %s", bytesToHex(goBin), meta.HexDump)
			}
		})
	}
}

func TestSerializeStringEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		// ASCII
		{"empty", ""},
		{"single-char", "a"},
		{"ascii-printable", "Hello, World!"},
		{"ascii-with-null", "a\x00b"},
		{"ascii-control-chars", "\x01\x02\x03\x1f"},

		// Latin-1 (valid UTF-8 representation)
		{"latin1-café", "café"},
		{"latin1-äöü", "äöü"},
		{"latin1-0x80", "\u0080"}, // First Latin-1 extended
		{"latin1-0xFF", "\u00FF"}, // Last Latin-1 (ÿ)
		{"latin1-all-extended", "\u0080\u0090\u00A0\u00B0\u00C0\u00D0\u00E0\u00F0\u00FF"},

		// UTF-16 required
		{"chinese", "你好"},
		{"emoji-single", "🌍"},
		{"emoji-multiple", "👨‍👩‍👧‍👦"},
		{"mixed-ascii-emoji", "Hello 🌍 World"},
		{"cyrillic", "Привет"},
		{"japanese", "こんにちは"},
		{"math-symbols", "∑∏∫∂"},
		{"currency", "€£¥₹"},

		// Edge cases at encoding boundaries
		{"latin1-boundary", "\u00FF\u0100"}, // Last Latin-1 + first non-Latin-1
		{"surrogate-pair", "𝄞"},             // Musical G clef (U+1D11E)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			data, err := Serialize(String(tt.value))
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			// Deserialize
			got, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			// Verify
			if got.Type() != TypeString {
				t.Fatalf("expected String, got %s", got.Type())
			}
			if got.AsString() != tt.value {
				t.Errorf("round-trip mismatch:\n  got:  %q (%x)\n  want: %q (%x)",
					got.AsString(), []byte(got.AsString()),
					tt.value, []byte(tt.value))
			}
		})
	}
}

func TestSerializeStringLengthBoundaries(t *testing.T) {
	// Test various string lengths to catch varint encoding issues
	lengths := []int{0, 1, 127, 128, 255, 256, 1000, 16383, 16384}

	for _, length := range lengths {
		t.Run(string(rune('L'))+string(rune('='+rune(length%10))), func(t *testing.T) {
			// Create string of given length
			s := make([]byte, length)
			for i := range s {
				s[i] = 'a' + byte(i%26)
			}
			value := string(s)

			data, err := Serialize(String(value))
			if err != nil {
				t.Fatalf("Serialize failed for length %d: %v", length, err)
			}

			got, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed for length %d: %v", length, err)
			}

			if got.AsString() != value {
				t.Errorf("round-trip failed for length %d: got len=%d, want len=%d",
					length, len(got.AsString()), len(value))
			}
		})
	}
}

func TestSerializeMapRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		entries []MapEntry
	}{
		{"empty", nil},
		{"single-string-key", []MapEntry{
			{Key: String("key"), Value: Int32(42)},
		}},
		{"multiple-entries", []MapEntry{
			{Key: String("a"), Value: Int32(1)},
			{Key: String("b"), Value: Int32(2)},
			{Key: String("c"), Value: Int32(3)},
		}},
		{"non-string-keys", []MapEntry{
			{Key: Int32(1), Value: String("one")},
			{Key: Int32(2), Value: String("two")},
		}},
		{"mixed-key-types", []MapEntry{
			{Key: String("str"), Value: Int32(1)},
			{Key: Int32(42), Value: String("num")},
			{Key: Bool(true), Value: String("bool")},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &JSMap{Entries: tt.entries}
			v := Value{typ: TypeMap, data: m}

			data, err := Serialize(v)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			got, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			if got.Type() != TypeMap {
				t.Fatalf("expected Map, got %s", got.Type())
			}

			gotMap := got.Interface().(*JSMap)
			if len(gotMap.Entries) != len(tt.entries) {
				t.Fatalf("expected %d entries, got %d", len(tt.entries), len(gotMap.Entries))
			}
		})
	}
}

func TestSerializeSetRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		values []Value
	}{
		{"empty", nil},
		{"single", []Value{Int32(42)}},
		{"numbers", []Value{Int32(1), Int32(2), Int32(3)}},
		{"strings", []Value{String("a"), String("b"), String("c")}},
		{"mixed", []Value{Int32(1), String("two"), Bool(true), Null()}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &JSSet{Values: tt.values}
			v := Value{typ: TypeSet, data: s}

			data, err := Serialize(v)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			got, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			if got.Type() != TypeSet {
				t.Fatalf("expected Set, got %s", got.Type())
			}

			gotSet := got.Interface().(*JSSet)
			if len(gotSet.Values) != len(tt.values) {
				t.Fatalf("expected %d values, got %d", len(tt.values), len(gotSet.Values))
			}
		})
	}
}

func TestSerializeTypedArrayRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		typeName string
		data     []byte
	}{
		{"uint8-empty", "Uint8Array", nil},
		{"uint8-data", "Uint8Array", []byte{1, 2, 3, 4}},
		{"int8", "Int8Array", []byte{0xff, 0x00, 0x7f}},
		{"uint16", "Uint16Array", []byte{1, 0, 2, 0}},
		{"int16", "Int16Array", []byte{0xff, 0xff, 0x00, 0x01}},
		{"uint32", "Uint32Array", []byte{1, 0, 0, 0, 2, 0, 0, 0}},
		{"int32", "Int32Array", []byte{0xff, 0xff, 0xff, 0xff}},
		{"float32", "Float32Array", []byte{0, 0, 0x80, 0x3f}},             // 1.0
		{"float64", "Float64Array", []byte{0, 0, 0, 0, 0, 0, 0xf0, 0x3f}}, // 1.0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := &ArrayBufferView{
				Buffer:     tt.data,
				ByteOffset: 0,
				ByteLength: len(tt.data),
				Type:       tt.typeName,
			}
			v := Value{typ: TypeTypedArray, data: view}

			data, err := Serialize(v)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			got, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			if got.Type() != TypeTypedArray {
				t.Fatalf("expected TypedArray, got %s", got.Type())
			}

			gotView := got.Interface().(*ArrayBufferView)
			if gotView.Type != tt.typeName {
				t.Errorf("type: got %s, want %s", gotView.Type, tt.typeName)
			}
			if !bytes.Equal(gotView.Buffer, tt.data) {
				t.Errorf("data mismatch: got %v, want %v", gotView.Buffer, tt.data)
			}
		})
	}
}

func TestSerializeErrorRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		jsError *JSError
	}{
		{"simple", &JSError{Name: "Error", Message: "something went wrong"}},
		{"type-error", &JSError{Name: "TypeError", Message: "undefined is not a function"}},
		{"range-error", &JSError{Name: "RangeError", Message: "invalid array length"}},
		{"reference-error", &JSError{Name: "ReferenceError", Message: "x is not defined"}},
		{"syntax-error", &JSError{Name: "SyntaxError", Message: "unexpected token"}},
		{"with-stack", &JSError{Name: "Error", Message: "oops", Stack: "Error: oops\n    at test.js:1:1"}},
		{"empty-message", &JSError{Name: "Error", Message: ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Value{typ: TypeError, data: tt.jsError}

			data, err := Serialize(v)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			got, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			if got.Type() != TypeError {
				t.Fatalf("expected Error, got %s", got.Type())
			}

			gotErr := got.Interface().(*JSError)
			if gotErr.Name != tt.jsError.Name {
				t.Errorf("name: got %s, want %s", gotErr.Name, tt.jsError.Name)
			}
			if gotErr.Message != tt.jsError.Message {
				t.Errorf("message: got %q, want %q", gotErr.Message, tt.jsError.Message)
			}
		})
	}
}

func TestSerializeBoxedPrimitiveRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		boxed *BoxedPrimitive
	}{
		{"number-42", &BoxedPrimitive{PrimitiveType: TypeDouble, Value: Double(42)}},
		{"number-pi", &BoxedPrimitive{PrimitiveType: TypeDouble, Value: Double(3.14159)}},
		{"bool-true", &BoxedPrimitive{PrimitiveType: TypeBool, Value: Bool(true)}},
		{"bool-false", &BoxedPrimitive{PrimitiveType: TypeBool, Value: Bool(false)}},
		{"string", &BoxedPrimitive{PrimitiveType: TypeString, Value: String("wrapped")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Value{typ: TypeBoxedPrimitive, data: tt.boxed}

			data, err := Serialize(v)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			got, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			if got.Type() != TypeBoxedPrimitive {
				t.Fatalf("expected BoxedPrimitive, got %s", got.Type())
			}
		})
	}
}

func TestSerializeNestedStructures(t *testing.T) {
	// Deeply nested object
	t.Run("deep-nesting", func(t *testing.T) {
		// Create 50 levels of nesting
		var v Value = Int32(42)
		for i := 0; i < 50; i++ {
			obj := map[string]Value{"nested": v}
			v = Value{typ: TypeObject, data: obj}
		}

		data, err := Serialize(v)
		if err != nil {
			t.Fatalf("Serialize failed: %v", err)
		}

		got, err := Deserialize(data)
		if err != nil {
			t.Fatalf("Deserialize failed: %v", err)
		}

		// Walk down to the leaf
		for i := 0; i < 50; i++ {
			if got.Type() != TypeObject {
				t.Fatalf("level %d: expected Object, got %s", i, got.Type())
			}
			obj := got.AsObject()
			nested, ok := obj["nested"]
			if !ok {
				t.Fatalf("level %d: missing 'nested' key", i)
			}
			got = nested
		}

		if got.Type() != TypeInt32 || got.AsInt32() != 42 {
			t.Errorf("leaf: expected Int32(42), got %v", got)
		}
	})

	// Array of objects
	t.Run("array-of-objects", func(t *testing.T) {
		arr := make([]Value, 10)
		for i := range arr {
			obj := map[string]Value{
				"index": Int32(int32(i)),
				"name":  String("item"),
			}
			arr[i] = Value{typ: TypeObject, data: obj}
		}
		v := Value{typ: TypeArray, data: arr}

		data, err := Serialize(v)
		if err != nil {
			t.Fatalf("Serialize failed: %v", err)
		}

		got, err := Deserialize(data)
		if err != nil {
			t.Fatalf("Deserialize failed: %v", err)
		}

		if got.Type() != TypeArray {
			t.Fatalf("expected Array, got %s", got.Type())
		}
		if len(got.AsArray()) != 10 {
			t.Fatalf("expected 10 elements, got %d", len(got.AsArray()))
		}
	})
}

// Helper functions

func bytesToHex(b []byte) string {
	const hex = "0123456789abcdef"
	result := make([]byte, len(b)*2)
	for i, v := range b {
		result[i*2] = hex[v>>4]
		result[i*2+1] = hex[v&0x0f]
	}
	return string(result)
}

func valuesEqual(a, b Value) bool {
	if a.Type() != b.Type() {
		return false
	}
	switch a.Type() {
	case TypeNull, TypeUndefined, TypeHole:
		return true
	case TypeBool:
		return a.AsBool() == b.AsBool()
	case TypeInt32:
		return a.AsInt32() == b.AsInt32()
	case TypeUint32:
		return a.AsUint32() == b.AsUint32()
	case TypeDouble:
		af, bf := a.AsDouble(), b.AsDouble()
		if math.IsNaN(af) && math.IsNaN(bf) {
			return true
		}
		return af == bf
	case TypeString:
		return a.AsString() == b.AsString()
	default:
		return false // complex types need deeper comparison
	}
}
