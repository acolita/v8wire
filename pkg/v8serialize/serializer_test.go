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
