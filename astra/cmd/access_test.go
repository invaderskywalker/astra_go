package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckDirectoryAccess(t *testing.T) {
	root := t.TempDir()
	if err := checkDirectoryAccess(root); err != nil {
		t.Fatalf("empty directory should be readable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "marker.txt"), []byte("ok"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := checkDirectoryAccess(root); err != nil {
		t.Fatalf("directory with files should be readable: %v", err)
	}
}

func TestIsFilesystemPermissionError(t *testing.T) {
	if !isFilesystemPermissionError(os.ErrPermission) {
		t.Fatal("os.ErrPermission should be classified as a filesystem permission error")
	}
	if !isFilesystemPermissionError(errors.New("open workspace: operation not permitted")) {
		t.Fatal("operation not permitted should be classified as a filesystem permission error")
	}
	if isFilesystemPermissionError(errors.New("file does not exist")) {
		t.Fatal("unrelated errors must not be classified as permission errors")
	}
}
