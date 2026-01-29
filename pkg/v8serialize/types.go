package v8serialize

import (
	"fmt"
	"math/big"
	"time"
)

// Type represents the type of a JavaScript value.
type Type uint8

const (
	TypeUndefined Type = iota
	TypeNull
	TypeBool
	TypeInt32
	TypeUint32
	TypeDouble
	TypeBigInt
	TypeString
	TypeDate
	TypeRegExp
	TypeObject
	TypeArray
	TypeMap
	TypeSet
	TypeArrayBuffer
	TypeTypedArray
	TypeDataView
	TypeHole           // Sparse array hole
	TypeError          // JavaScript Error object
	TypeBoxedPrimitive // Number/Boolean/String/BigInt object wrappers
)

// String returns the type name.
func (t Type) String() string {
	switch t {
	case TypeUndefined:
		return "undefined"
	case TypeNull:
		return "null"
	case TypeBool:
		return "boolean"
	case TypeInt32:
		return "int32"
	case TypeUint32:
		return "uint32"
	case TypeDouble:
		return "number"
	case TypeBigInt:
		return "bigint"
	case TypeString:
		return "string"
	case TypeDate:
		return "Date"
	case TypeRegExp:
		return "RegExp"
	case TypeObject:
		return "object"
	case TypeArray:
		return "Array"
	case TypeMap:
		return "Map"
	case TypeSet:
		return "Set"
	case TypeArrayBuffer:
		return "ArrayBuffer"
	case TypeTypedArray:
		return "TypedArray"
	case TypeDataView:
		return "DataView"
	case TypeHole:
		return "hole"
	case TypeError:
		return "Error"
	case TypeBoxedPrimitive:
		return "BoxedPrimitive"
	default:
		return fmt.Sprintf("Type(%d)", t)
	}
}

// Value represents a deserialized JavaScript value.
// Use the accessor methods to safely extract typed values.
type Value struct {
	typ  Type
	data interface{}
}

// Undefined returns a Value representing JavaScript undefined.
func Undefined() Value {
	return Value{typ: TypeUndefined}
}

// Null returns a Value representing JavaScript null.
func Null() Value {
	return Value{typ: TypeNull}
}

// Bool returns a Value representing a JavaScript boolean.
func Bool(b bool) Value {
	return Value{typ: TypeBool, data: b}
}

// Int32 returns a Value representing a JavaScript number (int32 range).
func Int32(n int32) Value {
	return Value{typ: TypeInt32, data: n}
}

// Uint32 returns a Value representing a JavaScript number (uint32 range).
func Uint32(n uint32) Value {
	return Value{typ: TypeUint32, data: n}
}

// Double returns a Value representing a JavaScript number (double).
func Double(f float64) Value {
	return Value{typ: TypeDouble, data: f}
}

// BigInt returns a Value representing a JavaScript BigInt.
func BigInt(n *big.Int) Value {
	return Value{typ: TypeBigInt, data: n}
}

// String returns a Value representing a JavaScript string.
func String(s string) Value {
	return Value{typ: TypeString, data: s}
}

// Date returns a Value representing a JavaScript Date.
func Date(t time.Time) Value {
	return Value{typ: TypeDate, data: t}
}

// Hole returns a Value representing an array hole.
func Hole() Value {
	return Value{typ: TypeHole}
}

// Object returns a Value representing a JavaScript object.
// If props is nil, creates an empty object.
func Object(props map[string]Value) Value {
	if props == nil {
		props = make(map[string]Value)
	}
	return Value{typ: TypeObject, data: props}
}

// Array returns a Value representing a JavaScript array.
// If elements is nil, creates an empty array.
func Array(elements []Value) Value {
	if elements == nil {
		elements = []Value{}
	}
	return Value{typ: TypeArray, data: elements}
}

// ArrayBuffer returns a Value representing a JavaScript ArrayBuffer.
func ArrayBuffer(data []byte) Value {
	if data == nil {
		data = []byte{}
	}
	return Value{typ: TypeArrayBuffer, data: data}
}

// Type returns the JavaScript type of this value.
func (v Value) Type() Type {
	return v.typ
}

// IsUndefined returns true if this value is JavaScript undefined.
func (v Value) IsUndefined() bool {
	return v.typ == TypeUndefined
}

// IsNull returns true if this value is JavaScript null.
func (v Value) IsNull() bool {
	return v.typ == TypeNull
}

// IsNullish returns true if this value is null or undefined.
func (v Value) IsNullish() bool {
	return v.typ == TypeNull || v.typ == TypeUndefined
}

// IsBool returns true if this value is a boolean.
func (v Value) IsBool() bool {
	return v.typ == TypeBool
}

// IsNumber returns true if this value is a number (int32, uint32, or double).
func (v Value) IsNumber() bool {
	return v.typ == TypeInt32 || v.typ == TypeUint32 || v.typ == TypeDouble
}

// IsBigInt returns true if this value is a BigInt.
func (v Value) IsBigInt() bool {
	return v.typ == TypeBigInt
}

// IsString returns true if this value is a string.
func (v Value) IsString() bool {
	return v.typ == TypeString
}

// IsDate returns true if this value is a Date.
func (v Value) IsDate() bool {
	return v.typ == TypeDate
}

// IsObject returns true if this value is an object (not null).
func (v Value) IsObject() bool {
	return v.typ == TypeObject
}

// IsArray returns true if this value is an array.
func (v Value) IsArray() bool {
	return v.typ == TypeArray
}

// IsHole returns true if this value represents an array hole.
func (v Value) IsHole() bool {
	return v.typ == TypeHole
}

// AsBool returns the boolean value. Panics if not a boolean.
func (v Value) AsBool() bool {
	if v.typ != TypeBool {
		panic(fmt.Sprintf("Value.AsBool: expected boolean, got %s", v.typ))
	}
	return v.data.(bool)
}

// AsInt32 returns the int32 value. Panics if not an int32.
func (v Value) AsInt32() int32 {
	if v.typ != TypeInt32 {
		panic(fmt.Sprintf("Value.AsInt32: expected int32, got %s", v.typ))
	}
	return v.data.(int32)
}

// AsUint32 returns the uint32 value. Panics if not a uint32.
func (v Value) AsUint32() uint32 {
	if v.typ != TypeUint32 {
		panic(fmt.Sprintf("Value.AsUint32: expected uint32, got %s", v.typ))
	}
	return v.data.(uint32)
}

// AsDouble returns the float64 value. Panics if not a double.
func (v Value) AsDouble() float64 {
	if v.typ != TypeDouble {
		panic(fmt.Sprintf("Value.AsDouble: expected double, got %s", v.typ))
	}
	return v.data.(float64)
}

// AsNumber returns the numeric value as float64.
// Works for int32, uint32, and double types.
func (v Value) AsNumber() float64 {
	switch v.typ {
	case TypeInt32:
		return float64(v.data.(int32))
	case TypeUint32:
		return float64(v.data.(uint32))
	case TypeDouble:
		return v.data.(float64)
	default:
		panic(fmt.Sprintf("Value.AsNumber: expected number, got %s", v.typ))
	}
}

// AsBigInt returns the big.Int value. Panics if not a BigInt.
func (v Value) AsBigInt() *big.Int {
	if v.typ != TypeBigInt {
		panic(fmt.Sprintf("Value.AsBigInt: expected bigint, got %s", v.typ))
	}
	return v.data.(*big.Int)
}

// AsString returns the string value. Panics if not a string.
func (v Value) AsString() string {
	if v.typ != TypeString {
		panic(fmt.Sprintf("Value.AsString: expected string, got %s", v.typ))
	}
	return v.data.(string)
}

// AsDate returns the time.Time value. Panics if not a Date.
func (v Value) AsDate() time.Time {
	if v.typ != TypeDate {
		panic(fmt.Sprintf("Value.AsDate: expected Date, got %s", v.typ))
	}
	return v.data.(time.Time)
}

// AsObject returns the object as map[string]Value. Panics if not an object.
func (v Value) AsObject() map[string]Value {
	if v.typ != TypeObject {
		panic(fmt.Sprintf("Value.AsObject: expected object, got %s", v.typ))
	}
	return v.data.(map[string]Value)
}

// AsArray returns the array as []Value. Panics if not an array.
func (v Value) AsArray() []Value {
	if v.typ != TypeArray {
		panic(fmt.Sprintf("Value.AsArray: expected array, got %s", v.typ))
	}
	return v.data.([]Value)
}

// Interface returns the underlying Go value.
// Returns nil for undefined and null.
func (v Value) Interface() interface{} {
	if v.typ == TypeUndefined || v.typ == TypeNull || v.typ == TypeHole {
		return nil
	}
	return v.data
}

// GoString implements fmt.GoStringer for debugging.
func (v Value) GoString() string {
	switch v.typ {
	case TypeUndefined:
		return "undefined"
	case TypeNull:
		return "null"
	case TypeBool:
		if v.data.(bool) {
			return "true"
		}
		return "false"
	case TypeInt32:
		return fmt.Sprintf("%d", v.data.(int32))
	case TypeUint32:
		return fmt.Sprintf("%d", v.data.(uint32))
	case TypeDouble:
		return fmt.Sprintf("%g", v.data.(float64))
	case TypeBigInt:
		return fmt.Sprintf("%sn", v.data.(*big.Int).String())
	case TypeString:
		return fmt.Sprintf("%q", v.data.(string))
	case TypeDate:
		return fmt.Sprintf("Date(%s)", v.data.(time.Time).Format(time.RFC3339Nano))
	case TypeHole:
		return "<hole>"
	case TypeObject:
		return fmt.Sprintf("Object{%d properties}", len(v.data.(map[string]Value)))
	case TypeArray:
		return fmt.Sprintf("Array[%d]", len(v.data.([]Value)))
	default:
		return fmt.Sprintf("%s(%v)", v.typ, v.data)
	}
}

// RegExp represents a JavaScript RegExp object.
type RegExp struct {
	Pattern string
	Flags   string
}

// MapEntry represents a key-value pair in a JavaScript Map.
// Maps preserve insertion order, so we use a slice of entries.
type MapEntry struct {
	Key   Value
	Value Value
}

// JSMap represents a JavaScript Map (preserves insertion order).
type JSMap struct {
	Entries []MapEntry
}

// JSSet represents a JavaScript Set (preserves insertion order).
type JSSet struct {
	Values []Value
}

// ArrayBufferView represents a typed view into an ArrayBuffer.
type ArrayBufferView struct {
	Buffer     []byte
	ByteOffset int
	ByteLength int
	Type       string // "Int8Array", "Uint8Array", etc.
}

// JSError represents a JavaScript Error object.
type JSError struct {
	Name    string
	Message string
	Stack   string
	Cause   *Value // ES2022 Error.cause (optional)
}

// BoxedPrimitive represents a boxed primitive (new Number(42), etc).
type BoxedPrimitive struct {
	PrimitiveType Type
	Value         Value
}
