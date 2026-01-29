package v8serialize

import (
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/acolita/v8wire/internal/wire"
)

// SerializeVersion is the V8 serialization format version we produce.
const SerializeVersion = 15

// Serializer serializes Go values to V8 Structured Clone format.
//
// LIMITATION: The current implementation does not support circular references.
// Attempting to serialize an object graph with cycles will cause a stack overflow.
// Use the deserializer's circular reference support to read such data, but avoid
// creating circular structures when serializing from Go.
type Serializer struct {
	writer  *wire.Writer
	objects map[interface{}]uint32 // object identity → reference ID (reserved for future circular ref support)
	nextID  uint32
}

// NewSerializer creates a new serializer.
func NewSerializer() *Serializer {
	return &Serializer{
		writer:  wire.NewWriter(256),
		objects: make(map[interface{}]uint32),
	}
}

// Serialize serializes a Value to V8 format.
func Serialize(v Value) ([]byte, error) {
	s := NewSerializer()
	return s.Serialize(v)
}

// SerializeGo serializes a Go value to V8 format.
// Supported types:
//   - nil → null
//   - bool → boolean
//   - int, int32, int64 → int32 or double
//   - uint, uint32, uint64 → uint32 or double
//   - float32, float64 → double
//   - string → string
//   - *big.Int → BigInt
//   - time.Time → Date
//   - []interface{} → array
//   - map[string]interface{} → object
//   - []byte → ArrayBuffer
func SerializeGo(v interface{}) ([]byte, error) {
	s := NewSerializer()
	return s.SerializeGo(v)
}

// Serialize serializes a Value.
func (s *Serializer) Serialize(v Value) ([]byte, error) {
	s.writeHeader()
	if err := s.writeValue(v); err != nil {
		return nil, err
	}
	return s.writer.Bytes(), nil
}

// SerializeGo serializes a Go value.
func (s *Serializer) SerializeGo(v interface{}) ([]byte, error) {
	s.writeHeader()
	if err := s.writeGoValue(v); err != nil {
		return nil, err
	}
	return s.writer.Bytes(), nil
}

func (s *Serializer) writeHeader() {
	s.writer.WriteByte(tagVersion)
	s.writer.WriteVarint32(SerializeVersion)
}

func (s *Serializer) writeValue(v Value) error {
	switch v.Type() {
	case TypeNull:
		s.writer.WriteByte(tagNull)
	case TypeUndefined:
		s.writer.WriteByte(tagUndefined)
	case TypeBool:
		if v.AsBool() {
			s.writer.WriteByte(tagTrue)
		} else {
			s.writer.WriteByte(tagFalse)
		}
	case TypeInt32:
		s.writer.WriteByte(tagInt32)
		s.writer.WriteZigZag32(v.AsInt32())
	case TypeUint32:
		s.writer.WriteByte(tagUint32)
		s.writer.WriteVarint32(v.AsUint32())
	case TypeDouble:
		s.writer.WriteByte(tagDouble)
		s.writer.WriteDouble(v.AsDouble())
	case TypeBigInt:
		return s.writeBigInt(v.AsBigInt())
	case TypeString:
		return s.writeString(v.AsString())
	case TypeDate:
		s.writer.WriteByte(tagDate)
		ms := float64(v.AsDate().UnixMilli())
		s.writer.WriteDouble(ms)
	case TypeObject:
		return s.writeObject(v.AsObject())
	case TypeArray:
		return s.writeArray(v.AsArray())
	case TypeMap:
		return s.writeMap(v.Interface().(*JSMap))
	case TypeSet:
		return s.writeSet(v.Interface().(*JSSet))
	case TypeArrayBuffer:
		return s.writeArrayBuffer(v.Interface().([]byte))
	case TypeRegExp:
		return s.writeRegExp(v.Interface().(*RegExp))
	case TypeError:
		return s.writeError(v.Interface().(*JSError))
	case TypeTypedArray:
		return s.writeTypedArray(v.Interface().(*ArrayBufferView))
	case TypeBoxedPrimitive:
		return s.writeBoxedPrimitive(v.Interface().(*BoxedPrimitive))
	case TypeHole:
		s.writer.WriteByte(tagHole)
	default:
		return fmt.Errorf("v8serialize: unsupported type %s", v.Type())
	}
	return nil
}

func (s *Serializer) writeGoValue(v interface{}) error {
	if v == nil {
		s.writer.WriteByte(tagNull)
		return nil
	}

	switch val := v.(type) {
	case bool:
		if val {
			s.writer.WriteByte(tagTrue)
		} else {
			s.writer.WriteByte(tagFalse)
		}
	case int:
		return s.writeInt(int64(val))
	case int8:
		return s.writeInt(int64(val))
	case int16:
		return s.writeInt(int64(val))
	case int32:
		s.writer.WriteByte(tagInt32)
		s.writer.WriteZigZag32(val)
	case int64:
		return s.writeInt(val)
	case uint:
		return s.writeUint(uint64(val))
	case uint8:
		return s.writeUint(uint64(val))
	case uint16:
		return s.writeUint(uint64(val))
	case uint32:
		if val <= math.MaxInt32 {
			s.writer.WriteByte(tagInt32)
			s.writer.WriteZigZag32(int32(val))
		} else {
			s.writer.WriteByte(tagDouble)
			s.writer.WriteDouble(float64(val))
		}
	case uint64:
		return s.writeUint(val)
	case float32:
		s.writer.WriteByte(tagDouble)
		s.writer.WriteDouble(float64(val))
	case float64:
		s.writer.WriteByte(tagDouble)
		s.writer.WriteDouble(val)
	case string:
		return s.writeString(val)
	case *big.Int:
		return s.writeBigInt(val)
	case time.Time:
		s.writer.WriteByte(tagDate)
		s.writer.WriteDouble(float64(val.UnixMilli()))
	case []byte:
		return s.writeArrayBuffer(val)
	case []interface{}:
		return s.writeGoArray(val)
	case map[string]interface{}:
		return s.writeGoObject(val)
	case Value:
		return s.writeValue(val)
	default:
		return fmt.Errorf("v8serialize: unsupported Go type %T", v)
	}
	return nil
}

func (s *Serializer) writeInt(n int64) error {
	if n >= math.MinInt32 && n <= math.MaxInt32 {
		s.writer.WriteByte(tagInt32)
		s.writer.WriteZigZag32(int32(n))
	} else {
		s.writer.WriteByte(tagDouble)
		s.writer.WriteDouble(float64(n))
	}
	return nil
}

func (s *Serializer) writeUint(n uint64) error {
	if n <= math.MaxInt32 {
		s.writer.WriteByte(tagInt32)
		s.writer.WriteZigZag32(int32(n))
	} else {
		s.writer.WriteByte(tagDouble)
		s.writer.WriteDouble(float64(n))
	}
	return nil
}

func (s *Serializer) writeString(str string) error {
	if wire.NeedsUTF16(str) {
		s.writer.WriteByte(tagTwoByteString)
		utf16Len := wire.UTF16Length(str)
		s.writer.WriteVarint32(uint32(utf16Len * 2)) // byte length
		s.writer.WriteTwoByteString(str)
	} else {
		s.writer.WriteByte(tagOneByteString)
		s.writer.WriteVarint32(uint32(len(str)))
		s.writer.WriteOneByteString(str)
	}
	return nil
}

func (s *Serializer) writeBigInt(n *big.Int) error {
	s.writer.WriteByte(tagBigInt)

	if n.Sign() == 0 {
		s.writer.WriteVarint(0) // bitfield: 0 digits, positive
		return nil
	}

	// Get absolute value bytes in big-endian
	absBytes := n.Bytes()

	// Calculate bitfield: bit 0 = sign, bits 1+ = byte length
	negative := n.Sign() < 0
	byteLen := uint64(len(absBytes))
	bitfield := byteLen << 1
	if negative {
		bitfield |= 1
	}
	s.writer.WriteVarint(bitfield)

	// Write bytes in little-endian order
	for i := len(absBytes) - 1; i >= 0; i-- {
		s.writer.WriteByte(absBytes[i])
	}

	return nil
}

func (s *Serializer) writeObject(obj map[string]Value) error {
	s.writer.WriteByte(tagBeginJSObject)

	for key, val := range obj {
		if err := s.writeString(key); err != nil {
			return err
		}
		if err := s.writeValue(val); err != nil {
			return err
		}
	}

	s.writer.WriteByte(tagEndJSObject)
	s.writer.WriteVarint32(uint32(len(obj)))
	return nil
}

func (s *Serializer) writeGoObject(obj map[string]interface{}) error {
	s.writer.WriteByte(tagBeginJSObject)

	for key, val := range obj {
		if err := s.writeString(key); err != nil {
			return err
		}
		if err := s.writeGoValue(val); err != nil {
			return err
		}
	}

	s.writer.WriteByte(tagEndJSObject)
	s.writer.WriteVarint32(uint32(len(obj)))
	return nil
}

func (s *Serializer) writeArray(arr []Value) error {
	s.writer.WriteByte(tagBeginDenseArray)
	s.writer.WriteVarint32(uint32(len(arr)))

	for _, elem := range arr {
		if err := s.writeValue(elem); err != nil {
			return err
		}
	}

	s.writer.WriteByte(tagEndDenseArray)
	s.writer.WriteVarint32(0) // no extra properties
	s.writer.WriteVarint32(uint32(len(arr)))
	return nil
}

func (s *Serializer) writeGoArray(arr []interface{}) error {
	s.writer.WriteByte(tagBeginDenseArray)
	s.writer.WriteVarint32(uint32(len(arr)))

	for _, elem := range arr {
		if err := s.writeGoValue(elem); err != nil {
			return err
		}
	}

	s.writer.WriteByte(tagEndDenseArray)
	s.writer.WriteVarint32(0) // no extra properties
	s.writer.WriteVarint32(uint32(len(arr)))
	return nil
}

func (s *Serializer) writeMap(m *JSMap) error {
	s.writer.WriteByte(tagBeginMap)

	for _, entry := range m.Entries {
		if err := s.writeValue(entry.Key); err != nil {
			return err
		}
		if err := s.writeValue(entry.Value); err != nil {
			return err
		}
	}

	s.writer.WriteByte(tagEndMap)
	s.writer.WriteVarint32(uint32(len(m.Entries) * 2))
	return nil
}

func (s *Serializer) writeSet(set *JSSet) error {
	s.writer.WriteByte(tagBeginSet)

	for _, val := range set.Values {
		if err := s.writeValue(val); err != nil {
			return err
		}
	}

	s.writer.WriteByte(tagEndSet)
	s.writer.WriteVarint32(uint32(len(set.Values)))
	return nil
}

func (s *Serializer) writeArrayBuffer(buf []byte) error {
	s.writer.WriteByte(tagArrayBuffer)
	s.writer.WriteVarint32(uint32(len(buf)))
	s.writer.WriteBytes(buf)
	return nil
}

func (s *Serializer) writeRegExp(re *RegExp) error {
	s.writer.WriteByte(tagRegExp)

	// Write pattern as string
	if err := s.writeString(re.Pattern); err != nil {
		return err
	}

	// Convert flags to bitfield
	var flags uint32
	for _, c := range re.Flags {
		switch c {
		case 'g':
			flags |= 1
		case 'i':
			flags |= 2
		case 'm':
			flags |= 4
		case 's':
			flags |= 8
		case 'u':
			flags |= 16
		case 'y':
			flags |= 32
		}
	}
	s.writer.WriteVarint32(flags)
	return nil
}

func (s *Serializer) writeError(jsErr *JSError) error {
	s.writer.WriteByte(tagError)

	// Determine error type tag
	switch jsErr.Name {
	case "Error":
		// Generic Error with message: 'm' serves as both type and message indicator
		s.writer.WriteByte(errorTypeErrorWithMessage)
		if err := s.writeString(jsErr.Message); err != nil {
			return err
		}
	case "EvalError":
		s.writer.WriteByte(errorTypeEvalError)
		s.writer.WriteByte(errorTagMessage)
		if err := s.writeString(jsErr.Message); err != nil {
			return err
		}
	case "RangeError":
		s.writer.WriteByte(errorTypeRangeError)
		s.writer.WriteByte(errorTagMessage)
		if err := s.writeString(jsErr.Message); err != nil {
			return err
		}
	case "ReferenceError":
		s.writer.WriteByte(errorTypeReferenceError)
		s.writer.WriteByte(errorTagMessage)
		if err := s.writeString(jsErr.Message); err != nil {
			return err
		}
	case "SyntaxError":
		s.writer.WriteByte(errorTypeSyntaxError)
		s.writer.WriteByte(errorTagMessage)
		if err := s.writeString(jsErr.Message); err != nil {
			return err
		}
	case "TypeError":
		s.writer.WriteByte(errorTypeTypeError)
		s.writer.WriteByte(errorTagMessage)
		if err := s.writeString(jsErr.Message); err != nil {
			return err
		}
	case "URIError":
		s.writer.WriteByte(errorTypeURIError)
		s.writer.WriteByte(errorTagMessage)
		if err := s.writeString(jsErr.Message); err != nil {
			return err
		}
	default:
		// Treat unknown error names as generic Error
		s.writer.WriteByte(errorTypeErrorWithMessage)
		if err := s.writeString(jsErr.Message); err != nil {
			return err
		}
	}

	// Write stack trace if present
	if jsErr.Stack != "" {
		s.writer.WriteByte(errorTagStack)
		if err := s.writeString(jsErr.Stack); err != nil {
			return err
		}
	}

	// Write cause if present
	if jsErr.Cause != nil {
		s.writer.WriteByte(errorTagCause)
		if err := s.writeValue(*jsErr.Cause); err != nil {
			return err
		}
	}

	// End of error
	s.writer.WriteByte(errorTagEnd)
	return nil
}

func (s *Serializer) writeTypedArray(view *ArrayBufferView) error {
	s.writer.WriteByte(tagTypedArray)

	// Determine type ID
	var typeID byte
	switch view.Type {
	case "Int8Array":
		typeID = typedArrayInt8
	case "Uint8Array":
		typeID = typedArrayUint8
	case "Uint8ClampedArray":
		typeID = typedArrayUint8Clamped
	case "Int16Array":
		typeID = typedArrayInt16
	case "Uint16Array":
		typeID = typedArrayUint16
	case "Int32Array":
		typeID = typedArrayInt32
	case "Uint32Array":
		typeID = typedArrayUint32
	case "Float32Array":
		typeID = typedArrayFloat32
	case "Float64Array":
		typeID = typedArrayFloat64
	case "DataView":
		typeID = typedArrayDataView
	case "Float16Array":
		typeID = typedArrayFloat16
	case "BigInt64Array":
		typeID = typedArrayBigInt64
	case "BigUint64Array":
		typeID = typedArrayBigUint64
	default:
		return fmt.Errorf("v8serialize: unknown TypedArray type %s", view.Type)
	}

	s.writer.WriteByte(typeID)
	s.writer.WriteVarint32(uint32(len(view.Buffer)))
	s.writer.WriteBytes(view.Buffer)
	return nil
}

func (s *Serializer) writeBoxedPrimitive(boxed *BoxedPrimitive) error {
	switch boxed.PrimitiveType {
	case TypeDouble:
		s.writer.WriteByte(tagNumberObject)
		s.writer.WriteDouble(boxed.Value.AsDouble())
	case TypeBool:
		if boxed.Value.AsBool() {
			s.writer.WriteByte(tagTrueObject)
		} else {
			s.writer.WriteByte(tagFalseObject)
		}
	case TypeString:
		s.writer.WriteByte(tagStringObject)
		return s.writeString(boxed.Value.AsString())
	case TypeBigInt:
		s.writer.WriteByte(tagBigIntObject)
		return s.writeBigInt(boxed.Value.AsBigInt())
	default:
		return fmt.Errorf("v8serialize: unsupported boxed primitive type %s", boxed.PrimitiveType)
	}
	return nil
}
