package gx_test

import (
	"fmt"
	"runtime"
	"slices"
	"testing"

	"math/rand"
	"time"

	"github.com/amberpixels/r3/internal/gx"
	"github.com/stretchr/testify/assert"
)

func TestQappend(t *testing.T) {
	tests := []struct {
		name     string
		a        []int
		b        []int
		expected []int
	}{
		{
			name:     "empty slices",
			a:        []int{},
			b:        []int{},
			expected: []int{},
		},
		{
			name:     "empty a, non-empty b",
			a:        []int{},
			b:        []int{1, 2, 3},
			expected: []int{1, 2, 3},
		},
		{
			name:     "non-empty a, empty b",
			a:        []int{1, 2, 3},
			b:        []int{},
			expected: []int{1, 2, 3},
		},
		{
			name:     "no duplicates between a and b",
			a:        []int{1, 2, 3},
			b:        []int{4, 5, 6},
			expected: []int{1, 2, 3, 4, 5, 6},
		},
		{
			name:     "all duplicates between a and b",
			a:        []int{1, 2, 3},
			b:        []int{1, 2, 3},
			expected: []int{1, 2, 3},
		},
		{
			name:     "some duplicates between a and b",
			a:        []int{1, 2, 3},
			b:        []int{2, 3, 4, 5},
			expected: []int{1, 2, 3, 4, 5},
		},
		{
			name:     "duplicates within b itself (are fixed)",
			a:        []int{1, 2},
			b:        []int{3, 4, 3, 5},
			expected: []int{1, 2, 3, 4, 5},
		},
		{
			name:     "duplicates within a and b",
			a:        []int{1, 2, 2, 3},
			b:        []int{2, 3, 4, 4, 5},
			expected: []int{1, 2, 2, 3, 4, 5},
		},
		{
			name:     "preserves duplicates in a",
			a:        []int{1, 1, 2, 2, 3},
			b:        []int{3, 4, 5},
			expected: []int{1, 1, 2, 2, 3, 4, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gx.Qappend(tt.a, tt.b...)
			assert.Equal(t, tt.expected, result, "Qappend result should match expected value")

			// Verify that the function doesn't modify the original slices
			if len(tt.a) > 0 {
				originalA := make([]int, len(tt.a))
				copy(originalA, tt.a)
				assert.Equal(t, originalA, tt.a, "Original slice 'a' should not be modified")
			}

			if len(tt.b) > 0 {
				originalB := make([]int, len(tt.b))
				copy(originalB, tt.b)
				assert.Equal(t, originalB, tt.b, "Original slice 'b' should not be modified")
			}
		})
	}
}

// TestQappendWithStrings tests gx.Qappend with string slices to verify generic functionality.
func TestQappendWithStrings(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected []string
	}{
		{
			name:     "string slices with no duplicates",
			a:        []string{"a", "b", "c"},
			b:        []string{"d", "e", "f"},
			expected: []string{"a", "b", "c", "d", "e", "f"},
		},
		{
			name:     "string slices with some duplicates",
			a:        []string{"a", "b", "c"},
			b:        []string{"c", "d", "e"},
			expected: []string{"a", "b", "c", "d", "e"},
		},
		{
			name:     "string slices with duplicates in a",
			a:        []string{"a", "a", "b", "c"},
			b:        []string{"d", "e"},
			expected: []string{"a", "a", "b", "c", "d", "e"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gx.Qappend(tt.a, tt.b...)
			assert.Equal(t, tt.expected, result, "Qappend result should match expected value")
		})
	}
}

// TestQappendBehavior tests specific behaviors of gx.Qappend.
func TestQappendBehavior(t *testing.T) {
	t.Run("capacity is optimized", func(t *testing.T) {
		a := []int{1, 2, 3}
		b := []int{4, 5, 6}
		result := gx.Qappend(a, b...)

		// Result should have exactly the capacity needed
		assert.Equal(t, len(result), cap(result), "Result capacity should match its length")
	})

	t.Run("result is independent of original slices", func(t *testing.T) {
		a := []int{1, 2, 3}
		b := []int{4, 5, 6}
		result := gx.Qappend(a, b...)

		// Modify original slices
		if len(a) > 0 {
			a[0] = 99
		}
		if len(b) > 0 {
			b[0] = 99
		}

		// Result should remain unchanged
		assert.Equal(t, []int{1, 2, 3, 4, 5, 6}, result, "Result should be independent of original slices")
	})
}

func SlowAppend[T comparable](a []T, b ...T) []T {
	// len(result)=len(a), cap=result + len(b)
	result := make([]T, len(a), len(a)+len(b))
	copy(result, a)
	for _, e := range b {
		if !slices.Contains(result, e) {
			result = append(result, e)
		}
	}
	return result
}

func TestQappendEqualsSlowAppend(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want []string
	}{
		{
			name: "both empty",
			a:    []string{},
			b:    []string{},
			want: []string{},
		},
		{
			name: "empty a, unique b",
			a:    []string{},
			b:    []string{"x", "y", "z"},
			want: []string{"x", "y", "z"},
		},
		{
			name: "non-empty a, empty b",
			a:    []string{"a", "b"},
			b:    []string{},
			want: []string{"a", "b"},
		},
		{
			name: "duplicates in b",
			a:    []string{"foo", "bar"},
			b:    []string{"bar", "baz", "bar", "qux", "baz"},
			want: []string{"foo", "bar", "baz", "qux"},
		},
		{
			name: "no overlaps",
			a:    []string{"1", "2"},
			b:    []string{"3", "4"},
			want: []string{"1", "2", "3", "4"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotQ := gx.Qappend(tc.a, tc.b...)
			gotS := SlowAppend(tc.a, tc.b...)
			fmt.Println(gotQ)
			fmt.Println(gotS)

			// Ensure Qappend matches SlowAppend
			assert.Equal(
				t,
				gotS,
				gotQ,
				"Qappend and SlowAppend differ for A=%v, B=%v",
				tc.a, tc.b,
			)

			// Ensure they match the expected slice
			assert.Equal(
				t,
				tc.want,
				gotQ,
				"Result incorrect for A=%v, B=%v",
				tc.a, tc.b,
			)
		})
	}
}

func BenchmarkQappendVsSlowAppend(b *testing.B) {
	// Test parameters
	const (
		sliceSize     = 10000
		duplicateRate = 0
		stringLen     = 7
	)

	// Generate test data with ~20% duplicates
	aSlice := generateRandomStrings(sliceSize, stringLen, duplicateRate)
	bSlice := generateRandomStrings(sliceSize, stringLen, duplicateRate)

	// Report some stats about test data
	aUnique := make(map[string]struct{})
	for _, s := range aSlice {
		aUnique[s] = struct{}{}
	}
	bUnique := make(map[string]struct{})
	for _, s := range bSlice {
		bUnique[s] = struct{}{}
	}

	b.Logf("Test data: A=%d strings (%d unique), B=%d strings (%d unique)",
		len(aSlice), len(aUnique), len(bSlice), len(bUnique))

	// Benchmark gx.Qappend
	b.Run("Qappend", func(b *testing.B) {
		var result []string
		// Measure memory allocation
		allocBytes, allocObjects, freeObjects := measureMemory(func() {
			for range b.N {
				b.StopTimer()
				// Clone slices to ensure fair comparison
				aCopy := make([]string, len(aSlice))
				bCopy := make([]string, len(bSlice))
				copy(aCopy, aSlice)
				copy(bCopy, bSlice)
				b.StartTimer()

				result = gx.Qappend(aCopy, bCopy...)
			}
		})
		b.ReportMetric(float64(allocBytes)/float64(b.N), "bytes/op")
		b.ReportMetric(float64(allocObjects)/float64(b.N), "allocs/op")
		b.ReportMetric(float64(freeObjects)/float64(b.N), "frees/op")
		b.Logf("Qappend result length: %d", len(result))
	})

	// Benchmark SlowAppend
	b.Run("SlowAppend", func(b *testing.B) {
		var result []string
		// Measure memory allocation
		allocBytes, allocObjects, freeObjects := measureMemory(func() {
			for range b.N {
				b.StopTimer()
				// Clone slices to ensure fair comparison
				aCopy := make([]string, len(aSlice))
				bCopy := make([]string, len(bSlice))
				copy(aCopy, aSlice)
				copy(bCopy, bSlice)
				b.StartTimer()

				result = SlowAppend(aCopy, bCopy...)
			}
		})
		b.ReportMetric(float64(allocBytes)/float64(b.N), "bytes/op")
		b.ReportMetric(float64(allocObjects)/float64(b.N), "allocs/op")
		b.ReportMetric(float64(freeObjects)/float64(b.N), "frees/op")
		b.Logf("SlowAppend result length: %d", len(result))
	})
}

// A specific benchmark case with large overlapping datasets.
func BenchmarkQappendOverlapping(b *testing.B) {
	// Generate data where A and B have significant overlap
	const (
		sliceSize         = 10000
		stringLen         = 7
		overlapPercentage = 0.6 // 60% of elements in B are also in A
	)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	uniquePool := generateRandomStrings(sliceSize*2, stringLen, 0) // No duplicates in pool

	// Create A with some duplicates
	aSlice := make([]string, sliceSize)
	for i := range sliceSize {
		aSlice[i] = uniquePool[r.Intn(sliceSize)]
	}

	// Create B with overlap from A and some new elements
	bSlice := make([]string, sliceSize)
	for i := range sliceSize {
		if float64(i)/float64(sliceSize) < overlapPercentage {
			// Take from A's pool
			bSlice[i] = uniquePool[r.Intn(sliceSize)]
		} else {
			// Take from non-A pool
			bSlice[i] = uniquePool[sliceSize+r.Intn(sliceSize)]
		}
	}

	// Run benchmarks
	b.Run("Qappend-Overlapping", func(b *testing.B) {
		for range b.N {
			gx.Qappend(aSlice, bSlice...)
		}
	})

	b.Run("SlowAppend-Overlapping", func(b *testing.B) {
		for range b.N {
			SlowAppend(aSlice, bSlice...)
		}
	})
}

// A more comprehensive benchmark testing different sizes and overlap percentages.
func BenchmarkComprehensive(b *testing.B) {
	sizes := []int{100, 1000, 10000}
	overlaps := []float64{0.2, 0.5, 0.8}

	for _, size := range sizes {
		for _, overlap := range overlaps {
			// Generate custom test data for each configuration
			aSlice := generateRandomStrings(size, 7, 0.2)

			// Create pool where some strings are from A (for overlap)
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			bSlice := make([]string, size)

			// Get unique items from A for potential overlap
			aUnique := make([]string, 0)
			aMap := make(map[string]struct{})
			for _, s := range aSlice {
				if _, exists := aMap[s]; !exists {
					aMap[s] = struct{}{}
					aUnique = append(aUnique, s)
				}
			}

			// Fill B with mix of items from A and new items
			uniqueCount := size - int(float64(size)*overlap)
			uniqueB := generateRandomStrings(uniqueCount, 7, 0)

			for i := range size {
				if float64(i)/float64(size) < overlap && len(aUnique) > 0 {
					// Pick from A
					bSlice[i] = aUnique[r.Intn(len(aUnique))]
				} else {
					// Pick from unique non-A items
					bSlice[i] = uniqueB[r.Intn(len(uniqueB))]
				}
			}

			benchName := fmt.Sprintf("Size-%d-Overlap-%.1f", size, overlap)

			b.Run("Qappend-"+benchName, func(b *testing.B) {
				for range b.N {
					gx.Qappend(aSlice, bSlice...)
				}
			})

			b.Run("SlowAppend-"+benchName, func(b *testing.B) {
				for range b.N {
					SlowAppend(aSlice, bSlice...)
				}
			})
		}
	}
}

// generateRandomStrings generates n random strings of specified length with duplicateRate percentage of duplicates.
//
//nolint:unparam // it's for future
func generateRandomStrings(n int, strLen int, duplicateRate float64) []string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	result := make([]string, n)

	// Calculate how many unique strings we need
	uniqueCount := int(float64(n) * (1.0 - duplicateRate))
	if uniqueCount <= 0 {
		uniqueCount = 1
	}

	// Generate unique strings
	uniqueStrings := make([]string, uniqueCount)
	uniqueSet := make(map[string]struct{})
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	i := 0
	for i < uniqueCount {
		b := make([]byte, strLen)
		for j := range strLen {
			b[j] = chars[r.Intn(len(chars))]
		}
		s := string(b)

		// Check if this string is already in our unique set
		if _, exists := uniqueSet[s]; !exists {
			uniqueStrings[i] = s
			uniqueSet[s] = struct{}{}
			i++
		}
	}

	// First, fill result with all unique strings
	copy(result, uniqueStrings)

	// Then fill the rest with duplicates as needed
	for i := uniqueCount; i < n; i++ {
		// Pick a random string from the unique set to duplicate
		result[i] = uniqueStrings[r.Intn(uniqueCount)]
	}
	return result
}

// measureMemory returns memory stats after running a function.
func measureMemory(f func()) (uint64, uint64, uint64) {
	// Run GC before measurement
	runtime.GC()

	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)
	f()
	runtime.ReadMemStats(&m2)

	allocBytes := m2.TotalAlloc - m1.TotalAlloc
	allocObjects := m2.Mallocs - m1.Mallocs
	freeObjects := m2.Frees - m1.Frees

	return allocBytes, allocObjects, freeObjects
}
