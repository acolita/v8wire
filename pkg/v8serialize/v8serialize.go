// Package v8serialize provides serialization and deserialization of V8's Structured Clone format.
//
// This format is used by Node.js v8.serialize() and v8.deserialize(), as well as
// various web APIs like postMessage, IndexedDB, and the Clipboard API.
//
// # Basic Usage
//
// Deserialize V8 data:
//
//	data := []byte{0xff, 0x0f, 0x49, 0x54} // V8-serialized int32(42)
//	val, err := v8serialize.Deserialize(data)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(val.AsInt32()) // 42
//
// Serialize Go values:
//
//	data, err := v8serialize.SerializeGo(map[string]interface{}{
//	    "message": "Hello from Go!",
//	    "numbers": []interface{}{1, 2, 3},
//	})
//
// # Supported Types
//
// The library supports all common JavaScript types including:
//   - Primitives: null, undefined, boolean, numbers (int32, double), BigInt, strings
//   - Objects: plain objects, arrays (dense and sparse with holes)
//   - Collections: Map, Set (preserving insertion order)
//   - Binary: ArrayBuffer, TypedArrays (Int8Array, Uint8Array, etc.)
//   - Special: Date, RegExp, Error, boxed primitives (new Number(), etc.)
//   - Circular references: self-referencing and mutual references (deserialize only)
//
// # Limitations
//
// The serializer does not currently support circular references. Attempting to
// serialize an object graph with cycles will cause a stack overflow. The deserializer
// fully supports circular references.
//
// ResizableArrayBuffer (V8 v14+) is not yet supported.
//
// # Compatibility
//
// Supported V8 serialization format versions: 13-15 (Node.js 18-22).
package v8serialize

import (
	"fmt"
)

// ToGo converts a Value to its closest Go equivalent:
//   - null, undefined, hole → nil
//   - boolean → bool
//   - int32 → int32
//   - uint32 → uint32
//   - double → float64
//   - BigInt → *big.Int
//   - string → string
//   - Date → time.Time
//   - Array → []interface{}
//   - Object → map[string]interface{}
//   - Map → map[interface{}]interface{} (note: non-string keys)
//   - Set → []interface{}
//   - ArrayBuffer → []byte
//   - TypedArray → *ArrayBufferView
//   - RegExp → *RegExp
//   - BoxedPrimitive → *BoxedPrimitive
func ToGo(v Value) interface{} {
	return toGo(v, make(map[*Value]interface{}))
}

func toGo(v Value, seen map[*Value]interface{}) interface{} {
	switch v.Type() {
	case TypeUndefined, TypeNull, TypeHole:
		return nil
	case TypeBool:
		return v.AsBool()
	case TypeInt32:
		return v.AsInt32()
	case TypeUint32:
		return v.AsUint32()
	case TypeDouble:
		return v.AsDouble()
	case TypeBigInt:
		return v.AsBigInt()
	case TypeString:
		return v.AsString()
	case TypeDate:
		return v.AsDate()
	case TypeObject:
		obj := v.AsObject()
		result := make(map[string]interface{}, len(obj))
		for k, val := range obj {
			result[k] = toGo(val, seen)
		}
		return result
	case TypeArray:
		arr := v.AsArray()
		result := make([]interface{}, len(arr))
		for i, val := range arr {
			if val.IsHole() {
				result[i] = nil // or could use a sentinel
			} else {
				result[i] = toGo(val, seen)
			}
		}
		return result
	case TypeMap:
		m := v.Interface().(*JSMap)
		result := make(map[interface{}]interface{}, len(m.Entries))
		for _, entry := range m.Entries {
			k := toGo(entry.Key, seen)
			val := toGo(entry.Value, seen)
			result[k] = val
		}
		return result
	case TypeSet:
		s := v.Interface().(*JSSet)
		result := make([]interface{}, len(s.Values))
		for i, val := range s.Values {
			result[i] = toGo(val, seen)
		}
		return result
	case TypeArrayBuffer:
		return v.Interface().([]byte)
	case TypeTypedArray:
		return v.Interface().(*ArrayBufferView)
	case TypeRegExp:
		return v.Interface().(*RegExp)
	case TypeBoxedPrimitive:
		return v.Interface().(*BoxedPrimitive)
	default:
		return v.Interface()
	}
}

// MustDeserialize deserializes V8 data and panics on error.
// Use this only when you're certain the data is valid.
func MustDeserialize(data []byte) Value {
	v, err := Deserialize(data)
	if err != nil {
		panic(fmt.Sprintf("v8serialize.MustDeserialize: %v", err))
	}
	return v
}

// IsValidV8Data checks if the data starts with a valid V8 serialization header.
// This is a quick check and doesn't validate the entire payload.
func IsValidV8Data(data []byte) bool {
	if len(data) < 2 {
		return false
	}
	if data[0] != tagVersion {
		return false
	}
	// Check version is in supported range
	version := uint32(data[1])
	if data[1]&0x80 != 0 {
		// Multi-byte varint, just check it starts reasonably
		return true
	}
	return version >= MinVersion && version <= MaxVersion
}
