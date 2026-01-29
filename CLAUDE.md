# Agentic Implementation Guide: Go V8 Deserializer

## Mission
Implement a production-grade V8 serialization format (Structured Clone) deserializer in Go that achieves **100% wire-format compatibility** with Node.js `v8.serialize()`/`v8.deserialize()` across versions 12-15.

## Core Strategy: Node.js as the Oracle
**You MUST use Node.js as your compatibility oracle.** Do not guess the format. Generate test vectors from Node, compare byte-by-byte.

### Node.js Integration Protocol
Always maintain a `testgen/` Node.js project:

\`\`\`javascript
// testgen/generate.js - Your compatibility oracle
const v8 = require('v8');
const fs = require('fs');

function encode(val, filename) {
  const buf = v8.serialize(val);
  fs.writeFileSync(`fixtures/${filename}.bin`, buf);
  fs.writeFileSync(`fixtures/${filename}.json`, JSON.stringify(val, (k,v) =&gt; {
    if (typeof v === 'bigint') return `BigInt:${v.toString()}`;
    if (v === undefined) return '___undefined___';
    if (Number.isNaN(v)) return '___NaN___';
    return v;
  }));
}

// Generate fixtures for every type, edge case, and version feature
encode(null, 'null');
encode(undefined, 'undefined');
encode(true, 'true');
encode(false, 'false');
encode(42, 'int32');
encode(2**31, 'uint32');  
encode(2**33, 'double');
encode(9007199254740993n, 'bigint');
encode("hello", 'string-onebyte');
encode("你好世界 🌍", 'string-utf16');
encode([1,2,3], 'array-dense');
encode([1,,3], 'array-sparse'); // hole
encode({a:1, b:2}, 'object-simple');
encode(new Date('2024-01-01'), 'date');
encode(/regex.*test/gi, 'regexp');
encode(new Map([['key', 'value']]), 'map');
encode(new Set([1,2,3]), 'set');
encode(Buffer.from('binary'), 'buffer'); // Node Buffer -&gt; ArrayBuffer
encode(new Uint8Array([1,2,3]), 'uint8array');
encode({ circular: {} }, 'circular-ref'); // Set circular.circular = circular
encode(new Number(42), 'boxed-number'); // Primitive wrapper
\`\`\`

Run with: \`mkdir -p fixtures && node testgen/generate.js\`

## Implementation Phases

### Phase 1: Foundation (Bootstrapping)
1. Create project structure:
   \`\`\`
   gov8serialize/
   ├── cmd/
   │   └── testgen/          # Node.js test vector generator
   ├── internal/
   │   └── wire/             # Binary primitives (varint, doubles)
   ├── pkg/
   │   └── v8serialize/
   │       ├── reader.go     # Bytecode reader with alignment
   │       ├── tags.go       # V8 tag constants
   │       ├── types.go      # Go representations of JS types
   │       └── deserializer.go
   ├── testdata/fixtures/    # Generated .bin and .json files
   └── compatibility_test.go # Node.js compatibility matrix
   \`\`\`

2. **Wire Format Primitives** (\`internal/wire/\`):
   - Varint (base-128) encode/decode
   - ZigZag for signed ints
   - IEEE 754 double (little-endian)
   - UTF-8 / UTF-16 string reading with alignment
   - **Verification**: Write round-trip tests, then verify against Node fixtures

3. **Tag Constants** (\`tags.go\`):
   Extract from V8 source \`src/serialization/ValueSerializer.cpp\`:
   \`\`\`go
   const (
       tagVersion byte = 0xFF
       tagNull    byte = '0'
       tagUndefined byte = '_'
       tagInt32   byte = 'I'
       // ... etc, map exactly to V8 values
   )
   \`\`\`

### Phase 2: Deserialization Engine
Implement in strict order (dependencies matter):

1. **Version Detection**: Parse 0xFF envelope, determine version (12-15)
2. **Primitive Tags**: null, undefined, true, false, int32, uint32, double, date, bigint
3. **String Tags**: one-byte (Latin1) and two-byte (UTF-16) with alignment handling
4. **Reference System**: Object ID assignment and resolution (crucial for circular refs)
5. **Object Tags**: begin/end object, sparse/dense arrays
6. **Collection Tags**: Map, Set, ArrayBuffer, TypedArrays
7. **Special Tags**: RegExp, Error objects, boxed primitives
8. **Host Objects**: Handle HostObject tag (skip or error gracefully)

### Phase 3: Go Type System Design
You must design Go representations that preserve JS semantics:

\`\`\`go
package v8serialize

type Type uint8
const (
    TypeUndefined Type = iota
    TypeNull
    TypeBool
    TypeNumber // Can be int32, uint32, or float64
    TypeBigInt
    TypeString
    TypeObject // map[string]Value
    TypeArray  // []Value with Hole support
    TypeDate   // time.Time
    TypeRegExp // struct { Pattern, Flags string }
    TypeMap    // []MapEntry (preserve insertion order)
    TypeSet    // []Value
    TypeArrayBuffer // []byte
    TypeTypedArray  // View into ArrayBuffer
    TypeHole        // Sparse array holes
    TypeReference   // Circular reference marker (resolved during unmarshal)
)

type Value struct {
    Type Type
    Data interface{}
}

// Helper accessors to avoid type assertions everywhere
func (v Value) IsUndefined() bool { return v.Type == TypeUndefined }
func (v Value) Int() (int64, error) { ... }
func (v Value) String() (string, error) { ... }
func (v Value) Object() (map[string]Value, error) { ... }
\`\`\`

**Key Decision**: For arrays with holes, use \`[]*Value\` where \`nil\` represents a hole, or add explicit \`TypeHole\`.

### Phase 4: Compatibility Testing
Create \`compatibility_test.go\` that:

1. Reads \`testdata/fixtures/*.bin\` and \`*.json\`
2. Deserializes the .bin file
3. Compares structure with .json expected value
4. **Round-trip test**: Serialize in Node → Deserialize in Go → Serialize in Node again → Compare bytes
5. **Version matrix**: Test against fixtures generated by Node 18 (v13), 20 (v14), 22 (v15)

Use property-based testing (fuzzing) with Go's \`testing/fstest\` or \`gopter\`:
\`\`\`go
func FuzzCompatibility(f *testing.F) {
    f.Add([]byte("...")) // seed with known fixtures
    f.Fuzz(func(t *testing.T, data []byte) {
        // Try to deserialize, should not panic
        // If valid, verify round-trip via Node
    })
}
\`\`\`

### Phase 5: Performance & Edge Cases
- **Memory pooling**: For large buffers, use \`sync.Pool\` on small objects
- **Streaming**: Support incremental parsing for large objects
- **Limits**: Configurable max depth (prevent stack overflow on malicious input)
- **Alignment bugs**: Two-byte strings require even alignment; verify padding bytes are skipped correctly
- **Sparse arrays**: Ensure holes are preserved exactly (not converted to undefined)

## Critical Implementation Rules

1. **Never assume string encoding**: V8 uses Latin1 for strings that fit, UTF-16 for others. Check the tag.
2. **Alignment matters**: After reading odd-length data, the next two-byte string might be padded.
3. **Reference IDs are 0-indexed**: First object has id 0, assigned sequentially during write.
4. **BigInt**: Can be negative and arbitrary precision. Use \`math/big.Int\`.
5. **RegExp flags**: Stored as string (e.g., "gi"), not bitwise.
6. **Date**: Milliseconds since epoch as double (not int64).
7. **Version quirks**: 
   - v12: No BigInt support
   - v13: Adds BigInt
   - v14: Adds shared ArrayBuffer support
   - v15: Adds WebAssembly.Module support (you can skip)

## Reference Materials (Priority Order)
1. **Primary**: \`worker-tools/v8-value-serializer\` (TypeScript) - Cleanest reference implementation
2. **Secondary**: V8 source \`src/serialization/ValueSerializer.cpp\` and \`ValueDeserializer.cpp\`
3. **Tertiary**: Node.js documentation and \`test/parallel/test-v8-serialize.js\` in Node source

## Success Criteria
- [ ] Passes all fixtures in \`testdata/fixtures/\` (100+ test cases)
- [ ] Round-trip compatible with Node.js for all supported types
- [ ] Handles circular references without infinite loops
- [ ] Correctly represents sparse arrays with holes
- [ ] Supports versions 13, 14, 15 (detect and adapt)
- [ ] Fuzz-tested without panics for 1M+ iterations
- [ ] Benchmark: Deserialize 10MB payload in &lt; 100ms

## Agent Workflow Checkpoints
Before proceeding to next phase, verify:
1. **Phase 1**: Can read all primitive fixtures generated by Node
2. **Phase 2**: Can deserialize \`circular-ref.bin\` without stack overflow
3. **Phase 3**: All TypeScript test vectors from reference implementation pass
4. **Phase 4**: 100% compatibility on Node.js test suite subset

Always commit progress after each checkpoint.
