package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalFS(t *testing.T) {
	tempDir := t.TempDir()
	fs := NewLocalFS(tempDir)
	ctx := context.Background()
	userID := "user123"
	itemName := "test.txt"

	// Test WriteItem
	content := []byte("hello storage")
	err := fs.WriteItem(ctx, userID, itemName, content)
	if err != nil {
		t.Fatalf("WriteItem failed: %v", err)
	}

	// Test ReadItem
	readContent, err := fs.ReadItem(ctx, userID, itemName)
	if err != nil {
		t.Fatalf("ReadItem failed: %v", err)
	}
	if string(readContent) != string(content) {
		t.Fatalf("Expected %s, got %s", string(content), string(readContent))
	}

	// Test DeleteItem
	err = fs.DeleteItem(ctx, userID, itemName)
	if err != nil {
		t.Fatalf("DeleteItem failed: %v", err)
	}

	// Test ReadItem after delete
	_, err = fs.ReadItem(ctx, userID, itemName)
	if err == nil {
		t.Fatalf("Expected error reading deleted item")
	}

	// Test DeleteItem for non-existent file
	err = fs.DeleteItem(ctx, userID, "non_existent.txt")
	if err != nil {
		t.Fatalf("DeleteItem for non-existent file should return nil, got: %v", err)
	}

	// Test WriteItem error (directory creation failure - using a file as dir)
	badDirFS := NewLocalFS(filepath.Join(tempDir, "file_as_dir"))
	_ = os.WriteFile(badDirFS.DataDir, []byte("not a dir"), 0644)
	err = badDirFS.WriteItem(ctx, userID, itemName, content)
	if err == nil {
		t.Fatalf("Expected error when MkdirAll fails")
	}

	// Test DeleteItem permission error (removing a directory instead of file)
	// We can't easily mock this in a cross-platform way, but MkdirAll failure covers the error paths mostly.
}
