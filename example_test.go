package v8wire_test

import (
	"fmt"
	"log"

	"github.com/acolita/v8wire/pkg/v8serialize"
)

func Example_deserializeInt32() {
	// V8-serialized int32(42): ff0f4954
	// - ff = version tag
	// - 0f = version 15
	// - 49 = 'I' = Int32 tag
	// - 54 = ZigZag(42) = 84 as varint
	data := []byte{0xff, 0x0f, 0x49, 0x54}

	val, err := v8serialize.Deserialize(data)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Type: %s\n", val.Type())
	fmt.Printf("Value: %d\n", val.AsInt32())
	// Output:
	// Type: int32
	// Value: 42
}

func Example_deserializeObject() {
	// V8-serialized {a: 1, b: 2}
	data := []byte{
		0xff, 0x0f, // version header
		0x6f,             // 'o' = begin object
		0x22, 0x01, 0x61, // one-byte string "a"
		0x49, 0x02, // int32(1) - ZigZag(1) = 2
		0x22, 0x01, 0x62, // one-byte string "b"
		0x49, 0x04, // int32(2) - ZigZag(2) = 4
		0x7b, 0x02, // '}' = end object, 2 properties
	}

	val, err := v8serialize.Deserialize(data)
	if err != nil {
		log.Fatal(err)
	}

	obj := val.AsObject()
	fmt.Printf("a = %d\n", obj["a"].AsInt32())
	fmt.Printf("b = %d\n", obj["b"].AsInt32())
	// Output:
	// a = 1
	// b = 2
}

func Example_toGo() {
	// V8-serialized [1, 2, 3]
	data := []byte{
		0xff, 0x0f, // version header
		0x41, 0x03, // 'A' = dense array, length 3
		0x49, 0x02, // int32(1)
		0x49, 0x04, // int32(2)
		0x49, 0x06, // int32(3)
		0x24, 0x00, 0x03, // '$' = end dense array
	}

	val, err := v8serialize.Deserialize(data)
	if err != nil {
		log.Fatal(err)
	}

	// Convert to native Go types
	arr := v8serialize.ToGo(val).([]interface{})
	fmt.Printf("Length: %d\n", len(arr))
	fmt.Printf("First element: %v\n", arr[0])
	// Output:
	// Length: 3
	// First element: 1
}

func Example_isValidV8Data() {
	validData := []byte{0xff, 0x0f, 0x30} // null
	invalidData := []byte{0x00, 0x01, 0x02}

	fmt.Printf("Valid: %v\n", v8serialize.IsValidV8Data(validData))
	fmt.Printf("Invalid: %v\n", v8serialize.IsValidV8Data(invalidData))
	// Output:
	// Valid: true
	// Invalid: false
}

func Example_serialize() {
	// Serialize a Go value to V8 format
	data, err := v8serialize.SerializeGo(map[string]interface{}{
		"name": "Alice",
		"age":  30,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Serialized %d bytes\n", len(data))
	fmt.Printf("Starts with version tag: %v\n", data[0] == 0xff)
	// Output:
	// Serialized 25 bytes
	// Starts with version tag: true
}

func Example_roundTrip() {
	// Create a Value and round-trip it
	original := v8serialize.String("Hello, 世界! 🌍")

	// Serialize
	data, err := v8serialize.Serialize(original)
	if err != nil {
		log.Fatal(err)
	}

	// Deserialize
	restored, err := v8serialize.Deserialize(data)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Original: %s\n", original.AsString())
	fmt.Printf("Restored: %s\n", restored.AsString())
	fmt.Printf("Match: %v\n", original.AsString() == restored.AsString())
	// Output:
	// Original: Hello, 世界! 🌍
	// Restored: Hello, 世界! 🌍
	// Match: true
}
