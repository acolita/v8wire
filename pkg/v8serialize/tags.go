// Package v8serialize implements deserialization of V8's Structured Clone format.
//
// This format is used by Node.js v8.serialize() and v8.deserialize().
package v8serialize

// V8 serialization format tags.
// These are extracted from V8 source: src/objects/value-serializer.cc
const (
	// Header tags
	tagVersion byte = 0xFF // Followed by version number as varint

	// Primitive tags
	tagNull      byte = '0' // 0x30
	tagUndefined byte = '_' // 0x5F
	tagTrue      byte = 'T' // 0x54
	tagFalse     byte = 'F' // 0x46
	tagInt32     byte = 'I' // 0x49 - followed by ZigZag-encoded varint
	tagUint32    byte = 'U' // 0x55 - followed by varint
	tagDouble    byte = 'N' // 0x4E - followed by IEEE 754 double (LE)
	tagBigInt    byte = 'Z' // 0x5A - followed by bitfield + digits
	tagDate      byte = 'D' // 0x44 - followed by double (ms since epoch)

	// String tags
	tagOneByteString byte = '"' // 0x22 - Latin1 string, length as varint
	tagTwoByteString byte = 'c' // 0x63 - UTF-16LE string, byte length as varint

	// Object/Array tags
	tagBeginJSObject    byte = 'o' // 0x6F - begin object literal
	tagEndJSObject      byte = '{' // 0x7B - end object (followed by property count)
	tagBeginDenseArray  byte = 'A' // 0x41 - begin dense array (followed by length)
	tagEndDenseArray    byte = '$' // 0x24 - end dense array (followed by property count, length)
	tagBeginSparseArray byte = 'a' // 0x61 - begin sparse array (followed by length)
	tagEndSparseArray   byte = '@' // 0x40 - end sparse array (followed by property count, length)
	tagHole             byte = '-' // 0x2D - array hole (sparse array element)

	// Reference tags
	tagObjectReference byte = '^' // 0x5E - back-reference to previously seen object

	// Collection tags
	tagBeginMap byte = ';'  // 0x3B - begin Map
	tagEndMap   byte = ':'  // 0x3A - end Map (followed by entry count * 2)
	tagBeginSet byte = '\'' // 0x27 - begin Set
	tagEndSet   byte = ','  // 0x2C - end Set (followed by entry count)

	// Binary data tags
	tagArrayBuffer          byte = 'B' // 0x42 - ArrayBuffer
	tagResizableArrayBuffer byte = '~' // 0x7E - ResizableArrayBuffer (v14+)
	tagArrayBufferTransfer  byte = 't' // 0x74 - transferred ArrayBuffer
	tagSharedArrayBuffer    byte = 'u' // 0x75 - SharedArrayBuffer

	// TypedArray tag (unified, type specified by sub-tag)
	tagTypedArray byte = '\\' // 0x5C - followed by type ID, byte length, data

	// TypedArray type identifiers (used after tagTypedArray)
	typedArrayInt8         byte = 0
	typedArrayUint8        byte = 1
	typedArrayUint8Clamped byte = 2
	typedArrayInt16        byte = 3
	typedArrayUint16       byte = 4
	typedArrayInt32        byte = 5
	typedArrayUint32       byte = 6
	typedArrayFloat32      byte = 7
	typedArrayFloat64      byte = 8
	typedArrayDataView     byte = 9
	typedArrayFloat16      byte = 10 // V8 12.x+ (Node 22+)
	typedArrayBigInt64     byte = 11
	typedArrayBigUint64    byte = 12

	// Special object tags
	tagRegExp       byte = 'R' // 0x52 - RegExp (pattern + flags)
	tagNumberObject byte = 'n' // 0x6E - boxed Number (followed by double)
	tagBigIntObject byte = 'z' // 0x7A - boxed BigInt
	tagTrueObject   byte = 'y' // 0x79 - boxed Boolean true
	tagFalseObject  byte = 'x' // 0x78 - boxed Boolean false
	tagStringObject byte = 's' // 0x73 - boxed String

	// Error tags (v15+)
	tagError byte = 'r' // 0x72 - Error object

	// Internal/Host tags
	tagHostObject byte = '\\' // 0x5C - host-defined object
	tagTheHole    byte = '-'  // internal V8 "the hole" value

	// Padding
	tagPadding byte = '\x00' // 0x00 - alignment padding
)

// Minimum and maximum supported serialization format versions.
const (
	MinVersion = 13 // Node.js 18.x
	MaxVersion = 15 // Node.js 22.x
)

// TagName returns a human-readable name for a tag byte.
func TagName(tag byte) string {
	switch tag {
	case tagVersion:
		return "Version"
	case tagNull:
		return "Null"
	case tagUndefined:
		return "Undefined"
	case tagTrue:
		return "True"
	case tagFalse:
		return "False"
	case tagInt32:
		return "Int32"
	case tagUint32:
		return "Uint32"
	case tagDouble:
		return "Double"
	case tagBigInt:
		return "BigInt"
	case tagDate:
		return "Date"
	case tagOneByteString:
		return "OneByteString"
	case tagTwoByteString:
		return "TwoByteString"
	case tagBeginJSObject:
		return "BeginJSObject"
	case tagEndJSObject:
		return "EndJSObject"
	case tagBeginDenseArray:
		return "BeginDenseArray"
	case tagEndDenseArray:
		return "EndDenseArray"
	case tagBeginSparseArray:
		return "BeginSparseArray"
	case tagEndSparseArray:
		return "EndSparseArray"
	case tagHole:
		return "Hole"
	case tagObjectReference:
		return "ObjectReference"
	case tagBeginMap:
		return "BeginMap"
	case tagEndMap:
		return "EndMap"
	case tagBeginSet:
		return "BeginSet"
	case tagEndSet:
		return "EndSet"
	case tagArrayBuffer:
		return "ArrayBuffer"
	case tagRegExp:
		return "RegExp"
	case tagNumberObject:
		return "NumberObject"
	case tagBigIntObject:
		return "BigIntObject"
	case tagTrueObject:
		return "TrueObject"
	case tagFalseObject:
		return "FalseObject"
	case tagStringObject:
		return "StringObject"
	case tagTypedArray: // Also tagHostObject (same byte value 0x5C)
		return "TypedArray"
	case tagError:
		return "Error"
	case tagPadding:
		return "Padding"
	default:
		return "Unknown"
	}
}
