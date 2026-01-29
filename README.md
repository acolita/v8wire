# v8wire

A Go library for serializing and deserializing V8's Structured Clone format, compatible with Node.js `v8.serialize()` and `v8.deserialize()`.

## Features

- **Full V8 format support**: Primitives, objects, arrays, Maps, Sets, TypedArrays, RegExp, Date, BigInt
- **Circular references**: Handles self-referencing and mutually referencing objects
- **Byte-compatible**: Output matches Node.js `v8.serialize()` exactly
- **Round-trip safe**: Serialize → Deserialize → Serialize produces identical bytes
- **No dependencies**: Pure Go implementation

## Use Cases

### Go ↔ Node.js Communication
Exchange complex data between Go services and Node.js processes without JSON overhead:
```go
// Go service receives data serialized by Node.js
data := receiveFromNodeProcess()
val, _ := v8serialize.Deserialize(data)
```

### Redis Session/Cache Sharing
Node.js apps often store `v8.serialize()` data in Redis. Go services can read/write the same cache:
```go
// Read session data written by Node.js
data, _ := redis.Get("session:user123")
session, _ := v8serialize.Deserialize(data)
```

### Message Queue Interop
Process messages from Node.js workers in Go consumers (or vice versa) via RabbitMQ, Kafka, NATS, etc.

### File Format Parsing
Read files or databases that store V8-serialized data (e.g., Electron app data, LevelDB stores used by Node apps).

### Migration/ETL Pipelines
Migrate data from Node.js systems to Go systems while preserving JavaScript semantics (BigInt, Date, typed arrays, circular refs).

### Why Not Just Use JSON?

| Feature | JSON | V8 Structured Clone |
|---------|------|---------------------|
| Binary data | Base64 (33% overhead) | Native `ArrayBuffer` |
| BigInt | Not supported | Native |
| Circular refs | Error | Handled |
| Date | String (lossy) | Preserved |
| `undefined` | Not supported | Preserved |
| Typed arrays | Array of numbers | Native views |

## Installation

```bash
go get github.com/acolita/v8wire
```

## Quick Start

### Deserialize V8 data

```go
package main

import (
    "fmt"
    "log"

    "github.com/acolita/v8wire/pkg/v8serialize"
)

func main() {
    // Data from Node.js: v8.serialize({name: "Alice", age: 30})
    data := []byte{0xff, 0x0f, 0x6f, 0x22, 0x04, 0x6e, 0x61, 0x6d, 0x65,
                   0x22, 0x05, 0x41, 0x6c, 0x69, 0x63, 0x65, 0x22, 0x03,
                   0x61, 0x67, 0x65, 0x49, 0x3c, 0x7b, 0x02}

    val, err := v8serialize.Deserialize(data)
    if err != nil {
        log.Fatal(err)
    }

    obj := val.AsObject()
    fmt.Println("Name:", obj["name"].AsString()) // Alice
    fmt.Println("Age:", obj["age"].AsInt32())    // 30
}
```

### Serialize Go values

```go
// Serialize native Go types
data, err := v8serialize.SerializeGo(map[string]interface{}{
    "message": "Hello from Go!",
    "numbers": []interface{}{1, 2, 3},
    "nested":  map[string]interface{}{"key": "value"},
})

// The output is compatible with Node.js v8.deserialize()
```

### Convert to native Go types

```go
val, _ := v8serialize.Deserialize(data)

// Convert V8 Value to native Go types
native := v8serialize.ToGo(val)

// Use type assertions
switch v := native.(type) {
case map[string]interface{}:
    fmt.Println("Object:", v)
case []interface{}:
    fmt.Println("Array:", v)
case string:
    fmt.Println("String:", v)
}
```

## Supported Types

| JavaScript Type | Go Type | Notes |
|----------------|---------|-------|
| `null` | `nil` | |
| `undefined` | `nil` | Distinguishable via `Value.IsUndefined()` |
| `boolean` | `bool` | |
| `number` (int32) | `int32` | Values in int32 range |
| `number` (double) | `float64` | All other numbers |
| `bigint` | `*big.Int` | Arbitrary precision |
| `string` | `string` | UTF-8 in Go, Latin1/UTF-16 in V8 |
| `Date` | `time.Time` | Millisecond precision |
| `RegExp` | `*RegExp` | Pattern and flags |
| `Object` | `map[string]Value` | |
| `Array` | `[]Value` | Supports sparse arrays with holes |
| `Map` | `*JSMap` | Preserves insertion order |
| `Set` | `*JSSet` | Preserves insertion order |
| `ArrayBuffer` | `[]byte` | |
| `TypedArray` | `*ArrayBufferView` | Int8Array, Uint8Array, etc. |
| Boxed primitives | `*BoxedPrimitive` | `new Number()`, `new Boolean()`, etc. |

## API Reference

### Deserialization

```go
// Deserialize V8 data to a Value
func Deserialize(data []byte) (Value, error)

// Deserialize or panic (for tests)
func MustDeserialize(data []byte) Value

// Check if data has valid V8 header
func IsValidV8Data(data []byte) bool

// Convert Value to native Go types
func ToGo(v Value) interface{}
```

### Serialization

```go
// Serialize a Value to V8 format
func Serialize(v Value) ([]byte, error)

// Serialize native Go types to V8 format
func SerializeGo(v interface{}) ([]byte, error)
```

### Value Methods

```go
val.Type() Type           // Get the type
val.IsNull() bool         // Check for null
val.IsUndefined() bool    // Check for undefined
val.IsBool() bool         // Check for boolean
val.IsNumber() bool       // Check for any number type
val.IsString() bool       // Check for string

val.AsBool() bool         // Get as bool (panics if wrong type)
val.AsInt32() int32       // Get as int32
val.AsDouble() float64    // Get as float64
val.AsNumber() float64    // Get any number as float64
val.AsString() string     // Get as string
val.AsDate() time.Time    // Get as time.Time
val.AsBigInt() *big.Int   // Get as big.Int
val.AsObject() map[string]Value  // Get as object
val.AsArray() []Value     // Get as array
```

## Compatibility

- **V8 format versions**: 13-15 (Node.js 18-22)
- **Tested with**: Node.js v22.21.0 (V8 12.4.254.21)

## Performance

```
BenchmarkDeserializeInt32     3.9M ops/s    322ns/op
BenchmarkDeserializeString    2.3M ops/s    535ns/op
BenchmarkDeserializeObject    767K ops/s    1.5µs/op
BenchmarkDeserializeArray     1.2M ops/s    1.1µs/op
```

## Testing

The library is tested against 105 fixtures generated by Node.js, covering:

- All primitive types and edge cases
- Unicode strings (Latin1, UTF-16, emoji)
- Nested objects and arrays
- Circular references
- TypedArrays and binary data
- Special objects (RegExp, Date, BigInt)

Run tests:
```bash
go test ./...
```

Run fuzz tests:
```bash
go test -fuzz=FuzzDeserialize ./pkg/v8serialize -fuzztime=30s
```

## License

MIT

## Credits

Based on the V8 serialization format as implemented in:
- [V8 source code](https://github.com/AhmedMostafa16/aspect-based-sentiment-analysis) (src/objects/value-serializer.cc)
- [worker-tools/v8-value-serializer](https://github.com/AhmedMostafa16/aspect-based-sentiment-analysis) (TypeScript reference)

## Makefile Targets

```bash
make test              # Run all tests
make test-compat       # Run cross-version compatibility tests
make generate-fixtures # Generate fixtures with local Node.js
make generate-all-fixtures # Generate fixtures for all Node.js versions (Docker)
make fuzz              # Run fuzz tests (30s)
make bench             # Run benchmarks
make coverage          # Generate coverage report
```
