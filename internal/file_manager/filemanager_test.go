package file_manager

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func init() {
	// Create logs directory if it doesn't exist
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Printf("Failed to create logs directory: %v", err)
	}

	// Redirect test logs to file
	logFile, err := os.OpenFile(filepath.Join("logs", "filemanager_test.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Failed to open test log file: %v", err)
	} else {
		log.SetOutput(logFile)
	}
}

// setupTestEnvironment creates a temporary test directory and redirects "data" to it
func setupTestEnvironment(t *testing.T) (string, func()) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "filemanager_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create a symbolic link to redirect "data" to our temp directory
	originalDataDir := "data"
	var originalDirExists bool
	if _, err := os.Stat(originalDataDir); err == nil {
		originalDirExists = true
		// Rename the original data directory temporarily
		os.Rename(originalDataDir, originalDataDir+"_backup")
	}

	// Create the data directory in our temp location
	os.Mkdir(filepath.Join(tempDir, "data"), 0755)
	os.Symlink(filepath.Join(tempDir, "data"), "data")

	// Return cleanup function
	cleanup := func() {
		os.Remove("data") // Remove the symlink
		os.RemoveAll(tempDir)
		if originalDirExists {
			os.Rename(originalDataDir+"_backup", originalDataDir)
		}
	}

	return tempDir, cleanup
}

func TestNewFileManager(t *testing.T) {
	// Test with sync disabled
	fm1 := NewFileManager(false)
	if fm1 == nil {
		t.Fatal("NewFileManager returned nil")
	}
	if fm1.openFiles == nil {
		t.Error("openFiles map not initialized")
	}
	if fm1.fileLocks == nil {
		t.Error("fileLocks map not initialized")
	}
	if fm1.logger == nil {
		t.Error("logger not initialized")
	}
	if fm1.syncEnabled != false {
		t.Error("syncEnabled should be false")
	}

	// Test with sync enabled
	fm2 := NewFileManager(true)
	if fm2.syncEnabled != true {
		t.Error("syncEnabled should be true")
	}
}

func TestBasicFileOperations(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	fm := NewFileManager(true) // Enable sync for testing

	// Test valid filename and content
	testContent := []byte("test content")
	err := fm.AppendToFile("file_0", testContent)
	if err != nil {
		t.Errorf("AppendToFile failed with valid input: %v", err)
	}

	// Verify content was written
	content, err := os.ReadFile(filepath.Join("data", "file_0"))
	if err != nil {
		t.Errorf("Failed to read file: %v", err)
	}
	if string(content) != string(testContent) {
		t.Errorf("File content mismatch. Got %s, want %s", content, testContent)
	}

	// Test appending more content
	moreContent := []byte(" additional content")
	err = fm.AppendToFile("file_0", moreContent)
	if err != nil {
		t.Errorf("Failed to append more content: %v", err)
	}

	// Verify appended content
	content, err = os.ReadFile(filepath.Join("data", "file_0"))
	if err != nil {
		t.Errorf("Failed to read file after append: %v", err)
	}
	expectedContent := string(testContent) + string(moreContent)
	if string(content) != expectedContent {
		t.Errorf("Appended content mismatch. Got %s, want %s", content, expectedContent)
	}
}

func TestFilenameValidation(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	fm := NewFileManager(false)
	testContent := []byte("test content")

	// Test cases for invalid filenames
	invalidFilenames := []struct {
		name  string
		input string
	}{
		{"invalid prefix", "invalid_file"},
		{"out of range (high)", "file_100"},
		{"out of range (negative)", "file_-1"},
		{"non-numeric", "file_abc"},
		{"empty", "file_"},
	}

	for _, tc := range invalidFilenames {
		t.Run(tc.name, func(t *testing.T) {
			err := fm.AppendToFile(tc.input, testContent)
			if err == nil {
				t.Errorf("AppendToFile should fail with %s", tc.input)
			}
		})
	}

	// Test all valid filenames
	for i := 0; i < 100; i++ {
		filename := fmt.Sprintf("file_%d", i)
		err := fm.AppendToFile(filename, testContent)
		if err != nil {
			t.Errorf("AppendToFile failed with valid filename %s: %v", filename, err)
		}
	}
}

func TestConcurrentSameFileAppends(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	fm := NewFileManager(false)
	filename := "file_0"

	// Number of goroutines and writes per goroutine
	numGoroutines := 10
	writesPerGoroutine := 100

	// Use a WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Each goroutine will write its ID followed by the iteration number
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < writesPerGoroutine; j++ {
				content := fmt.Sprintf("G%d-%d\n", id, j)
				err := fm.AppendToFile(filename, []byte(content))
				if err != nil {
					t.Errorf("Goroutine %d failed to append: %v", id, err)
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Verify the file contains the expected number of lines
	content, err := os.ReadFile(filepath.Join("data", filename))
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	lines := bytes.Count(content, []byte("\n"))
	expectedLines := numGoroutines * writesPerGoroutine
	if lines != expectedLines {
		t.Errorf("Expected %d lines, got %d", expectedLines, lines)
	}
}

func TestConcurrentMultiFileAppends(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	fm := NewFileManager(false)

	// Number of goroutines and files
	numGoroutines := 20
	numFiles := 10
	writesPerGoroutine := 50

	// Track total writes per file
	writesPerFile := make(map[string]int)
	var writesPerFileMu sync.Mutex

	// Use a WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Each goroutine will write to random files
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// Create a random number generator with a unique seed
			r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))

			for j := 0; j < writesPerGoroutine; j++ {
				// Choose a random file
				fileNum := r.Intn(numFiles)
				filename := fmt.Sprintf("file_%d", fileNum)

				// Write to the file
				content := fmt.Sprintf("G%d-%d\n", id, j)
				err := fm.AppendToFile(filename, []byte(content))
				if err != nil {
					t.Errorf("Goroutine %d failed to append to %s: %v", id, filename, err)
					return
				}

				// Track the write
				writesPerFileMu.Lock()
				writesPerFile[filename]++
				writesPerFileMu.Unlock()
			}
		}(i)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Verify each file has the expected number of writes
	totalWrites := 0
	for filename, count := range writesPerFile {
		content, err := os.ReadFile(filepath.Join("data", filename))
		if err != nil {
			t.Fatalf("Failed to read file %s: %v", filename, err)
		}

		lines := bytes.Count(content, []byte("\n"))
		if lines != count {
			t.Errorf("File %s: Expected %d lines, got %d", filename, count, lines)
		}

		totalWrites += count
	}

	expectedTotalWrites := numGoroutines * writesPerGoroutine
	if totalWrites != expectedTotalWrites {
		t.Errorf("Expected %d total writes, got %d", expectedTotalWrites, totalWrites)
	}
}

func TestStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	fm := NewFileManager(false)

	// Configuration
	numGoroutines := 100
	numFiles := 20
	writesPerGoroutine := 50
	dataSize := 1024 // 1KB per write

	// Generate random data once to reuse
	randomData := make([]byte, dataSize)
	rand.Read(randomData)

	// Track metrics
	var totalWrites int64
	startTime := time.Now()

	// Use a WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Start the goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// Create a random number generator with a unique seed
			r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))

			for j := 0; j < writesPerGoroutine; j++ {
				// Choose a random file
				fileNum := r.Intn(numFiles)
				filename := fmt.Sprintf("file_%d", fileNum)

				// Write to the file
				err := fm.AppendToFile(filename, randomData)
				if err != nil {
					t.Errorf("Goroutine %d failed to append to %s: %v", id, filename, err)
					return
				}

				atomic.AddInt64(&totalWrites, 1)
			}
		}(i)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Calculate metrics
	duration := time.Since(startTime)
	writesPerSecond := float64(totalWrites) / duration.Seconds()
	bytesWritten := int64(totalWrites) * int64(dataSize)
	mbWritten := float64(bytesWritten) / (1024 * 1024)
	mbPerSecond := mbWritten / duration.Seconds()

	t.Logf("Stress Test Results:")
	t.Logf("  Duration: %v", duration)
	t.Logf("  Total Writes: %d", totalWrites)
	t.Logf("  Writes/sec: %.2f", writesPerSecond)
	t.Logf("  Data Written: %.2f MB", mbWritten)
	t.Logf("  Throughput: %.2f MB/sec", mbPerSecond)

	// Verify all files exist and have content
	for i := 0; i < numFiles; i++ {
		filename := filepath.Join("data", fmt.Sprintf("file_%d", i))
		info, err := os.Stat(filename)
		if os.IsNotExist(err) {
			t.Errorf("File %s does not exist", filename)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("File %s is empty", filename)
		}
	}
}

func TestResourceLeaks(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	fm := NewFileManager(false)

	// Record initial number of goroutines
	initialGoroutines := runtime.NumGoroutine()

	// Perform a series of operations
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			filename := fmt.Sprintf("file_%d", j)
			content := []byte(fmt.Sprintf("test content %d-%d", i, j))
			err := fm.AppendToFile(filename, content)
			if err != nil {
				t.Fatalf("Failed to append to file: %v", err)
			}
		}
	}

	// Clean up
	fm.Cleanup()

	// Check for goroutine leaks
	time.Sleep(100 * time.Millisecond) // Give goroutines time to exit
	finalGoroutines := runtime.NumGoroutine()
	if finalGoroutines > initialGoroutines+5 { // Allow for some background goroutines
		t.Errorf("Possible goroutine leak: started with %d, ended with %d",
			initialGoroutines, finalGoroutines)
	}

	// Check that all file handles were closed
	fm.mu.Lock()
	openFilesCount := len(fm.openFiles)
	fm.mu.Unlock()

	if openFilesCount > 0 {
		t.Errorf("File handle leak: %d files still open after cleanup", openFilesCount)
	}
}

func TestCreateFiles(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	fm := NewFileManager(false)
	fm.CreateFiles()

	// Verify all 100 files were created
	for i := 0; i < 100; i++ {
		filename := filepath.Join("data", fmt.Sprintf("file_%d", i))
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			t.Errorf("File %s was not created", filename)
		}
	}
}

func TestCleanup(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	fm := NewFileManager(false)

	// Create and open files
	for i := 0; i < 10; i++ {
		filename := fmt.Sprintf("file_%d", i)
		err := fm.AppendToFile(filename, []byte("test"))
		if err != nil {
			t.Fatalf("Failed to append to file: %v", err)
		}
	}

	// Verify files are in openFiles map
	fm.mu.Lock()
	initialOpenFiles := len(fm.openFiles)
	fm.mu.Unlock()

	if initialOpenFiles == 0 {
		t.Error("No files in openFiles map")
	}

	// Test cleanup
	fm.Cleanup()

	// Verify openFiles map is empty
	fm.mu.Lock()
	finalOpenFiles := len(fm.openFiles)
	fm.mu.Unlock()

	if finalOpenFiles != 0 {
		t.Errorf("openFiles map should be empty after cleanup, but has %d entries", finalOpenFiles)
	}
}

func TestErrorHandling(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	fm := NewFileManager(false)

	// Test with read-only directory
	if runtime.GOOS != "windows" { // Skip on Windows as permissions work differently
		// Make data directory read-only
		err := os.Chmod("data", 0555)
		if err != nil {
			t.Fatalf("Failed to change directory permissions: %v", err)
		}

		// Try to write to a file
		err = fm.AppendToFile("file_0", []byte("test"))
		if err == nil {
			t.Error("Expected error when writing to read-only directory")
		}

		// Restore permissions
		os.Chmod("data", 0755)
	}
}

func BenchmarkAppendToFile(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "filemanager_bench")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a symbolic link to redirect "data" to our temp directory
	originalDataDir := "data"
	var originalDirExists bool
	if _, err := os.Stat(originalDataDir); err == nil {
		originalDirExists = true
		os.Rename(originalDataDir, originalDataDir+"_backup")
	}

	os.Mkdir(filepath.Join(tempDir, "data"), 0755)
	os.Symlink(filepath.Join(tempDir, "data"), "data")
	defer func() {
		os.Remove("data")
		if originalDirExists {
			os.Rename(originalDataDir+"_backup", originalDataDir)
		}
	}()

	fm := NewFileManager(false)
	data := []byte("benchmark test data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fileNum := i % 100
		filename := fmt.Sprintf("file_%d", fileNum)
		err := fm.AppendToFile(filename, data)
		if err != nil {
			b.Fatalf("Failed to append to file: %v", err)
		}
	}
	b.StopTimer()

	fm.Cleanup()
}

func BenchmarkConcurrentAppends(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "filemanager_bench")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a symbolic link to redirect "data" to our temp directory
	originalDataDir := "data"
	var originalDirExists bool
	if _, err := os.Stat(originalDataDir); err == nil {
		originalDirExists = true
		os.Rename(originalDataDir, originalDataDir+"_backup")
	}

	os.Mkdir(filepath.Join(tempDir, "data"), 0755)
	os.Symlink(filepath.Join(tempDir, "data"), "data")
	defer func() {
		os.Remove("data")
		if originalDirExists {
			os.Rename(originalDataDir+"_backup", originalDataDir)
		}
	}()

	fm := NewFileManager(false)
	data := []byte("benchmark test data")

	// Number of concurrent goroutines
	numGoroutines := runtime.GOMAXPROCS(0) * 2

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for g := 0; g < numGoroutines; g++ {
			go func(id int) {
				defer wg.Done()
				fileNum := id % 100
				filename := fmt.Sprintf("file_%d", fileNum)
				err := fm.AppendToFile(filename, data)
				if err != nil {
					b.Errorf("Failed to append to file: %v", err)
				}
			}(g)
		}

		wg.Wait()
	}

	b.StopTimer()
	fm.Cleanup()
}
