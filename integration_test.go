package netjugo

import (
	"os"
	"testing"
)

func TestFullWorkflowIntegration(t *testing.T) {
	// Test the complete workflow: load, configure, aggregate, exclude, export
	pa := NewPrefixAggregator()

	// Step 1: Configure minimum prefix lengths
	err := pa.SetMinPrefixLength(24, 48)
	if err != nil {
		t.Fatalf("Failed to set min prefix lengths: %v", err)
	}

	// Step 2: Add base prefixes (smaller ranges for faster testing)
	basePrefixes := []string{
		"192.168.0.0/22",     // Will be split to 4x /24s
		"10.0.0.0/23",        // Will be split to 2x /24s
		"172.16.0.0/23",      // Will be split to 2x /24s
		"2001:db8::/47",      // Will be split to 2x /48s
		"2001:db8:1000::/47", // Will be split to 2x /48s
	}

	for _, prefix := range basePrefixes {
		err := pa.AddPrefix(prefix)
		if err != nil {
			t.Fatalf("Failed to add base prefix %s: %v", prefix, err)
		}
	}

	// Step 3: Set include prefixes
	includePrefixes := []string{
		"203.0.113.0/24",
		"198.51.100.0/24",
		"2001:db8:2000::/48",
	}

	err = pa.SetIncludePrefixes(includePrefixes)
	if err != nil {
		t.Fatalf("Failed to set include prefixes: %v", err)
	}

	// Step 4: Set exclude prefixes
	excludePrefixes := []string{
		"192.168.1.0/24",    // Exclude specific /24
		"10.0.0.0/24",       // Exclude specific /24
		"2001:db8:0:1::/64", // Exclude specific /64 from split /48s
	}

	err = pa.SetExcludePrefixes(excludePrefixes)
	if err != nil {
		t.Fatalf("Failed to set exclude prefixes: %v", err)
	}

	// Step 5: Get initial stats
	initialStats := pa.GetStats()
	t.Logf("Initial prefixes: %d", initialStats.TotalPrefixes)

	// Step 6: Perform aggregation
	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Aggregation failed: %v", err)
	}

	// Step 7: Verify results
	finalStats := pa.GetStats()

	// Original count only includes prefixes added via AddPrefix, not includes
	expectedOriginal := len(basePrefixes)
	if finalStats.OriginalCount != expectedOriginal {
		t.Errorf("Expected %d original prefixes, got %d", expectedOriginal, finalStats.OriginalCount)
	}

	// Should have some aggregated prefixes
	if finalStats.TotalPrefixes == 0 {
		t.Error("Expected some aggregated prefixes, got 0")
	}

	// Processing time should be reasonable
	if finalStats.ProcessingTimeMs > 1000 {
		t.Errorf("Processing took %d ms, expected < 1000ms for small dataset", finalStats.ProcessingTimeMs)
	}

	// Step 8: Verify excluded prefixes are not in results
	allPrefixes := pa.GetPrefixes()
	prefixMap := make(map[string]bool)
	for _, prefix := range allPrefixes {
		prefixMap[prefix] = true
	}

	for _, excluded := range excludePrefixes {
		if prefixMap[excluded] {
			t.Errorf("Excluded prefix %s found in results", excluded)
		}
	}

	// Step 10: Test file I/O
	tempFile := "/tmp/integration_test_output.txt"
	err = pa.WriteToFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to write to file: %v", err)
	}
	defer func() {
		if err := os.Remove(tempFile); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()

	// Read back and verify
	pa2 := NewPrefixAggregator()
	err = pa2.AddFromFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read from file: %v", err)
	}

	readPrefixes := pa2.GetPrefixes()
	if len(readPrefixes) != len(allPrefixes) {
		t.Errorf("File I/O round trip failed: wrote %d prefixes, read %d", len(allPrefixes), len(readPrefixes))
	}

	// Step 11: Memory usage verification
	memStats := pa.GetMemoryStats()
	if memStats.AggregatorBytes <= 0 {
		t.Error("Memory usage calculation should be positive")
	}

	t.Logf("Integration test completed successfully")
	t.Logf("  Original: %d, Final: %d", finalStats.OriginalCount, finalStats.TotalPrefixes)
	t.Logf("  Reduction: %.2f%%", finalStats.ReductionRatio*100)
	t.Logf("  Processing: %d ms", finalStats.ProcessingTimeMs)
	t.Logf("  Memory: %d bytes", memStats.AggregatorBytes)
}

func TestRealWorldDatasetSample(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real-world dataset test in short mode")
	}

	// Test with a sample of the large dataset
	samplePath := ".samples/sample-100k-prefixes.txt"
	if _, err := os.Stat(samplePath); os.IsNotExist(err) {
		t.Skipf("Sample dataset not found at %s", samplePath)
	}

	pa := NewPrefixAggregator()

	// Load the sample
	err := pa.AddFromFile(samplePath)
	if err != nil {
		t.Fatalf("Failed to load sample dataset: %v", err)
	}

	initialStats := pa.GetStats()
	if initialStats.OriginalCount < 10000 {
		t.Skipf("Sample too small (%d prefixes), skipping", initialStats.OriginalCount)
	}

	// Aggregate
	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Aggregation failed: %v", err)
	}

	finalStats := pa.GetStats()

	// Verify performance requirements
	maxTimeMs := int64(5000) // 5 seconds should be plenty for 100k prefixes
	if finalStats.ProcessingTimeMs > maxTimeMs {
		t.Errorf("Processing took %d ms, expected < %d ms", finalStats.ProcessingTimeMs, maxTimeMs)
	}

	// Verify significant aggregation occurred
	if finalStats.ReductionRatio < 0.5 {
		t.Logf("Low reduction ratio: %.2f%% (may be expected for real-world data)", finalStats.ReductionRatio*100)
	}

	// Memory should be reasonable
	memStats := pa.GetMemoryStats()
	maxMemoryMB := int64(50) // 50MB should be plenty for 100k prefixes
	actualMemoryMB := memStats.AggregatorBytes / (1024 * 1024)
	if actualMemoryMB > maxMemoryMB {
		t.Errorf("Memory usage %d MB exceeds %d MB limit", actualMemoryMB, maxMemoryMB)
	}

	t.Logf("Real-world sample test completed:")
	t.Logf("  Processed: %d prefixes", finalStats.OriginalCount)
	t.Logf("  Result: %d prefixes", finalStats.TotalPrefixes)
	t.Logf("  Reduction: %.2f%%", finalStats.ReductionRatio*100)
	t.Logf("  Time: %d ms", finalStats.ProcessingTimeMs)
	t.Logf("  Memory: %d MB", actualMemoryMB)
}

func TestErrorHandlingIntegration(t *testing.T) {
	pa := NewPrefixAggregator()

	// Test invalid minimum prefix lengths
	err := pa.SetMinPrefixLength(-1, 64)
	if err == nil {
		t.Error("Expected error for negative IPv4 min length")
	}

	err = pa.SetMinPrefixLength(24, 129)
	if err == nil {
		t.Error("Expected error for IPv6 min length > 128")
	}

	// Test invalid prefixes in SetIncludePrefixes
	err = pa.SetIncludePrefixes([]string{"invalid-prefix"})
	if err == nil {
		t.Error("Expected error for invalid include prefix")
	}

	// Test invalid prefixes in SetExcludePrefixes
	err = pa.SetExcludePrefixes([]string{"not-a-prefix"})
	if err == nil {
		t.Error("Expected error for invalid exclude prefix")
	}

	// Test file not found
	err = pa.AddFromFile("nonexistent-file.txt")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}

	// Test write to invalid path
	err = pa.WriteToFile("/root/invalid-path/file.txt")
	if err == nil {
		t.Error("Expected error for invalid write path")
	}

	t.Log("Error handling integration test completed")
}

func TestThreadSafetyIntegration(t *testing.T) {
	pa := NewPrefixAggregator()

	// Add some initial data
	testPrefixes := []string{
		"192.168.1.0/24",
		"192.168.2.0/24",
		"10.0.0.0/24",
		"2001:db8::/64",
	}

	for _, prefix := range testPrefixes {
		err := pa.AddPrefix(prefix)
		if err != nil {
			t.Fatalf("Failed to add prefix %s: %v", prefix, err)
		}
	}

	err := pa.Aggregate()
	if err != nil {
		t.Fatalf("Initial aggregation failed: %v", err)
	}

	// Test concurrent read operations
	done := make(chan bool, 10)

	// Launch multiple concurrent readers
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			// Perform various read operations
			for j := 0; j < 100; j++ {
				_ = pa.GetStats()
				_ = pa.GetPrefixes()
				_ = pa.GetIPv4Prefixes()
				_ = pa.GetIPv6Prefixes()
				_ = pa.GetMemoryStats()
			}
		}()
	}

	// Wait for all readers to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	t.Log("Thread safety integration test completed")
}

func TestPerformanceRequirementsIntegration(t *testing.T) {
	// Test against the specific performance requirements from SOW

	t.Run("100K_Prefixes_Under_5_Seconds", func(t *testing.T) {
		prefixes := generateTestPrefixes(100000)
		pa := NewPrefixAggregator()

		for _, prefix := range prefixes {
			err := pa.AddPrefix(prefix)
			if err != nil {
				t.Fatalf("Failed to add prefix: %v", err)
			}
		}

		err := pa.Aggregate()
		if err != nil {
			t.Fatalf("Aggregation failed: %v", err)
		}

		stats := pa.GetStats()
		maxTimeMs := int64(5000) // 5 seconds
		if stats.ProcessingTimeMs > maxTimeMs {
			t.Errorf("100K prefixes took %d ms, requirement: < %d ms", stats.ProcessingTimeMs, maxTimeMs)
		} else {
			t.Logf("✓ 100K prefixes aggregated in %d ms (requirement: < %d ms)", stats.ProcessingTimeMs, maxTimeMs)
		}
	})

	t.Run("Memory_Scaling_Linear", func(t *testing.T) {
		sizes := []int{1000, 5000, 10000}
		var memoryPerPrefix []float64

		for _, size := range sizes {
			prefixes := generateTestPrefixes(size)
			pa := NewPrefixAggregator()

			for _, prefix := range prefixes {
				if err := pa.AddPrefix(prefix); err != nil {
					t.Logf("Failed to add prefix %s: %v", prefix, err)
				}
			}

			memStats := pa.GetMemoryStats()
			memPerPrefix := float64(memStats.AggregatorBytes) / float64(size)
			memoryPerPrefix = append(memoryPerPrefix, memPerPrefix)

			t.Logf("Size %d: %.2f bytes/prefix", size, memPerPrefix)
		}

		// Check that memory scaling is reasonable (not exponential)
		ratio := memoryPerPrefix[2] / memoryPerPrefix[0]
		if ratio > 2.0 {
			t.Errorf("Memory scaling not linear: ratio %.2f (10k vs 1k)", ratio)
		} else {
			t.Logf("✓ Memory scaling is linear: ratio %.2f", ratio)
		}
	})
}
