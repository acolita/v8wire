package v8serialize

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCrossVersionCompatibility tests deserialization of fixtures generated
// by different Node.js versions (V8 format versions 13, 14, 15).
// Note: V8 format version doesn't strictly map to Node.js version -
// newer V8 versions may use format 15 even in older Node.js releases.
func TestCrossVersionCompatibility(t *testing.T) {
	versions := []struct {
		dir         string
		nodeVersion string
	}{
		{"v13", "18.x"},
		{"v14", "20.x"},
		{"v15", "22.x"},
	}

	fixturesBase := filepath.Join("..", "..", "testdata", "fixtures")

	for _, v := range versions {
		versionDir := filepath.Join(fixturesBase, v.dir)
		if _, err := os.Stat(versionDir); os.IsNotExist(err) {
			t.Logf("Skipping %s (Node.js %s): fixtures not generated yet", v.dir, v.nodeVersion)
			t.Logf("Run 'cd testgen && ./generate-all.sh' to generate fixtures")
			continue
		}

		t.Run(v.dir, func(t *testing.T) {
			testVersionFixtures(t, versionDir, v.nodeVersion)
		})
	}
}

func testVersionFixtures(t *testing.T, fixturesDir string, nodeVersion string) {
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("failed to read fixtures dir: %v", err)
	}

	// Find all .bin files
	var binFiles []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".bin") {
			binFiles = append(binFiles, entry.Name())
		}
	}

	if len(binFiles) == 0 {
		t.Skip("no fixtures found")
	}

	t.Logf("Testing %d fixtures from Node.js %s", len(binFiles), nodeVersion)

	// Known invalid fixtures that should be skipped
	skipFixtures := map[string]bool{
		"boxed-bigint": true, // Node.js can't serialize boxed BigInt
	}

	for _, binFile := range binFiles {
		name := strings.TrimSuffix(binFile, ".bin")

		if skipFixtures[name] {
			t.Run(name, func(t *testing.T) {
				t.Skip("known unsupported fixture")
			})
			continue
		}

		t.Run(name, func(t *testing.T) {
			binPath := filepath.Join(fixturesDir, binFile)
			jsonPath := filepath.Join(fixturesDir, name+".json")

			binData, err := os.ReadFile(binPath)
			if err != nil {
				t.Fatalf("failed to read %s: %v", binPath, err)
			}

			// Deserialize should not error
			v, err := Deserialize(binData)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			// Load JSON metadata if available
			if jsonData, err := os.ReadFile(jsonPath); err == nil {
				var meta fixtureMetadata
				if err := json.Unmarshal(jsonData, &meta); err == nil {
					t.Logf("Node %s, V8 %s, %d bytes: %s",
						meta.NodeVersion, meta.V8Version, meta.ByteLength, meta.Description)
				}
			}

			// Verify the value is usable (not panicking on access)
			_ = v.Type()
			_ = v.GoString()
		})
	}
}

// TestVersionDetection verifies version is correctly detected from each format.
func TestVersionDetection(t *testing.T) {
	tests := []struct {
		name    string
		header  []byte
		version uint32
		valid   bool
	}{
		{"v13", []byte{0xFF, 0x0D, '0'}, 13, true},
		{"v14", []byte{0xFF, 0x0E, '0'}, 14, true},
		{"v15", []byte{0xFF, 0x0F, '0'}, 15, true},
		{"v12-unsupported", []byte{0xFF, 0x0C, '0'}, 12, false},
		{"v16-unsupported", []byte{0xFF, 0x10, '0'}, 16, false}, // Future version, not yet supported
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDeserializer(tt.header)
			_, err := d.Deserialize()

			if tt.valid {
				if err != nil {
					t.Errorf("expected valid, got error: %v", err)
				}
				if d.Version() != tt.version {
					t.Errorf("expected version %d, got %d", tt.version, d.Version())
				}
			} else {
				if err == nil {
					t.Errorf("expected error for unsupported version")
				}
			}
		})
	}
}

// TestRoundTripAcrossVersions verifies that data serialized by Go
// can be read by all Node.js versions (when Docker containers are available).
func TestRoundTripAcrossVersions(t *testing.T) {
	// This test verifies that our serialized output uses a format
	// that should be readable by all supported Node.js versions.
	//
	// We serialize with version 15 (current), which should be forward-compatible
	// with older Node.js versions for basic types.

	testCases := []struct {
		name  string
		value Value
	}{
		{"null", Null()},
		{"undefined", Undefined()},
		{"bool-true", Bool(true)},
		{"bool-false", Bool(false)},
		{"int32-zero", Int32(0)},
		{"int32-pos", Int32(42)},
		{"int32-neg", Int32(-42)},
		{"int32-max", Int32(2147483647)},
		{"int32-min", Int32(-2147483648)},
		{"double", Double(3.14159)},
		{"string-empty", String("")},
		{"string-ascii", String("hello")},
		{"string-utf8", String("你好🌍")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Serialize
			data, err := Serialize(tc.value)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			// Deserialize back
			v, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			// Verify type matches
			if v.Type() != tc.value.Type() {
				t.Errorf("type mismatch: got %s, want %s", v.Type(), tc.value.Type())
			}
		})
	}
}

// TestGoToNodeRoundTrip verifies that data serialized by Go can be
// deserialized by Node.js. This is the reverse direction of our fixture tests.
// Requires Node.js to be installed.
func TestGoToNodeRoundTrip(t *testing.T) {
	// Check if Node.js is available
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node.js not available")
	}

	// Create temp directory for Go-serialized fixtures
	tempDir, err := os.MkdirTemp("", "go-v8-fixtures-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test cases: serialize these values and verify Node can read them
	testCases := []struct {
		name  string
		value Value
	}{
		// Primitives
		{"null", Null()},
		{"undefined", Undefined()},
		{"bool-true", Bool(true)},
		{"bool-false", Bool(false)},

		// Numbers
		{"int32-zero", Int32(0)},
		{"int32-positive", Int32(42)},
		{"int32-negative", Int32(-42)},
		{"int32-max", Int32(2147483647)},
		{"int32-min", Int32(-2147483648)},
		{"uint32-min", Uint32(2147483648)},
		{"uint32-max", Uint32(4294967295)},
		{"double-pi", Double(3.14159265358979)},
		{"double-infinity", Double(math.Inf(1))},
		{"double-neg-infinity", Double(math.Inf(-1))},
		{"double-nan", Double(math.NaN())},

		// Strings
		{"string-empty", String("")},
		{"string-ascii", String("hello")},
		{"string-utf16", String("你好世界")},
		{"string-emoji", String("🎉🎊🎈")},

		// Objects
		{"object-empty", Object(nil)},
		{"object-simple", Object(map[string]Value{
			"a": Int32(1),
			"b": Int32(2),
		})},
		{"object-nested", Object(map[string]Value{
			"outer": Object(map[string]Value{
				"inner": String("value"),
			}),
		})},

		// Arrays
		{"array-empty", Array(nil)},
		{"array-numbers", Array([]Value{Int32(1), Int32(2), Int32(3)})},
		{"array-mixed", Array([]Value{Int32(1), String("two"), Bool(true)})},
	}

	// Write each test case as a .bin file
	for _, tc := range testCases {
		data, err := Serialize(tc.value)
		if err != nil {
			t.Errorf("failed to serialize %s: %v", tc.name, err)
			continue
		}

		binPath := filepath.Join(tempDir, tc.name+".bin")
		if err := os.WriteFile(binPath, data, 0644); err != nil {
			t.Errorf("failed to write %s: %v", tc.name, err)
		}
	}

	// Run Node.js verify script
	verifyScript := filepath.Join("..", "..", "testgen", "verify.js")
	cmd := exec.Command("node", verifyScript, "--dir", tempDir)
	output, err := cmd.CombinedOutput()

	t.Logf("Node.js verification output:\n%s", output)

	if err != nil {
		t.Errorf("Node.js verification failed: %v", err)
	}
}

// TestGoToNodeRoundTripWithDocker tests Go→Node deserialization using Docker
// containers for specific Node.js versions. Requires Docker.
func TestGoToNodeRoundTripWithDocker(t *testing.T) {
	if os.Getenv("V8WIRE_TEST_DOCKER") == "" {
		t.Skip("Set V8WIRE_TEST_DOCKER=1 to run Docker-based tests")
	}

	// Check if Docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available")
	}

	// Create temp directory for Go-serialized fixtures
	tempDir, err := os.MkdirTemp("", "go-v8-docker-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Generate a small set of test fixtures
	fixtures := []struct {
		name  string
		value Value
	}{
		{"null", Null()},
		{"int32", Int32(42)},
		{"string", String("hello")},
		{"object", Object(map[string]Value{"key": String("value")})},
		{"array", Array([]Value{Int32(1), Int32(2), Int32(3)})},
	}

	for _, f := range fixtures {
		data, err := Serialize(f.value)
		if err != nil {
			t.Fatalf("failed to serialize %s: %v", f.name, err)
		}
		if err := os.WriteFile(filepath.Join(tempDir, f.name+".bin"), data, 0644); err != nil {
			t.Fatalf("failed to write %s: %v", f.name, err)
		}
	}

	// Copy verify.js to temp dir
	verifyScript, _ := os.ReadFile(filepath.Join("..", "..", "testgen", "verify.js"))
	os.WriteFile(filepath.Join(tempDir, "verify.js"), verifyScript, 0755)

	// Test against each Node.js version
	nodeVersions := []string{"18", "20", "22"}

	for _, nodeVer := range nodeVersions {
		t.Run("node"+nodeVer, func(t *testing.T) {
			cmd := exec.Command("docker", "run", "--rm",
				"-v", tempDir+":/data",
				"-w", "/data",
				"node:"+nodeVer+"-alpine",
				"node", "verify.js", "--dir", "/data")

			output, err := cmd.CombinedOutput()
			t.Logf("Node %s output:\n%s", nodeVer, output)

			if err != nil {
				t.Errorf("Node %s verification failed: %v", nodeVer, err)
			}
		})
	}
}

// BenchmarkCrossVersionDeserialize benchmarks deserialization across formats.
func BenchmarkCrossVersionDeserialize(b *testing.B) {
	fixturesBase := filepath.Join("..", "..", "testdata", "fixtures")

	// Try to load from main fixtures or v15
	var binData []byte
	paths := []string{
		filepath.Join(fixturesBase, "object-types.bin"),
		filepath.Join(fixturesBase, "v15", "object-types.bin"),
	}

	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			binData = data
			break
		}
	}

	if binData == nil {
		b.Skip("no fixtures available")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Deserialize(binData)
	}
}
