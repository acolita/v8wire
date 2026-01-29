package v8serialize

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/acolita/v8wire/internal/wire"
)

// Common errors.
var (
	ErrInvalidHeader      = errors.New("v8serialize: invalid header")
	ErrUnsupportedVersion = errors.New("v8serialize: unsupported version")
	ErrUnexpectedTag      = errors.New("v8serialize: unexpected tag")
	ErrMalformedData      = errors.New("v8serialize: malformed data")
	ErrMaxDepthExceeded   = errors.New("v8serialize: max depth exceeded")
	ErrMaxSizeExceeded    = errors.New("v8serialize: max size exceeded")
	ErrInvalidReference   = errors.New("v8serialize: invalid object reference")
)

// Deserializer deserializes V8 Structured Clone format data.
type Deserializer struct {
	reader        *wire.Reader
	version       uint32
	maxDepth      int
	maxSize       int
	maxArrayLen   int
	maxObjectKeys int
	depth         int

	// Object reference table for circular references
	objects []Value
}

// DefaultMaxArrayLen is the default maximum array length (10 million elements).
// This prevents memory exhaustion from malicious input.
const DefaultMaxArrayLen = 10_000_000

// DefaultMaxObjectKeys is the default maximum object keys (1 million keys).
// This prevents memory exhaustion from malicious input.
const DefaultMaxObjectKeys = 1_000_000

// Option configures the deserializer.
type Option func(*Deserializer)

// WithMaxDepth sets the maximum nesting depth (default 1000).
func WithMaxDepth(depth int) Option {
	return func(d *Deserializer) {
		d.maxDepth = depth
	}
}

// WithMaxSize sets the maximum input size in bytes (default unlimited).
// Use this to prevent denial-of-service attacks from large inputs.
func WithMaxSize(size int) Option {
	return func(d *Deserializer) {
		d.maxSize = size
	}
}

// WithMaxArrayLen sets the maximum array length (default 10 million).
func WithMaxArrayLen(length int) Option {
	return func(d *Deserializer) {
		d.maxArrayLen = length
	}
}

// WithMaxObjectKeys sets the maximum number of object keys (default 1 million).
func WithMaxObjectKeys(keys int) Option {
	return func(d *Deserializer) {
		d.maxObjectKeys = keys
	}
}

// NewDeserializer creates a new deserializer for the given data.
func NewDeserializer(data []byte, opts ...Option) *Deserializer {
	d := &Deserializer{
		reader:        wire.NewReader(data),
		maxDepth:      1000,
		maxSize:       0, // 0 means unlimited
		maxArrayLen:   DefaultMaxArrayLen,
		maxObjectKeys: DefaultMaxObjectKeys,
		objects:       make([]Value, 0, 16),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Deserialize deserializes the data and returns the root value.
func Deserialize(data []byte, opts ...Option) (Value, error) {
	d := NewDeserializer(data, opts...)
	return d.Deserialize()
}

// Deserialize reads the header and deserializes the root value.
func (d *Deserializer) Deserialize() (Value, error) {
	// Check max size limit
	if d.maxSize > 0 && d.reader.Len() > d.maxSize {
		return Value{}, fmt.Errorf("%w: input size %d exceeds limit %d", ErrMaxSizeExceeded, d.reader.Len(), d.maxSize)
	}

	if err := d.readHeader(); err != nil {
		return Value{}, err
	}
	return d.readValue()
}

// Version returns the serialization format version (valid after Deserialize).
func (d *Deserializer) Version() uint32 {
	return d.version
}

// readHeader reads and validates the version header.
func (d *Deserializer) readHeader() error {
	// Read version tag
	tag, err := d.reader.ReadByte()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidHeader, err)
	}

	if tag != tagVersion {
		return fmt.Errorf("%w: expected version tag 0xFF, got 0x%02X", ErrInvalidHeader, tag)
	}

	// Read version number
	version, err := d.reader.ReadVarint32()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidHeader, err)
	}

	if version < MinVersion || version > MaxVersion {
		return fmt.Errorf("%w: version %d (supported: %d-%d)", ErrUnsupportedVersion, version, MinVersion, MaxVersion)
	}

	d.version = version
	return nil
}

// readValue reads a single value from the stream.
func (d *Deserializer) readValue() (Value, error) {
	// Check depth limit
	d.depth++
	if d.depth > d.maxDepth {
		return Value{}, ErrMaxDepthExceeded
	}
	defer func() { d.depth-- }()

	// Skip any padding bytes
	for {
		tag, err := d.reader.Peek()
		if err != nil {
			return Value{}, fmt.Errorf("%w: %v", ErrMalformedData, err)
		}
		if tag != tagPadding {
			break
		}
		_, _ = d.reader.ReadByte() // consume padding (already peeked)
	}

	tag, err := d.reader.ReadByte()
	if err != nil {
		return Value{}, fmt.Errorf("%w: %v", ErrMalformedData, err)
	}

	switch tag {
	// Primitives (no additional data)
	case tagNull:
		return Null(), nil
	case tagUndefined:
		return Undefined(), nil
	case tagTrue:
		return Bool(true), nil
	case tagFalse:
		return Bool(false), nil
	case tagHole:
		return Hole(), nil

	// Numbers
	case tagInt32:
		return d.readInt32()
	case tagUint32:
		return d.readUint32()
	case tagDouble:
		return d.readDouble()
	case tagBigInt:
		return d.readBigInt()

	// Strings
	case tagOneByteString:
		return d.readOneByteString()
	case tagTwoByteString:
		return d.readTwoByteString()

	// Date
	case tagDate:
		return d.readDate()

	// Objects and arrays
	case tagBeginJSObject:
		return d.readObject()
	case tagBeginDenseArray:
		return d.readDenseArray()
	case tagBeginSparseArray:
		return d.readSparseArray()

	// References
	case tagObjectReference:
		return d.readObjectReference()

	// Collections
	case tagBeginMap:
		return d.readMap()
	case tagBeginSet:
		return d.readSet()

	// Binary data
	case tagArrayBuffer:
		return d.readArrayBuffer()

	// TypedArrays
	case tagTypedArray:
		return d.readTypedArray()

	// Special objects
	case tagRegExp:
		return d.readRegExp()
	case tagNumberObject:
		return d.readNumberObject()
	case tagTrueObject:
		return d.readTrueObject()
	case tagFalseObject:
		return d.readFalseObject()
	case tagStringObject:
		return d.readStringObject()
	case tagBigIntObject:
		return d.readBigIntObject()

	// Error objects
	case tagError:
		return d.readError()

	default:
		return Value{}, fmt.Errorf("%w: unknown tag 0x%02X ('%c') at position %d",
			ErrUnexpectedTag, tag, tag, d.reader.Pos()-1)
	}
}

// readInt32 reads a ZigZag-encoded int32.
func (d *Deserializer) readInt32() (Value, error) {
	n, err := d.reader.ReadZigZag32()
	if err != nil {
		return Value{}, err
	}
	return Int32(n), nil
}

// readUint32 reads a varint-encoded uint32.
func (d *Deserializer) readUint32() (Value, error) {
	n, err := d.reader.ReadVarint32()
	if err != nil {
		return Value{}, err
	}
	return Uint32(n), nil
}

// readDouble reads an IEEE 754 double.
func (d *Deserializer) readDouble() (Value, error) {
	f, err := d.reader.ReadDouble()
	if err != nil {
		return Value{}, err
	}
	return Double(f), nil
}

// readBigInt reads a BigInt value.
// Format: bitfield (varint) + raw bytes (little-endian)
// Bitfield: bit 0 = sign (1 = negative), bits 1+ = byte length
func (d *Deserializer) readBigInt() (Value, error) {
	bitfield, err := d.reader.ReadVarint()
	if err != nil {
		return Value{}, err
	}

	negative := (bitfield & 1) == 1
	byteLength := bitfield >> 1

	if byteLength == 0 {
		return BigInt(big.NewInt(0)), nil
	}

	// Read raw bytes in little-endian order
	bytes, err := d.reader.ReadBytes(int(byteLength))
	if err != nil {
		return Value{}, err
	}

	// Convert little-endian bytes to big.Int
	// big.Int.SetBytes expects big-endian, so we reverse
	reversed := make([]byte, len(bytes))
	for i := 0; i < len(bytes); i++ {
		reversed[i] = bytes[len(bytes)-1-i]
	}

	result := new(big.Int).SetBytes(reversed)

	if negative {
		result.Neg(result)
	}

	return BigInt(result), nil
}

// readOneByteString reads a Latin1 encoded string.
func (d *Deserializer) readOneByteString() (Value, error) {
	length, err := d.reader.ReadVarint32()
	if err != nil {
		return Value{}, err
	}
	s, err := d.reader.ReadOneByteString(int(length))
	if err != nil {
		return Value{}, err
	}
	v := String(s)
	d.objects = append(d.objects, v) // strings are added to reference table
	return v, nil
}

// readTwoByteString reads a UTF-16LE encoded string.
func (d *Deserializer) readTwoByteString() (Value, error) {
	byteLength, err := d.reader.ReadVarint32()
	if err != nil {
		return Value{}, err
	}
	// Length is in bytes, convert to UTF-16 code units
	utf16Length := int(byteLength) / 2
	s, err := d.reader.ReadTwoByteString(utf16Length)
	if err != nil {
		return Value{}, err
	}
	v := String(s)
	d.objects = append(d.objects, v) // strings are added to reference table
	return v, nil
}

// readDate reads a JavaScript Date (ms since epoch as double).
func (d *Deserializer) readDate() (Value, error) {
	ms, err := d.reader.ReadDouble()
	if err != nil {
		return Value{}, err
	}
	// Convert milliseconds to time.Time
	sec := int64(ms / 1000)
	nsec := int64((ms - float64(sec)*1000) * 1e6)
	t := time.Unix(sec, nsec).UTC()
	v := Date(t)
	d.objects = append(d.objects, v) // dates are added to reference table
	return v, nil
}

// readObject reads a JavaScript object.
func (d *Deserializer) readObject() (Value, error) {
	obj := make(map[string]Value)
	v := Value{typ: TypeObject, data: obj}

	// Add to reference table immediately (for self-reference support)
	objIndex := len(d.objects)
	d.objects = append(d.objects, v)

	// Read properties until we see EndJSObject
	for {
		tag, err := d.reader.Peek()
		if err != nil {
			return Value{}, err
		}

		if tag == tagEndJSObject {
			_, _ = d.reader.ReadByte() // consume end tag (already peeked)
			// Read property count (for validation)
			_, err := d.reader.ReadVarint32()
			if err != nil {
				return Value{}, err
			}
			break
		}

		// Read key (can be string or number for integer keys)
		key, err := d.readValue()
		if err != nil {
			return Value{}, err
		}

		// Convert key to string
		var keyStr string
		switch key.Type() {
		case TypeString:
			keyStr = key.AsString()
		case TypeInt32:
			keyStr = fmt.Sprintf("%d", key.AsInt32())
		case TypeUint32:
			keyStr = fmt.Sprintf("%d", key.AsUint32())
		case TypeDouble:
			keyStr = fmt.Sprintf("%g", key.AsDouble())
		default:
			return Value{}, fmt.Errorf("%w: object key must be string or number, got %s", ErrMalformedData, key.Type())
		}

		// Read value
		val, err := d.readValue()
		if err != nil {
			return Value{}, err
		}

		obj[keyStr] = val
	}

	// Update the stored reference with populated object
	d.objects[objIndex] = v
	return v, nil
}

// readDenseArray reads a dense JavaScript array.
func (d *Deserializer) readDenseArray() (Value, error) {
	length, err := d.reader.ReadVarint32()
	if err != nil {
		return Value{}, err
	}

	// Check array length limit
	if int(length) > d.maxArrayLen {
		return Value{}, fmt.Errorf("%w: array length %d exceeds limit %d", ErrMalformedData, length, d.maxArrayLen)
	}

	arr := make([]Value, 0, length)
	v := Value{typ: TypeArray, data: arr}

	// Add to reference table immediately
	arrIndex := len(d.objects)
	d.objects = append(d.objects, v)

	// Read elements
	for i := uint32(0); i < length; i++ {
		elem, err := d.readValue()
		if err != nil {
			return Value{}, err
		}
		arr = append(arr, elem)
	}

	// Read any additional properties (arrays can have properties in JS)
	for {
		tag, err := d.reader.Peek()
		if err != nil {
			return Value{}, err
		}

		if tag == tagEndDenseArray {
			_, _ = d.reader.ReadByte() // consume end tag (already peeked)
			// Read property count and length
			_, err := d.reader.ReadVarint32() // properties
			if err != nil {
				return Value{}, err
			}
			_, err = d.reader.ReadVarint32() // length
			if err != nil {
				return Value{}, err
			}
			break
		}

		// Skip property (key + value)
		_, err = d.readValue() // key
		if err != nil {
			return Value{}, err
		}
		_, err = d.readValue() // value
		if err != nil {
			return Value{}, err
		}
	}

	v.data = arr
	d.objects[arrIndex] = v
	return v, nil
}

// readSparseArray reads a sparse JavaScript array.
func (d *Deserializer) readSparseArray() (Value, error) {
	length, err := d.reader.ReadVarint32()
	if err != nil {
		return Value{}, err
	}

	// Check array length limit
	if int(length) > d.maxArrayLen {
		return Value{}, fmt.Errorf("%w: array length %d exceeds limit %d", ErrMalformedData, length, d.maxArrayLen)
	}

	// Create array filled with holes
	arr := make([]Value, length)
	for i := range arr {
		arr[i] = Hole()
	}

	v := Value{typ: TypeArray, data: arr}

	// Add to reference table immediately
	arrIndex := len(d.objects)
	d.objects = append(d.objects, v)

	// Read index-value pairs until end tag
	for {
		tag, err := d.reader.Peek()
		if err != nil {
			return Value{}, err
		}

		if tag == tagEndSparseArray {
			_, _ = d.reader.ReadByte() // consume end tag (already peeked)
			// Read property count and length
			_, err := d.reader.ReadVarint32() // properties
			if err != nil {
				return Value{}, err
			}
			_, err = d.reader.ReadVarint32() // length
			if err != nil {
				return Value{}, err
			}
			break
		}

		// Read index (could be number or string)
		key, err := d.readValue()
		if err != nil {
			return Value{}, err
		}

		// Read value
		val, err := d.readValue()
		if err != nil {
			return Value{}, err
		}

		// If key is a number in range, set the array element
		if key.IsNumber() {
			idx := int(key.AsNumber())
			if idx >= 0 && idx < len(arr) {
				arr[idx] = val
			}
		}
		// Non-numeric keys are array properties (ignored for now)
	}

	v.data = arr
	d.objects[arrIndex] = v
	return v, nil
}

// readObjectReference reads a back-reference to a previously seen object.
func (d *Deserializer) readObjectReference() (Value, error) {
	id, err := d.reader.ReadVarint32()
	if err != nil {
		return Value{}, err
	}

	if int(id) >= len(d.objects) {
		return Value{}, fmt.Errorf("%w: reference %d (only %d objects seen)", ErrInvalidReference, id, len(d.objects))
	}

	return d.objects[id], nil
}

// readMap reads a JavaScript Map.
func (d *Deserializer) readMap() (Value, error) {
	entries := make([]MapEntry, 0)
	jsMap := &JSMap{Entries: entries}
	v := Value{typ: TypeMap, data: jsMap}

	// Add to reference table immediately
	mapIndex := len(d.objects)
	d.objects = append(d.objects, v)

	// Read key-value pairs until end tag
	for {
		tag, err := d.reader.Peek()
		if err != nil {
			return Value{}, err
		}

		if tag == tagEndMap {
			_, _ = d.reader.ReadByte() // consume end tag (already peeked)
			// Read entry count * 2
			_, err := d.reader.ReadVarint32()
			if err != nil {
				return Value{}, err
			}
			break
		}

		key, err := d.readValue()
		if err != nil {
			return Value{}, err
		}

		val, err := d.readValue()
		if err != nil {
			return Value{}, err
		}

		entries = append(entries, MapEntry{Key: key, Value: val})
	}

	jsMap.Entries = entries
	d.objects[mapIndex] = v
	return v, nil
}

// readSet reads a JavaScript Set.
func (d *Deserializer) readSet() (Value, error) {
	values := make([]Value, 0)
	jsSet := &JSSet{Values: values}
	v := Value{typ: TypeSet, data: jsSet}

	// Add to reference table immediately
	setIndex := len(d.objects)
	d.objects = append(d.objects, v)

	// Read values until end tag
	for {
		tag, err := d.reader.Peek()
		if err != nil {
			return Value{}, err
		}

		if tag == tagEndSet {
			_, _ = d.reader.ReadByte() // consume end tag (already peeked)
			// Read entry count
			_, err := d.reader.ReadVarint32()
			if err != nil {
				return Value{}, err
			}
			break
		}

		val, err := d.readValue()
		if err != nil {
			return Value{}, err
		}

		values = append(values, val)
	}

	jsSet.Values = values
	d.objects[setIndex] = v
	return v, nil
}

// readArrayBuffer reads an ArrayBuffer.
func (d *Deserializer) readArrayBuffer() (Value, error) {
	byteLength, err := d.reader.ReadVarint32()
	if err != nil {
		return Value{}, err
	}

	data, err := d.reader.ReadBytes(int(byteLength))
	if err != nil {
		return Value{}, err
	}

	// Copy the data to avoid referencing the original buffer
	buf := make([]byte, len(data))
	copy(buf, data)

	v := Value{typ: TypeArrayBuffer, data: buf}
	d.objects = append(d.objects, v)
	return v, nil
}

// readRegExp reads a JavaScript RegExp.
func (d *Deserializer) readRegExp() (Value, error) {
	// Read pattern (string)
	pattern, err := d.readValue()
	if err != nil {
		return Value{}, err
	}
	if !pattern.IsString() {
		return Value{}, fmt.Errorf("%w: regexp pattern must be string", ErrMalformedData)
	}

	// Read flags as varint (bitfield)
	flagBits, err := d.reader.ReadVarint32()
	if err != nil {
		return Value{}, err
	}

	// Convert flag bits to string
	var flags string
	if flagBits&1 != 0 {
		flags += "g" // global
	}
	if flagBits&2 != 0 {
		flags += "i" // ignoreCase
	}
	if flagBits&4 != 0 {
		flags += "m" // multiline
	}
	if flagBits&8 != 0 {
		flags += "s" // dotAll (ES2018)
	}
	if flagBits&16 != 0 {
		flags += "u" // unicode
	}
	if flagBits&32 != 0 {
		flags += "y" // sticky
	}

	re := &RegExp{
		Pattern: pattern.AsString(),
		Flags:   flags,
	}

	v := Value{typ: TypeRegExp, data: re}
	d.objects = append(d.objects, v)
	return v, nil
}

// readTypedArray reads a TypedArray (Uint8Array, Int32Array, etc.)
func (d *Deserializer) readTypedArray() (Value, error) {
	// Read TypedArray type
	arrayType, err := d.reader.ReadByte()
	if err != nil {
		return Value{}, err
	}

	// Read byte length
	byteLength, err := d.reader.ReadVarint32()
	if err != nil {
		return Value{}, err
	}

	// Read raw data
	data, err := d.reader.ReadBytes(int(byteLength))
	if err != nil {
		return Value{}, err
	}

	// Copy data
	buf := make([]byte, len(data))
	copy(buf, data)

	// Determine type name
	var typeName string
	switch arrayType {
	case typedArrayInt8:
		typeName = "Int8Array"
	case typedArrayUint8:
		typeName = "Uint8Array"
	case typedArrayUint8Clamped:
		typeName = "Uint8ClampedArray"
	case typedArrayInt16:
		typeName = "Int16Array"
	case typedArrayUint16:
		typeName = "Uint16Array"
	case typedArrayInt32:
		typeName = "Int32Array"
	case typedArrayUint32:
		typeName = "Uint32Array"
	case typedArrayFloat32:
		typeName = "Float32Array"
	case typedArrayFloat64:
		typeName = "Float64Array"
	case typedArrayDataView:
		typeName = "DataView"
	case typedArrayFloat16:
		typeName = "Float16Array"
	case typedArrayBigInt64:
		typeName = "BigInt64Array"
	case typedArrayBigUint64:
		typeName = "BigUint64Array"
	default:
		typeName = fmt.Sprintf("TypedArray(%d)", arrayType)
	}

	view := &ArrayBufferView{
		Buffer:     buf,
		ByteOffset: 0,
		ByteLength: len(buf),
		Type:       typeName,
	}

	v := Value{typ: TypeTypedArray, data: view}
	d.objects = append(d.objects, v)
	return v, nil
}

// readNumberObject reads a boxed Number (contains double directly).
func (d *Deserializer) readNumberObject() (Value, error) {
	f, err := d.reader.ReadDouble()
	if err != nil {
		return Value{}, err
	}

	boxed := &BoxedPrimitive{
		PrimitiveType: TypeDouble,
		Value:         Double(f),
	}
	v := Value{typ: TypeBoxedPrimitive, data: boxed}
	d.objects = append(d.objects, v)
	return v, nil
}

// readTrueObject reads a boxed Boolean true.
func (d *Deserializer) readTrueObject() (Value, error) {
	boxed := &BoxedPrimitive{
		PrimitiveType: TypeBool,
		Value:         Bool(true),
	}
	v := Value{typ: TypeBoxedPrimitive, data: boxed}
	d.objects = append(d.objects, v)
	return v, nil
}

// readFalseObject reads a boxed Boolean false.
func (d *Deserializer) readFalseObject() (Value, error) {
	boxed := &BoxedPrimitive{
		PrimitiveType: TypeBool,
		Value:         Bool(false),
	}
	v := Value{typ: TypeBoxedPrimitive, data: boxed}
	d.objects = append(d.objects, v)
	return v, nil
}

// readStringObject reads a boxed String.
func (d *Deserializer) readStringObject() (Value, error) {
	inner, err := d.readValue()
	if err != nil {
		return Value{}, err
	}

	// Validate that inner is actually a String
	if inner.Type() != TypeString {
		return Value{}, fmt.Errorf("%w: boxed String contains %s, not String", ErrMalformedData, inner.Type())
	}

	boxed := &BoxedPrimitive{
		PrimitiveType: TypeString,
		Value:         inner,
	}
	v := Value{typ: TypeBoxedPrimitive, data: boxed}
	d.objects = append(d.objects, v)
	return v, nil
}

// readBigIntObject reads a boxed BigInt.
func (d *Deserializer) readBigIntObject() (Value, error) {
	inner, err := d.readValue()
	if err != nil {
		return Value{}, err
	}

	// Validate that inner is actually a BigInt
	if inner.Type() != TypeBigInt {
		return Value{}, fmt.Errorf("%w: boxed BigInt contains %s, not BigInt", ErrMalformedData, inner.Type())
	}

	boxed := &BoxedPrimitive{
		PrimitiveType: TypeBigInt,
		Value:         inner,
	}
	v := Value{typ: TypeBoxedPrimitive, data: boxed}
	d.objects = append(d.objects, v)
	return v, nil
}

// Error sub-tags
const (
	errorTagMessage byte = 'm' // 0x6d - message follows
	errorTagStack   byte = 's' // 0x73 - stack follows
	errorTagCause   byte = 'c' // 0x63 - cause follows
	errorTagEnd     byte = '.' // 0x2e - end of error
)

// Error type tags (after the 'r' tag)
const (
	// 'm' (0x6d) is special: for generic Error, it means "Error with message"
	// and the message string follows directly (no separate message sub-tag)
	errorTypeErrorWithMessage byte = 'm' // 0x6d - generic Error + message follows
	errorTypeEvalError        byte = 'E' // 0x45
	errorTypeRangeError       byte = 'R' // 0x52
	errorTypeReferenceError   byte = 'F' // 0x46
	errorTypeSyntaxError      byte = 'S' // 0x53
	errorTypeTypeError        byte = 'T' // 0x54
	errorTypeURIError         byte = 'U' // 0x55
)

// readError reads a JavaScript Error object.
// Format varies:
// - Generic Error with message: 'r' + 'm' + message_string + ('s' + stack_string)? + '.'
// - Typed errors: 'r' + type + 'm' + message_string + ('s' + stack_string)? + '.'
func (d *Deserializer) readError() (Value, error) {
	// Read error type indicator
	errType, err := d.reader.ReadByte()
	if err != nil {
		return Value{}, err
	}

	jsErr := &JSError{}

	// Handle the special case where 'm' is both type (generic Error) AND
	// indicates that message follows directly
	if errType == errorTypeErrorWithMessage {
		jsErr.Name = "Error"
		// Message string follows directly (no sub-tag)
		val, err := d.readValue()
		if err != nil {
			return Value{}, err
		}
		if val.IsString() {
			jsErr.Message = val.AsString()
		}
	} else {
		// Map error type to name
		switch errType {
		case errorTypeEvalError:
			jsErr.Name = "EvalError"
		case errorTypeRangeError:
			jsErr.Name = "RangeError"
		case errorTypeReferenceError:
			jsErr.Name = "ReferenceError"
		case errorTypeSyntaxError:
			jsErr.Name = "SyntaxError"
		case errorTypeTypeError:
			jsErr.Name = "TypeError"
		case errorTypeURIError:
			jsErr.Name = "URIError"
		default:
			jsErr.Name = "Error"
		}
	}

	// Read remaining sub-tags until we hit the end tag
	for {
		subTag, err := d.reader.ReadByte()
		if err != nil {
			return Value{}, err
		}

		if subTag == errorTagEnd {
			break
		}

		// Read the value following the sub-tag
		val, err := d.readValue()
		if err != nil {
			return Value{}, err
		}

		switch subTag {
		case errorTagMessage:
			if val.IsString() {
				jsErr.Message = val.AsString()
			}
		case errorTagStack:
			if val.IsString() {
				jsErr.Stack = val.AsString()
			}
		case errorTagCause:
			// Cause is another value (usually an Error)
			jsErr.Cause = &val
		}
	}

	v := Value{typ: TypeError, data: jsErr}
	d.objects = append(d.objects, v)
	return v, nil
}
