#!/usr/bin/env node
/**
 * Verify that Go-serialized V8 data can be deserialized by Node.js
 *
 * Usage modes:
 *   node verify.js <hex-string>           # Verify single hex string
 *   node verify.js --dir <path>           # Verify all .bin files in directory
 *
 * Example: node verify.js ff0f4954
 * Example: node verify.js --dir /path/to/go-serialized
 */

const v8 = require('v8');
const fs = require('fs');
const path = require('path');

function hexToBuffer(hex) {
  const bytes = [];
  for (let i = 0; i < hex.length; i += 2) {
    bytes.push(parseInt(hex.substr(i, 2), 16));
  }
  return Buffer.from(bytes);
}

function prettyPrint(val, indent = 0) {
  const pad = '  '.repeat(indent);

  if (val === null) return 'null';
  if (val === undefined) return 'undefined';
  if (typeof val === 'boolean') return val.toString();
  if (typeof val === 'number') return val.toString();
  if (typeof val === 'bigint') return val.toString() + 'n';
  if (typeof val === 'string') return JSON.stringify(val);
  if (val instanceof Date) return `Date(${val.toISOString()})`;
  if (val instanceof RegExp) return val.toString();
  if (val instanceof Map) {
    const entries = Array.from(val.entries())
      .map(([k, v]) => `${pad}  ${prettyPrint(k)} => ${prettyPrint(v, indent + 1)}`)
      .join(',\n');
    return `Map {\n${entries}\n${pad}}`;
  }
  if (val instanceof Set) {
    const values = Array.from(val.values())
      .map(v => `${pad}  ${prettyPrint(v, indent + 1)}`)
      .join(',\n');
    return `Set {\n${values}\n${pad}}`;
  }
  if (ArrayBuffer.isView(val)) {
    return `${val.constructor.name}(${Array.from(val).join(', ')})`;
  }
  if (val instanceof ArrayBuffer) {
    return `ArrayBuffer(${val.byteLength})`;
  }
  if (Array.isArray(val)) {
    if (val.length === 0) return '[]';
    const elements = val.map((v, i) => {
      // Check for holes
      if (!(i in val)) return `${pad}  <hole>`;
      return `${pad}  ${prettyPrint(v, indent + 1)}`;
    }).join(',\n');
    return `[\n${elements}\n${pad}]`;
  }
  if (typeof val === 'object') {
    const keys = Object.keys(val);
    if (keys.length === 0) return '{}';
    const props = keys.map(k => `${pad}  ${JSON.stringify(k)}: ${prettyPrint(val[k], indent + 1)}`).join(',\n');
    return `{\n${props}\n${pad}}`;
  }
  return String(val);
}

/**
 * Verify all .bin files in a directory
 */
function verifyDirectory(dirPath) {
  console.log('V8 Batch Deserialization Verifier');
  console.log(`Node.js ${process.version} / V8 ${process.versions.v8}`);
  console.log('='.repeat(60));
  console.log(`Directory: ${dirPath}`);
  console.log('');

  const binFiles = fs.readdirSync(dirPath)
    .filter(f => f.endsWith('.bin'))
    .sort();

  if (binFiles.length === 0) {
    console.log('No .bin files found.');
    return { passed: 0, failed: 0 };
  }

  console.log(`Found ${binFiles.length} fixtures to verify.\n`);

  let passed = 0;
  let failed = 0;
  const failures = [];

  for (const binFile of binFiles) {
    const name = binFile.replace('.bin', '');
    const binPath = path.join(dirPath, binFile);
    const expectedPath = path.join(dirPath, `${name}.expected.json`);

    try {
      const binData = fs.readFileSync(binPath);
      const val = v8.deserialize(binData);

      // If there's an expected.json file, compare values
      if (fs.existsSync(expectedPath)) {
        const expected = JSON.parse(fs.readFileSync(expectedPath, 'utf8'));
        const actual = normalizeForComparison(val);

        if (!deepEquals(actual, expected)) {
          throw new Error(`Value mismatch`);
        }
      }

      console.log(`[PASS] ${name}: ${describeValue(val)}`);
      passed++;
    } catch (err) {
      console.log(`[FAIL] ${name}: ${err.message}`);
      failures.push({ name, error: err.message });
      failed++;
    }
  }

  console.log('');
  console.log('='.repeat(60));
  console.log(`Results: ${passed} passed, ${failed} failed`);

  if (failures.length > 0) {
    console.log('\nFailures:');
    for (const f of failures) {
      console.log(`  - ${f.name}: ${f.error}`);
    }
  }

  return { passed, failed };
}

/**
 * Normalize a value for comparison
 */
function normalizeForComparison(val, seen = new WeakSet()) {
  if (val === null) return null;
  if (val === undefined) return { __type: 'undefined' };
  if (typeof val === 'bigint') return { __type: 'bigint', value: val.toString() };
  if (typeof val === 'number') {
    if (Number.isNaN(val)) return { __type: 'NaN' };
    if (val === Infinity) return { __type: 'Infinity' };
    if (val === -Infinity) return { __type: '-Infinity' };
    if (Object.is(val, -0)) return { __type: '-0' };
    return val;
  }
  if (typeof val === 'string') return val;
  if (val instanceof Date) return { __type: 'Date', ms: val.getTime() };
  if (val instanceof RegExp) return { __type: 'RegExp', source: val.source, flags: val.flags };
  if (val instanceof Map) {
    if (seen.has(val)) return { __type: 'CircularRef' };
    seen.add(val);
    const entries = [];
    for (const [k, v] of val) {
      entries.push([normalizeForComparison(k, seen), normalizeForComparison(v, seen)]);
    }
    return { __type: 'Map', entries };
  }
  if (val instanceof Set) {
    if (seen.has(val)) return { __type: 'CircularRef' };
    seen.add(val);
    const values = [];
    for (const v of val) {
      values.push(normalizeForComparison(v, seen));
    }
    return { __type: 'Set', values };
  }
  if (ArrayBuffer.isView(val) && !(val instanceof DataView)) {
    return {
      __type: val.constructor.name,
      data: Array.from(val.constructor.name.includes('BigInt') ? [...val].map(n => n.toString()) : val),
      byteOffset: val.byteOffset,
      byteLength: val.byteLength
    };
  }
  if (val instanceof ArrayBuffer) {
    return { __type: 'ArrayBuffer', data: Array.from(new Uint8Array(val)) };
  }
  if (val instanceof DataView) {
    return {
      __type: 'DataView',
      byteOffset: val.byteOffset,
      byteLength: val.byteLength
    };
  }
  if (val instanceof Error) {
    return {
      __type: val.constructor.name,
      message: val.message,
      cause: val.cause !== undefined ? normalizeForComparison(val.cause, seen) : undefined
    };
  }
  if (val instanceof Number) return { __type: 'NumberObject', value: val.valueOf() };
  if (val instanceof Boolean) return { __type: 'BooleanObject', value: val.valueOf() };
  if (val instanceof String) return { __type: 'StringObject', value: val.valueOf() };
  if (Array.isArray(val)) {
    if (seen.has(val)) return { __type: 'CircularRef' };
    seen.add(val);
    const result = [];
    for (let i = 0; i < val.length; i++) {
      if (i in val) {
        result[i] = normalizeForComparison(val[i], seen);
      }
    }
    result.length = val.length;
    return result;
  }
  if (typeof val === 'object') {
    if (seen.has(val)) return { __type: 'CircularRef' };
    seen.add(val);
    const result = {};
    for (const key of Object.keys(val)) {
      result[key] = normalizeForComparison(val[key], seen);
    }
    return result;
  }
  return val;
}

/**
 * Deep equality check
 */
function deepEquals(a, b) {
  if (a === b) return true;
  if (typeof a !== typeof b) return false;
  if (a === null || b === null) return a === b;
  if (typeof a !== 'object') return a === b;

  if (Array.isArray(a)) {
    if (!Array.isArray(b)) return false;
    if (a.length !== b.length) return false;
    for (let i = 0; i < a.length; i++) {
      const aHas = i in a;
      const bHas = i in b;
      if (aHas !== bHas) return false;
      if (aHas && !deepEquals(a[i], b[i])) return false;
    }
    return true;
  }

  const aKeys = Object.keys(a).sort();
  const bKeys = Object.keys(b).sort();
  if (aKeys.length !== bKeys.length) return false;
  for (let i = 0; i < aKeys.length; i++) {
    if (aKeys[i] !== bKeys[i]) return false;
    if (!deepEquals(a[aKeys[i]], b[bKeys[i]])) return false;
  }
  return true;
}

/**
 * Describe a value for logging
 */
function describeValue(val) {
  if (val === null) return 'null';
  if (val === undefined) return 'undefined';
  if (typeof val === 'boolean') return `boolean(${val})`;
  if (typeof val === 'number') {
    if (Number.isNaN(val)) return 'NaN';
    if (val === Infinity) return 'Infinity';
    if (val === -Infinity) return '-Infinity';
    if (Object.is(val, -0)) return '-0';
    return `number(${val})`;
  }
  if (typeof val === 'bigint') return `bigint(${val}n)`;
  if (typeof val === 'string') return `string(${val.length} chars)`;
  if (val instanceof Date) return `Date(${val.toISOString()})`;
  if (val instanceof RegExp) return `RegExp(/${val.source}/${val.flags})`;
  if (val instanceof Map) return `Map(${val.size} entries)`;
  if (val instanceof Set) return `Set(${val.size} values)`;
  if (val instanceof ArrayBuffer) return `ArrayBuffer(${val.byteLength} bytes)`;
  if (ArrayBuffer.isView(val)) return `${val.constructor.name}(${val.length} elements)`;
  if (val instanceof Error) return `${val.constructor.name}("${val.message}")`;
  if (val instanceof Number) return `Number(${val.valueOf()})`;
  if (val instanceof Boolean) return `Boolean(${val.valueOf()})`;
  if (val instanceof String) return `String("${val.valueOf()}")`;
  if (Array.isArray(val)) return `Array(${val.length} elements)`;
  if (typeof val === 'object') return `Object(${Object.keys(val).length} keys)`;
  return String(val);
}

// Main
const args = process.argv.slice(2);

if (args.length === 0) {
  console.log('V8 Deserialization Verifier');
  console.log('Usage:');
  console.log('  node verify.js <hex-string>           # Verify single hex string');
  console.log('  node verify.js --dir <path>           # Verify all .bin files in directory');
  console.log('');
  console.log('Examples:');
  console.log('  node verify.js ff0f30                 # null');
  console.log('  node verify.js ff0f4954               # int32(42)');
  console.log('  node verify.js --dir ./go-serialized  # batch verify');
  process.exit(0);
}

// Check for --dir mode
if (args[0] === '--dir') {
  if (!args[1]) {
    console.error('Error: --dir requires a path argument');
    process.exit(1);
  }
  const dirPath = path.resolve(args[1]);
  if (!fs.existsSync(dirPath)) {
    console.error(`Error: Directory does not exist: ${dirPath}`);
    process.exit(1);
  }
  const result = verifyDirectory(dirPath);
  process.exit(result.failed > 0 ? 1 : 0);
}

// Single hex string mode
const hex = args[0].replace(/\s/g, '').toLowerCase();

try {
  const buf = hexToBuffer(hex);
  console.log(`Input: ${hex} (${buf.length} bytes)`);

  const val = v8.deserialize(buf);
  console.log(`Type: ${typeof val}${val === null ? ' (null)' : ''}`);
  console.log(`Value: ${prettyPrint(val)}`);

  // Re-serialize to verify round-trip
  const reserialized = v8.serialize(val);
  const reserializedHex = reserialized.toString('hex');

  if (reserializedHex === hex) {
    console.log('Round-trip: ✓ (bytes match exactly)');
  } else {
    console.log(`Round-trip: Node re-serialized to ${reserializedHex}`);
    // Try deserializing again to see if semantically equal
    const val2 = v8.deserialize(reserialized);
    console.log(`Re-deserialized: ${prettyPrint(val2)}`);
  }
} catch (err) {
  console.error(`Error: ${err.message}`);
  process.exit(1);
}
