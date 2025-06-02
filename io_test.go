package netjugo

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestAddFromReader(t *testing.T) {
	pa := NewPrefixAggregator()

	input := `# Test file with comments
192.168.1.0/24
# Another comment
10.0.0.0/16

# Empty line above
2001:db8::/32
	# Indented comment
172.16.0.0/12   
`

	reader := strings.NewReader(input)
	err := pa.AddFromReader(reader)
	if err != nil {
		t.Fatalf("Failed to add from reader: %v", err)
	}

	prefixes := pa.GetPrefixes()
	expectedPrefixes := []string{
		"192.168.1.0/24",
		"10.0.0.0/16",
		"2001:db8::/32",
		"172.16.0.0/12",
	}

	if len(prefixes) != len(expectedPrefixes) {
		t.Errorf("Expected %d prefixes, got %d", len(expectedPrefixes), len(prefixes))
	}

	prefixMap := make(map[string]bool)
	for _, prefix := range prefixes {
		prefixMap[prefix] = true
	}

	for _, expected := range expectedPrefixes {
		if !prefixMap[expected] {
			t.Errorf("Expected prefix %s not found in result", expected)
		}
	}
}

func TestAddFromReaderWithErrors(t *testing.T) {
	pa := NewPrefixAggregator()

	input := `192.168.1.0/24
invalid-prefix
10.0.0.0/16`

	reader := strings.NewReader(input)
	err := pa.AddFromReader(reader)
	if err != nil {
		t.Errorf("Expected graceful degradation (no error), got: %v", err)
	}

	// Should have loaded the valid prefixes despite the invalid one
	prefixes := pa.GetPrefixes()
	if len(prefixes) != 2 {
		t.Errorf("Expected 2 valid prefixes to be loaded, got %d", len(prefixes))
	}
}

func TestAddFromFile(t *testing.T) {
	// Create a temporary file
	content := `192.168.1.0/24
10.0.0.0/16
# Comment
2001:db8::/32`

	tmpfile, err := os.CreateTemp("", "prefix_test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	pa := NewPrefixAggregator()
	err = pa.AddFromFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to add from file: %v", err)
	}

	prefixes := pa.GetPrefixes()
	if len(prefixes) != 3 {
		t.Errorf("Expected 3 prefixes, got %d", len(prefixes))
	}
}

func TestAddFromFileNotFound(t *testing.T) {
	pa := NewPrefixAggregator()
	err := pa.AddFromFile("nonexistent-file.txt")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
		return
	}

	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("Expected file not found error, got: %v", err)
	}
}

func TestWriteToWriter(t *testing.T) {
	pa := NewPrefixAggregator()

	testPrefixes := []string{
		"192.168.1.0/24",
		"10.0.0.0/16",
		"2001:db8::/32",
	}

	err := pa.AddPrefixes(testPrefixes)
	if err != nil {
		t.Fatalf("Failed to add prefixes: %v", err)
	}

	var buf bytes.Buffer
	err = pa.WriteToWriter(&buf)
	if err != nil {
		t.Fatalf("Failed to write to writer: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != len(testPrefixes) {
		t.Errorf("Expected %d lines, got %d", len(testPrefixes), len(lines))
	}

	for _, expectedPrefix := range testPrefixes {
		found := false
		for _, line := range lines {
			if strings.TrimSpace(line) == expectedPrefix {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected prefix %s not found in output", expectedPrefix)
		}
	}
}

func TestWriteToFile(t *testing.T) {
	pa := NewPrefixAggregator()

	testPrefixes := []string{
		"192.168.1.0/24",
		"10.0.0.0/16",
	}

	err := pa.AddPrefixes(testPrefixes)
	if err != nil {
		t.Fatalf("Failed to add prefixes: %v", err)
	}

	tmpfile, err := os.CreateTemp("", "prefix_output")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()
	if err := tmpfile.Close(); err != nil {
		t.Logf("Failed to close temp file: %v", err)
	} // Close it so WriteToFile can open it

	err = pa.WriteToFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to write to file: %v", err)
	}

	// Read the file back
	content, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)
	for _, expectedPrefix := range testPrefixes {
		if !strings.Contains(output, expectedPrefix) {
			t.Errorf("Expected prefix %s not found in file output", expectedPrefix)
		}
	}
}

func TestRoundTripFileIO(t *testing.T) {
	// Test writing to file and reading back
	pa1 := NewPrefixAggregator()

	originalPrefixes := []string{
		"192.168.1.0/24",
		"10.0.0.0/16",
		"2001:db8::/32",
		"172.16.0.0/12",
	}

	err := pa1.AddPrefixes(originalPrefixes)
	if err != nil {
		t.Fatalf("Failed to add original prefixes: %v", err)
	}

	// Write to file
	tmpfile, err := os.CreateTemp("", "roundtrip_test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()
	if err := tmpfile.Close(); err != nil {
		t.Logf("Failed to close temp file: %v", err)
	}

	err = pa1.WriteToFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to write to file: %v", err)
	}

	// Read back into new aggregator
	pa2 := NewPrefixAggregator()
	err = pa2.AddFromFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to read from file: %v", err)
	}

	// Compare results
	readPrefixes := pa2.GetPrefixes()

	if len(readPrefixes) != len(originalPrefixes) {
		t.Errorf("Expected %d prefixes after round trip, got %d", len(originalPrefixes), len(readPrefixes))
	}

	originalMap := make(map[string]bool)
	for _, prefix := range originalPrefixes {
		originalMap[prefix] = true
	}

	for _, prefix := range readPrefixes {
		if !originalMap[prefix] {
			t.Errorf("Unexpected prefix %s found after round trip", prefix)
		}
	}
}

func TestCommentAndEmptyLineHandling(t *testing.T) {
	pa := NewPrefixAggregator()

	input := `
# This is a header comment
# Another comment line

192.168.1.0/24

# Comment between prefixes
10.0.0.0/16


# Multiple empty lines above
2001:db8::/32
  # Indented comment
  
# Final comment`

	reader := strings.NewReader(input)
	err := pa.AddFromReader(reader)
	if err != nil {
		t.Fatalf("Failed to add from reader: %v", err)
	}

	prefixes := pa.GetPrefixes()
	expected := 3 // Only the 3 actual prefixes, comments and empty lines should be ignored

	if len(prefixes) != expected {
		t.Errorf("Expected %d prefixes, got %d", expected, len(prefixes))
	}
}
