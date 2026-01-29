# Go V8 Deserializer: Implementation Roadmap

## Legend
- [ ] Not started
- [-] In progress
- [x] Complete
- [!] Blocked/Issue

## Phase 0: Bootstrap & Tooling
**Goal**: Infrastructure to generate test vectors and verify compatibility

- [x] Initialize Go module (`go mod init github.com/acolita/v8wire`)
- [x] Create `testgen/` Node.js project with `package.json`
- [x] Implement `testgen/generate.js` to output fixtures
- [x] Create `testdata/fixtures/README.md` documenting Node version used
- [x] Set up Go test harness to load fixtures and compare
- [x] **Checkpoint**: Successfully generate and read 5 primitive fixtures (135 fixtures generated!)
- [x] Add edge case fixtures from Deno v8_valueserializer test suite

## Phase 1: Wire Format Primitives
**Goal**: Read raw bytes correctly

- [x] Varint (base-128) reader/writer
  - [x] Decode single-byte varints
  - [x] Decode multi-byte varints
  - [x] ZigZag decoding for signed ints
- [x] IEEE 754 double reader (little-endian)
- [x] String readers
  - [x] One-byte (Latin1) string
  - [x] Two-byte (UTF-16) string with alignment
- [x] Bytecode alignment handling (padding skip)
- [x] **Test**: Round-trip varint/double with Node-generated fixtures
- [x] **Checkpoint**: All primitive wire types match Node byte-for-byte

## Phase 2: Version & Header
**Goal**: Parse serialization envelope

- [x] Detect version tag (0xFF)
- [x] Parse version number (varint)
- [x] Validate version support (13-15)
- [ ] Handle legacy format (no version tag = version 0)
- [x] **Test**: Fixtures from Node 22 parse version correctly
- [x] **Checkpoint**: Can identify version for 100% of generated fixtures

## Phase 3: Primitive Types
**Goal**: Deserialize basic values

- [x] tagNull ('0')
- [x] tagUndefined ('_')
- [x] tagTrue ('T')
- [x] tagFalse ('F')
- [x] tagInt32 ('I')
- [x] tagUint32 ('U')
- [x] tagDouble ('N')
- [x] tagDate ('D')
- [x] tagBigInt ('Z')
- [x] **Test Matrix**:
  - [x] null, undefined
  - [x] true, false
  - [x] 0, -1, 2^31-1 (int32)
  - [x] 2^31, 2^32-1 (uint32 - encoded as double by V8)
  - [x] 2^33, 3.14159, -0, NaN, Infinity (double)
  - [x] 0n, -1n, beyond safe int, huge bigint (bigint)
  - [x] Date objects (epoch, recent, negative)
- [x] **Checkpoint**: All primitive fixtures deserialize to correct Go values

## Phase 4: Strings & References
**Goal**: Handle text and object identity

- [x] tagOneByteString ('"')
- [x] tagTwoByteString ('c')
- [x] tagStringObject ('s') - wrapper
- [x] Reference table implementation
  - [x] Assign IDs on first read (0-indexed)
  - [x] Store in slice for lookup
  - [x] tagObjectReference ('^')
- [x] **Test Matrix**:
  - [x] ASCII string
  - [x] UTF-8 multi-byte string (Latin1)
  - [x] Emojis (UTF-16 surrogate pairs)
  - [x] Long strings (256 chars)
  - [x] Empty string
  - [x] String object (boxed primitive)
  - [x] Duplicate string references
- [x] **Checkpoint**: Can deserialize fixture with 100 duplicate string refs correctly

## Phase 5: Objects & Arrays
**Goal**: Handle collections and properties

- [x] tagBeginJSObject ('o')
- [x] tagEndJSObject ('{')
- [x] tagBeginDenseJSArray ('A')
- [x] tagEndDenseJSArray ('$')
- [x] tagBeginSparseJSArray ('a')
- [x] tagEndSparseJSArray ('@')
- [x] tagHole ('-')
- [x] Property parsing (key-value pairs)
- [x] Array length handling
- [x] **Test Matrix**:
  - [x] Empty object `{}`
  - [x] Simple object `{a: 1, b: "two"}`
  - [x] Nested objects
  - [x] Object with numeric keys
  - [x] Empty array `[]`
  - [x] Dense array `[1,2,3,4,5]`
  - [x] Sparse array `[1,,3,,5]` (holes)
  - [x] Large sparse array (only 3 elements of 10000)
  - [x] Array with properties (js allows this)
- [x] **Checkpoint**: Can handle object-within-object depth of 100

## Phase 6: Circular References
**Goal**: Handle the hard part of graphs

- [x] Self-referencing object `a.a = a`
- [x] Mutual references `a.b = b; b.a = a`
- [x] Arrays with circular refs `[1, a, 3]` where `a` contains array
- [x] Deep circular paths
- [x] **Test**: All circular fixtures resolve without panic
- [x] **Checkpoint**:
  - [x] `GoString()` doesn't infinite loop on circular refs (tested with timeout)
  - [x] Round-trip through Node preserves structure (via TestGoToNodeRoundTrip)

## Phase 7: Collections & Binary Data
**Goal**: ES6+ and binary types

- [x] tagMap (';')
- [x] tagSet ('\'')
- [x] tagArrayBuffer ('B')
- [ ] tagResizableArrayBuffer (~)
- [ ] tagSharedArrayBuffer (not supported, handle gracefully)
- [x] tagTypedArray ('\\' with type ID)
- [x] tagDataView ('?') - handled via TypedArray with type ID 9
- [x] **Test Matrix**:
  - [x] Map with string keys
  - [x] Map with number keys
  - [ ] Map with circular refs (V8 limitation - causes stack overflow)
  - [x] Set of primitives
  - [x] Set of objects
  - [x] ArrayBuffer (various sizes)
  - [x] Int8Array, Uint8Array
  - [x] Int16Array, Uint16Array
  - [x] Int32Array, Uint32Array, Float32Array, Float64Array
  - [x] BigInt64Array, BigUint64Array
  - [x] DataView
- [x] **Checkpoint**: TypedArray views point to correct offsets in ArrayBuffer

## Phase 8: Special Objects
**Goal**: Complete type coverage

- [x] tagRegExp ('R')
- [x] tagNumberObject ('n') - boxed Number with double
- [x] tagBigIntObject ('z') - boxed BigInt (deserialization with validation)
- [x] tagTrueObject ('y') - boxed Boolean true
- [x] tagFalseObject ('x') - boxed Boolean false
- [x] tagStringObject ('s') - boxed String
- [ ] tagSymbol - may error (symbols often non-serializable)
- [x] tagObjectReference ('^') - already done in P4
- [ ] tagGenerateFreshMap (internal V8, handle gracefully)
- [x] **Test Matrix**:
  - [x] RegExp: /simple/, /complex.*pattern/gi
  - [x] new Number(42), new Boolean(true), new String("wrapped")
  - [ ] new BigInt(123n) (Node.js doesn't support boxing BigInt)
  - [ ] Symbol.for('test') - expect error or handle
- [x] **Checkpoint**: All special object fixtures pass

## Phase 8.5: Error Objects
**Goal**: Support JavaScript Error objects

- [x] Generic Error deserialization
- [x] TypeError, RangeError, SyntaxError, ReferenceError
- [x] Error message extraction
- [x] Error stack trace extraction
- [x] Error.cause (ES2022) - deserialization support
- [x] Error serialization

## Phase 9: Error Handling & Edge Cases
**Goal**: Robustness

- [x] Malformed input handling (don't panic) - fuzz tested
- [x] Unknown tag handling (returns error with tag info)
- [x] Truncated data detection (ErrUnexpectedEOF)
- [x] Max depth limit (configurable via WithMaxDepth)
- [x] Max size limit (configurable via WithMaxSize)
- [x] Invalid UTF-16 handling (unpaired surrogates) - passes through
- [x] **Test Matrix**:
  - [x] Corrupted varints (fuzz tested)
  - [x] Unexpected EOF (fuzz tested)
  - [x] Invalid reference IDs (ErrInvalidReference)
  - [x] Unknown version number (ErrUnsupportedVersion)
  - [x] Circular reference depth 1000 (stack overflow prevention via WithMaxDepth)
- [x] **Checkpoint**: Fuzz testing 2 minutes without panic (3.5M+ executions)

## Phase 10: Performance & Polish
**Goal**: Production ready

- [x] Benchmark suite (deserialize up to 100K elements, ~4MB payload)
  - Array-100000elements: 42 MB/s throughput
  - Object-10000keys: 21 MB/s throughput
  - DeepNested-500: 17 MB/s throughput
- [ ] Memory allocation optimization (sync.Pool for small objects)
- [ ] Streaming API for large objects (if feasible)
- [x] API Documentation (Go doc style) - basic docs in README
- [x] README with usage examples
- [x] CI/CD with multiple Node versions (GitHub Actions)
- [ ] **Test Matrix**:
  - [ ] Benchmark against Node.js native performance (within 5x)
  - [ ] Memory profile shows no leaks on circular structures
- [x] **Checkpoint**: Ready for v0.1.0 release
  - [x] go vet passes
  - [x] 135 fixtures, 567+ tests pass
  - [x] 67.7% coverage on v8serialize, 78.5% on wire
  - [x] Package documentation complete
  - [x] Serializer circular ref limitation documented

## Phase 11: Multi-Version Compatibility Testing
**Goal**: Guarantee backwards compatibility across V8 wire format versions using Docker

### Infrastructure
- [x] Create `testgen/Dockerfile.node18` for Node.js 18.x (format v13)
- [x] Create `testgen/Dockerfile.node20` for Node.js 20.x (format v14)
- [x] Create `testgen/Dockerfile.node22` for Node.js 22.x (format v15)
- [x] Create `testgen/docker-compose.yml` to orchestrate all versions
- [x] Create `testgen/generate-all.sh` script to run all containers
- [x] Add `--output-dir` argument to `generate.js`
- [x] Create `pkg/v8serialize/compat_test.go` for cross-version tests
- [x] Create `Makefile` with `test-compat` and `generate-all-fixtures` targets

### Fixture Generation by Version
- [x] Generate `testdata/fixtures/v13/` from Node.js 18.x container (105 fixtures)
- [x] Generate `testdata/fixtures/v14/` from Node.js 20.x container (105 fixtures)
- [x] Generate `testdata/fixtures/v15/` from Node.js 22.x container (105 fixtures)
- [x] Add version metadata to each fixture set (node version, v8 version, date)

### Cross-Version Tests
- [x] Test: v13 fixtures deserialize correctly (104/105, 1 skipped - boxed BigInt unsupported)
- [x] Test: v14 fixtures deserialize correctly (104/105, 1 skipped)
- [x] Test: v15 fixtures deserialize correctly (104/105, 1 skipped)
- [x] Test: Go-serialized data deserializes in local Node.js (25 test cases)
- [x] Test: Go-serialized data deserializes in Node 18 container (via V8WIRE_TEST_DOCKER=1)
- [x] Test: Go-serialized data deserializes in Node 20 container (via V8WIRE_TEST_DOCKER=1)
- [x] Test: Go-serialized data deserializes in Node 22 container (via V8WIRE_TEST_DOCKER=1)

### Regression Testing
- [x] Add `make test-compat` target to run all version tests
- [ ] CI job to regenerate fixtures and detect format changes
- [x] Document any version-specific quirks or incompatibilities (see below)

### Version-Specific Quirks (Documented)

1. **V8 always uses latest format version**: V8's ValueSerializer always uses `kLatestVersion`
   (currently 15). Even older Node.js versions (18.x) may serialize with format v15 if running
   a newer V8. There's no API to force older formats.

2. **Boxed BigInt not serializable**: `Object(123n)` cannot be serialized by Node.js - it throws
   "DataCloneError: BigInt value can't be serialized as [object BigInt]". All v13/v14/v15
   fixtures skip this test case.

3. **Float16Array (v12+)**: V8 12.x (Node 22+) added Float16Array at TypedArray type ID 10,
   shifting DataView to ID 9 and BigInt64Array/BigUint64Array to IDs 11/12. Older code
   expecting DataView at ID 10 will misparse.

4. **Error.cause (ES2022)**: Only available in Node 22+ (format v15). Earlier versions
   will ignore the cause property when serializing errors.

5. **ResizableArrayBuffer (v14+)**: Tag `~` for growable ArrayBuffers. Not implemented
   in this library yet.

6. **SharedArrayBuffer**: Requires special handling and shared memory support. We return
   an error rather than attempting to deserialize.

### Version-Specific Features
- [x] Test BigInt (v13+)
- [ ] Test ResizableArrayBuffer (v14+)
- [x] Test Error.cause (v15+, Node 22+)
- [x] Gracefully handle unsupported features per version (skip boxed BigInt)

### **Checkpoint**: All fixtures from all Node versions pass deserialization ✓

## Compatibility Checklist
Verify against these Node.js versions:
- [x] Node.js 18.x (V8 v10.x, format v13) - 104/105 fixtures pass
- [x] Node.js 20.x (V8 v11.x, format v14) - 104/105 fixtures pass
- [x] Node.js 22.x (V8 v12.x, format v15) - 104/105 fixtures pass

## Known Difficult/Optional (Post-MVP)
- [ ] HostObject deserialization (requires V8 embedder knowledge)
- [ ] SharedArrayBuffer (requires shared memory coordination)
- [ ] WebAssembly.Module (v15 feature, complex)
- [ ] WASM.Exception (v15 feature)
- [ ] Context loss recovery (internal V8 details)

## Final Verification
Before declaring complete:
- [ ] Run full Node.js v8 module test suite against your implementation
- [ ] Property-based testing (generate random JS objects, verify round-trip)
- [ ] Security audit (no panic on malicious input)
- [ ] Documentation review
