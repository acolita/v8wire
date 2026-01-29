/**
 * V8 Serialization Test Fixture Generator
 *
 * This is the "oracle" for testing Go V8 deserialization.
 * Generates .bin (raw V8 bytes) and .json (expected values) fixtures.
 */

const v8 = require('v8');
const fs = require('fs');
const path = require('path');

// Parse command line arguments
function parseArgs() {
  const args = process.argv.slice(2);
  let outputDir = null;

  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--output-dir' && args[i + 1]) {
      outputDir = args[i + 1];
      i++;
    }
  }

  return { outputDir };
}

const { outputDir } = parseArgs();

// Use provided output directory or default to testdata/fixtures
const FIXTURES_DIR = outputDir
  ? path.resolve(outputDir)
  : path.join(__dirname, '..', 'testdata', 'fixtures');

// Ensure fixtures directory exists
if (!fs.existsSync(FIXTURES_DIR)) {
  fs.mkdirSync(FIXTURES_DIR, { recursive: true });
}

/**
 * Custom JSON replacer to handle special JS types
 */
function jsonReplacer(key, value) {
  if (typeof value === 'bigint') {
    return { __type: 'bigint', value: value.toString() };
  }
  if (value === undefined) {
    return { __type: 'undefined' };
  }
  if (typeof value === 'number') {
    if (Number.isNaN(value)) {
      return { __type: 'NaN' };
    }
    if (value === Infinity) {
      return { __type: 'Infinity' };
    }
    if (value === -Infinity) {
      return { __type: '-Infinity' };
    }
    if (Object.is(value, -0)) {
      return { __type: '-0' };
    }
  }
  if (value instanceof Date) {
    return { __type: 'Date', value: value.toISOString(), ms: value.getTime() };
  }
  if (value instanceof RegExp) {
    return { __type: 'RegExp', source: value.source, flags: value.flags };
  }
  if (value instanceof Map) {
    return { __type: 'Map', entries: Array.from(value.entries()) };
  }
  if (value instanceof Set) {
    return { __type: 'Set', values: Array.from(value.values()) };
  }
  if (ArrayBuffer.isView(value) && !(value instanceof DataView)) {
    return {
      __type: value.constructor.name,
      data: Array.from(value),
      byteOffset: value.byteOffset,
      byteLength: value.byteLength
    };
  }
  if (value instanceof ArrayBuffer) {
    return { __type: 'ArrayBuffer', data: Array.from(new Uint8Array(value)) };
  }
  // Handle boxed primitives
  if (value instanceof Number) {
    return { __type: 'NumberObject', value: value.valueOf() };
  }
  if (value instanceof Boolean) {
    return { __type: 'BooleanObject', value: value.valueOf() };
  }
  if (value instanceof String) {
    return { __type: 'StringObject', value: value.valueOf() };
  }
  return value;
}

/**
 * Safely stringify a value that may have circular references
 */
function safeStringify(val, replacer) {
  const seen = new WeakSet();
  return JSON.stringify(val, function(key, value) {
    // First apply custom replacer
    let v = replacer ? replacer.call(this, key, value) : value;

    // Handle circular references
    if (typeof v === 'object' && v !== null) {
      if (seen.has(v)) {
        return { __type: 'CircularRef', ref: '[Circular]' };
      }
      seen.add(v);
    }
    return v;
  }, 2);
}

/**
 * Encode a value and save both .bin and .json fixtures
 */
function encode(val, filename, description) {
  const binPath = path.join(FIXTURES_DIR, `${filename}.bin`);
  const jsonPath = path.join(FIXTURES_DIR, `${filename}.json`);

  try {
    const buf = v8.serialize(val);
    fs.writeFileSync(binPath, buf);

    // Create metadata object - use safe stringify to handle circular refs
    const meta = {
      description: description,
      nodeVersion: process.version,
      v8Version: process.versions.v8,
      generatedAt: new Date().toISOString(),
      byteLength: buf.length,
      hexDump: buf.toString('hex'),
      value: val
    };

    fs.writeFileSync(jsonPath, safeStringify(meta, jsonReplacer));

    console.log(`[OK] ${filename}: ${buf.length} bytes`);
    console.log(`     Hex: ${buf.toString('hex')}`);
  } catch (err) {
    console.error(`[FAIL] ${filename}: ${err.message}`);
  }
}

console.log('V8 Wire Format Test Fixture Generator');
console.log(`Node.js ${process.version} / V8 ${process.versions.v8}`);
console.log('=' .repeat(60));
console.log('');

// ============================================================================
// Phase 0: Primitives
// ============================================================================
console.log('--- Primitives ---');

encode(null, 'null', 'null value');
encode(undefined, 'undefined', 'undefined value');
encode(true, 'true', 'boolean true');
encode(false, 'false', 'boolean false');

// ============================================================================
// Phase 1: Numbers
// ============================================================================
console.log('\n--- Numbers ---');

// Int32 range: -2^31 to 2^31-1
encode(0, 'int32-zero', 'int32 zero');
encode(42, 'int32-positive', 'int32 positive (42)');
encode(-42, 'int32-negative', 'int32 negative (-42)');
encode(2147483647, 'int32-max', 'int32 max (2^31-1)');
encode(-2147483648, 'int32-min', 'int32 min (-2^31)');

// Uint32 range: 0 to 2^32-1 (values > 2^31-1 use uint32 tag)
encode(2147483648, 'uint32-min', 'uint32 min (2^31)');
encode(4294967295, 'uint32-max', 'uint32 max (2^32-1)');

// Double: values outside int32/uint32 range, or with decimals
encode(3.14159265358979, 'double-pi', 'double pi');
encode(-0, 'double-negative-zero', 'double negative zero');
encode(Infinity, 'double-infinity', 'double positive infinity');
encode(-Infinity, 'double-neg-infinity', 'double negative infinity');
encode(NaN, 'double-nan', 'double NaN');
encode(8589934592, 'double-large', 'double large integer (2^33)');
encode(1.7976931348623157e+308, 'double-max', 'double max');
encode(5e-324, 'double-min-positive', 'double min positive');

// ============================================================================
// Phase 1: BigInt
// ============================================================================
console.log('\n--- BigInt ---');

encode(0n, 'bigint-zero', 'bigint zero');
encode(42n, 'bigint-positive', 'bigint positive (42n)');
encode(-42n, 'bigint-negative', 'bigint negative (-42n)');
encode(9007199254740993n, 'bigint-beyond-safe', 'bigint beyond safe integer (2^53+1)');
encode(BigInt('123456789012345678901234567890'), 'bigint-huge', 'bigint huge (30 digits)');
encode(-BigInt('123456789012345678901234567890'), 'bigint-huge-negative', 'bigint huge negative');

// ============================================================================
// Phase 1: Strings
// ============================================================================
console.log('\n--- Strings ---');

encode('', 'string-empty', 'empty string');
encode('hello', 'string-onebyte', 'one-byte string (ASCII/Latin1)');
encode('Hello, World!', 'string-hello-world', 'simple ASCII string');
encode('caf\u00E9', 'string-latin1', 'Latin1 string with extended char (cafe with accent)');
encode('\u4F60\u597D\u4E16\u754C', 'string-twobyte-chinese', 'two-byte string (Chinese: Hello World)');
encode('\u4F60\u597D\uD83C\uDF0D', 'string-twobyte-emoji', 'two-byte string with emoji (Chinese + globe)');
encode('\uD83D\uDE00\uD83D\uDE01\uD83D\uDE02', 'string-emoji-only', 'emoji-only string');
encode('a'.repeat(256), 'string-256', '256 character one-byte string');
encode('\u4E2D'.repeat(256), 'string-256-twobyte', '256 character two-byte string');

// Latin-1 boundary tests (0x80-0xFF range)
console.log('\n--- Latin-1 Boundary Tests ---');
encode('\u0080', 'string-latin1-0x80', 'Latin1 char at 0x80 boundary');
encode('\u00FF', 'string-latin1-0xFF', 'Latin1 char at 0xFF (max Latin1)');
encode('\u00E4', 'string-latin1-aumlaut', 'Latin1 ä (U+00E4)');
encode('\u00F6', 'string-latin1-oumlaut', 'Latin1 ö (U+00F6)');
encode('\u00FC', 'string-latin1-uumlaut', 'Latin1 ü (U+00FC)');
encode('\u00DF', 'string-latin1-eszett', 'Latin1 ß (U+00DF)');
encode('\u00A0\u00A9\u00AE\u00B0', 'string-latin1-symbols', 'Latin1 symbols: NBSP © ® °');
encode('\u00C0\u00C1\u00C2\u00C3\u00C4\u00C5', 'string-latin1-accented-A', 'Latin1 accented A chars: ÀÁÂÃÄÅ');
encode('\u00E0\u00E1\u00E2\u00E3\u00E4\u00E5', 'string-latin1-accented-a', 'Latin1 accented a chars: àáâãäå');
encode('M\u00FCnchen Caf\u00E9 na\u00EFve', 'string-latin1-words', 'Latin1 words: München Café naïve');
encode('\u007F', 'string-ascii-del', 'ASCII DEL character (0x7F)');
encode('\u0100', 'string-first-non-latin1', 'First non-Latin1 char (U+0100)');

// Latin-1 range full coverage
let latin1Full = '';
for (let i = 0x80; i <= 0xFF; i++) {
  latin1Full += String.fromCharCode(i);
}
encode(latin1Full, 'string-latin1-full-range', 'All Latin1 extended chars (0x80-0xFF)');

// Mixed ASCII and Latin-1
encode('ABC\u00C4\u00D6\u00DC123', 'string-mixed-ascii-latin1', 'Mixed ASCII and Latin1 extended');
encode('Price: \u00A31.99 (\u00A92024)', 'string-latin1-currency', 'Latin1 with currency symbols: £ ©');

// Multi-byte UTF-8 that fits in Latin-1
encode('\u00C3\u00A4', 'string-latin1-multi-utf8-chars', 'Multiple Latin1 chars that have 2-byte UTF-8');

// ============================================================================
// Phase 1: Date
// ============================================================================
console.log('\n--- Date ---');

encode(new Date(0), 'date-epoch', 'date at Unix epoch');
encode(new Date('2024-01-15T12:30:45.123Z'), 'date-recent', 'recent date');
encode(new Date(-86400000), 'date-before-epoch', 'date before epoch (Dec 31, 1969)');
encode(new Date(8640000000000000), 'date-max', 'max representable date');
encode(new Date(-8640000000000000), 'date-min', 'min representable date');

// ============================================================================
// Phase 2: Objects and Arrays
// ============================================================================
console.log('\n--- Objects and Arrays ---');

encode({}, 'object-empty', 'empty object');
encode({ a: 1, b: 2 }, 'object-simple', 'simple object with two properties');
encode({ name: 'John', age: 30 }, 'object-mixed', 'object with string and number');
encode({ nested: { inner: 'value' } }, 'object-nested', 'nested object');
encode({ num: 42, str: 'hello', bool: true, nil: null }, 'object-types', 'object with various types');

encode([], 'array-empty', 'empty array');
encode([1, 2, 3], 'array-dense', 'dense array of numbers');
encode(['a', 'b', 'c'], 'array-strings', 'dense array of strings');
encode([1, 'two', true, null], 'array-mixed', 'array with mixed types');
encode([[1, 2], [3, 4]], 'array-nested', 'nested arrays');

// Sparse array with holes
const sparse = [1, , , 4, , 6];
sparse[10] = 11;
encode(sparse, 'array-sparse', 'sparse array with holes');

// Array with just a hole
encode([,], 'array-single-hole', 'array with single hole');

// ============================================================================
// Phase 3: Maps and Sets
// ============================================================================
console.log('\n--- Maps and Sets ---');

encode(new Map(), 'map-empty', 'empty Map');
encode(new Map([['key1', 'value1'], ['key2', 'value2']]), 'map-strings', 'Map with string keys');
encode(new Map([[1, 'one'], [2, 'two']]), 'map-numbers', 'Map with number keys');

encode(new Set(), 'set-empty', 'empty Set');
encode(new Set([1, 2, 3]), 'set-numbers', 'Set of numbers');
encode(new Set(['a', 'b', 'c']), 'set-strings', 'Set of strings');

// ============================================================================
// Phase 4: Circular References
// ============================================================================
console.log('\n--- Circular References ---');

// Self-referencing object
const selfRef = { name: 'self' };
selfRef.self = selfRef;
encode(selfRef, 'circular-self', 'self-referencing object');

// Mutual references
const objA = { name: 'A' };
const objB = { name: 'B' };
objA.other = objB;
objB.other = objA;
encode(objA, 'circular-mutual', 'mutually referencing objects');

// Array with circular reference
const circArr = [1, 2, 3];
circArr.push(circArr);
encode(circArr, 'circular-array', 'array containing itself');

// Deep circular
const deep = { level: 1, child: { level: 2, child: { level: 3 } } };
deep.child.child.parent = deep;
encode(deep, 'circular-deep', 'deep object with circular reference');

// ============================================================================
// Phase 5: Binary Data
// ============================================================================
console.log('\n--- Binary Data ---');

encode(new ArrayBuffer(0), 'arraybuffer-empty', 'empty ArrayBuffer');
encode(new ArrayBuffer(8), 'arraybuffer-8', 'ArrayBuffer of 8 bytes');
encode(new Uint8Array([1, 2, 3, 4]).buffer, 'arraybuffer-data', 'ArrayBuffer with data');

encode(new Uint8Array([255, 0, 128]), 'uint8array', 'Uint8Array');
encode(new Int8Array([-128, 0, 127]), 'int8array', 'Int8Array');
encode(new Uint16Array([0, 65535]), 'uint16array', 'Uint16Array');
encode(new Int16Array([-32768, 32767]), 'int16array', 'Int16Array');
encode(new Uint32Array([0, 4294967295]), 'uint32array', 'Uint32Array');
encode(new Int32Array([-2147483648, 2147483647]), 'int32array', 'Int32Array');
encode(new Float32Array([1.5, -2.5]), 'float32array', 'Float32Array');
encode(new Float64Array([Math.PI, Math.E]), 'float64array', 'Float64Array');

// ============================================================================
// Phase 6: Special Objects
// ============================================================================
console.log('\n--- Special Objects ---');

encode(/hello/, 'regexp-simple', 'simple RegExp');
encode(/pattern.*test/gi, 'regexp-flags', 'RegExp with flags');
encode(/^start$/, 'regexp-anchors', 'RegExp with anchors');

encode(new Number(42), 'boxed-number', 'boxed Number');
encode(new Boolean(true), 'boxed-boolean-true', 'boxed Boolean true');
encode(new Boolean(false), 'boxed-boolean-false', 'boxed Boolean false');
encode(new String('wrapped'), 'boxed-string', 'boxed String');

// ============================================================================
// Phase 7: Error Objects
// ============================================================================
console.log('\n--- Error Objects ---');

encode(new Error('simple error'), 'error-simple', 'simple Error');
encode(new TypeError('type error'), 'error-type', 'TypeError');
encode(new RangeError('range error'), 'error-range', 'RangeError');
encode(new SyntaxError('syntax error'), 'error-syntax', 'SyntaxError');
encode(new ReferenceError('ref error'), 'error-reference', 'ReferenceError');

// Error with cause (ES2022)
try {
  const cause = new Error('root cause');
  const err = new Error('wrapper', { cause });
  encode(err, 'error-with-cause', 'Error with cause');
} catch (e) {
  console.log('[SKIP] error-with-cause: Error cause not supported');
}

// ============================================================================
// Phase 8: Additional Edge Cases
// ============================================================================
console.log('\n--- Additional Edge Cases ---');

// Object with numeric keys
encode({ 0: 'zero', 1: 'one', 2: 'two' }, 'object-numeric-keys', 'object with numeric keys');
encode({ 100: 'hundred', 200: 'two hundred' }, 'object-sparse-numeric-keys', 'object with sparse numeric keys');

// Large sparse array (only 3 elements in a 10000-element array)
const largeSparse = [];
largeSparse[0] = 'first';
largeSparse[5000] = 'middle';
largeSparse[9999] = 'last';
encode(largeSparse, 'array-large-sparse', 'large sparse array (10000 elements, 3 values)');

// Array with properties (JavaScript allows this)
const arrayWithProps = [1, 2, 3];
arrayWithProps.customProp = 'custom value';
arrayWithProps.anotherProp = 42;
encode(arrayWithProps, 'array-with-properties', 'array with custom properties');

// Duplicate string references
const sharedString = 'This string appears multiple times';
encode({ a: sharedString, b: sharedString, c: sharedString }, 'string-duplicate-refs', 'object with duplicate string references');

// Many duplicate string references
const manyRefs = {};
const refString = 'repeated';
for (let i = 0; i < 100; i++) {
  manyRefs[`key${i}`] = refString;
}
encode(manyRefs, 'string-many-refs', 'object with 100 duplicate string references');

// Map with circular references
const circMap = new Map();
const circMapObj = { map: circMap };
circMap.set('self', circMap);
circMap.set('obj', circMapObj);
encode(circMap, 'map-circular', 'Map with circular references');

// Set of objects
const setObj1 = { id: 1 };
const setObj2 = { id: 2 };
const setObj3 = { id: 3 };
encode(new Set([setObj1, setObj2, setObj3]), 'set-objects', 'Set containing objects');

// Set with circular reference
const circSet = new Set();
const circSetObj = { set: circSet };
circSet.add(circSetObj);
encode(circSet, 'set-circular', 'Set with circular reference');

// BigInt64Array and BigUint64Array
encode(new BigInt64Array([0n, -1n, 9223372036854775807n, -9223372036854775808n]), 'bigint64array', 'BigInt64Array');
encode(new BigUint64Array([0n, 1n, 18446744073709551615n]), 'biguint64array', 'BigUint64Array');

// DataView
const dvBuffer = new ArrayBuffer(16);
const dataView = new DataView(dvBuffer);
dataView.setInt32(0, 12345, true);
dataView.setFloat64(8, Math.PI, true);
encode(dataView, 'dataview', 'DataView');

// DataView with offset
const dvBuffer2 = new ArrayBuffer(32);
const dataView2 = new DataView(dvBuffer2, 8, 16);
encode(dataView2, 'dataview-offset', 'DataView with offset');

// Deep nested object (depth 100)
let deepObj = { value: 'bottom' };
for (let i = 0; i < 100; i++) {
  deepObj = { nested: deepObj, level: i };
}
encode(deepObj, 'object-deep-100', 'object with 100 levels of nesting');

// Object with boxed BigInt (if supported)
try {
  const boxedBigInt = Object(123n);
  encode(boxedBigInt, 'boxed-bigint', 'boxed BigInt object');
} catch (e) {
  console.log('[SKIP] boxed-bigint: BigInt boxing not supported');
}

// TypedArray with view into shared buffer
const sharedBuf = new ArrayBuffer(16);
const view1 = new Uint8Array(sharedBuf, 0, 8);
const view2 = new Uint8Array(sharedBuf, 8, 8);
view1.set([1, 2, 3, 4, 5, 6, 7, 8]);
view2.set([9, 10, 11, 12, 13, 14, 15, 16]);
encode({ view1, view2 }, 'typed-array-views', 'two TypedArray views into same buffer');

// Very long string (for stress testing)
encode('x'.repeat(10000), 'string-10k', '10000 character string');

// Unicode edge cases
encode('\uD800', 'string-unpaired-high-surrogate', 'unpaired high surrogate');
encode('\uDC00', 'string-unpaired-low-surrogate', 'unpaired low surrogate');
encode('\uFFFD', 'string-replacement-char', 'Unicode replacement character');

// ============================================================================
// Additional Edge Cases (from Deno v8_valueserializer tests)
// ============================================================================
console.log('\n--- Additional Edge Cases ---');

// String with null byte
encode('foo\x00bar', 'string-with-null', 'string containing null byte');

// Unpaired surrogate with context
encode('foo\uD800bar', 'string-unpaired-surrogate-context', 'unpaired high surrogate with surrounding chars');

// Very large BigInt (128-bit)
encode(BigInt('340282366920938463463374607431768211455'), 'bigint-u128-max', 'BigInt u128 max value');

// Empty RegExp
encode(new RegExp('(?:)'), 'regexp-empty', 'empty RegExp pattern');

// RegExp with emoji
encode(/🗄️/, 'regexp-emoji', 'RegExp with emoji');

// Invalid Date
encode(new Date(NaN), 'date-invalid', 'invalid Date (NaN)');

// Object with various SMI keys
encode({ 0: 'zero', '-1': 'neg-one', 2147483647: 'max' }, 'object-smi-keys', 'object with SMI property keys');

// Sparse array with explicit length
const sparseExplicit = new Array(5);
sparseExplicit[2] = 'middle';
encode(sparseExplicit, 'array-sparse-explicit', 'sparse array created with explicit length');

// Sparse array with properties (ordered)
const sparseWithPropsOrdered = new Array(3);
sparseWithPropsOrdered[0] = 'first';
sparseWithPropsOrdered.customProp = 'custom';
encode(sparseWithPropsOrdered, 'array-sparse-with-props', 'sparse array with custom properties');

// Dense array circular (self-referencing)
const denseCircular = [undefined];
denseCircular[0] = denseCircular;
encode(denseCircular, 'array-dense-circular-self', 'dense array referencing itself');

// TypedArray subarrays (views into portions)
encode(new Uint8Array([1, 2, 3, 4]).subarray(1, 3), 'uint8array-subarray', 'Uint8Array subarray view');
encode(new Int32Array([10, 20, 30]).subarray(1, 2), 'int32array-subarray', 'Int32Array subarray view');
encode(new Float64Array([1.5, 2.5, 3.5]).subarray(0, 2), 'float64array-subarray', 'Float64Array subarray view');

// TypedArray with special float values
encode(new Float32Array([0, -1.5, 2, -3.5, NaN, -5.5, -Infinity, -0, Infinity]), 'float32array-special-values', 'Float32Array with NaN, Infinity, -0');
encode(new Float64Array([0, -1.5, 2, -3.5, NaN, -5.5, -Infinity, -0, Infinity]), 'float64array-special-values', 'Float64Array with NaN, Infinity, -0');

// DataView with offset
const dvBuf = new Uint8Array([1, 2, 3, 4, 5]).buffer;
encode(new DataView(dvBuf, 1, 3), 'dataview-with-offset', 'DataView with byte offset and length');

// Error without stack
const errNoStack = new Error('no stack');
delete errNoStack.stack;
encode(errNoStack, 'error-no-stack', 'Error with stack deleted');

// Error with primitive cause
try {
  encode(new Error('with cause', { cause: 42 }), 'error-cause-primitive', 'Error with primitive cause');
} catch (e) {
  console.log('[SKIP] error-cause-primitive: Error cause not supported');
}

// Error with object cause
try {
  encode(new Error('with cause', { cause: { reason: 'failed' } }), 'error-cause-object', 'Error with object cause');
} catch (e) {
  console.log('[SKIP] error-cause-object: Error cause not supported');
}

// Multiple objects sharing string references
const sharedKey = 'shared_key_value';
encode([{ key: sharedKey }, { key: sharedKey }, { key: sharedKey }], 'array-shared-strings', 'array of objects sharing string references');

// Circular Map (map containing itself)
const circularMap = new Map();
circularMap.set('self', circularMap);
encode(circularMap, 'map-circular-self', 'Map containing itself');

// Circular Set (set containing object that references set)
const circularSet = new Set();
const setObj = { set: circularSet };
circularSet.add(setObj);
encode(circularSet, 'set-circular-obj', 'Set with circular reference through object');

// Boxed Number with special values
encode(new Number(NaN), 'boxed-number-nan', 'boxed Number(NaN)');
encode(new Number(Infinity), 'boxed-number-infinity', 'boxed Number(Infinity)');
encode(new Number(-0), 'boxed-number-neg-zero', 'boxed Number(-0)');

// Map with non-string keys
encode(new Map([[1, 'one'], [true, 'bool'], [null, 'null']]), 'map-non-string-keys', 'Map with number, boolean, null keys');

// Set with mixed types
encode(new Set([1, 'two', true, null, undefined]), 'set-mixed-types', 'Set with mixed primitive types');

// Multiple TypedArray views of same buffer (different types)
const multiViewBuf = new ArrayBuffer(16);
new Float64Array(multiViewBuf).set([Math.PI, Math.E]);
encode({
  asFloat64: new Float64Array(multiViewBuf),
  asUint8: new Uint8Array(multiViewBuf),
  asInt32: new Int32Array(multiViewBuf),
}, 'typed-array-multi-views', 'multiple TypedArray types viewing same buffer');

// ResizableArrayBuffer (if supported)
try {
  const resizable = new ArrayBuffer(8, { maxByteLength: 16 });
  encode(resizable, 'arraybuffer-resizable', 'ResizableArrayBuffer');
  encode(new Uint8Array(resizable), 'uint8array-resizable', 'Uint8Array on ResizableArrayBuffer');
} catch (e) {
  console.log('[SKIP] ResizableArrayBuffer: not supported in this Node version');
}

// ============================================================================
// Summary
// ============================================================================
console.log('\n' + '=' .repeat(60));
console.log('Fixture generation complete!');
console.log(`Output directory: ${FIXTURES_DIR}`);
