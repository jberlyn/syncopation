package storage

import (
	"context"
	"os"
	"path/filepath"
)

type LocalFS struct {
	DataDir string
}

func NewLocalFS(dataDir string) *LocalFS {
	return &LocalFS{DataDir: dataDir}
}

func (fs *LocalFS) userDir(userID string) string {
	return filepath.Join(fs.DataDir, userID)
}

func (fs *LocalFS) itemPath(userID, itemName string) string {
	return filepath.Join(fs.userDir(userID), itemName)
}

func (fs *LocalFS) ReadItem(ctx context.Context, userID, itemName string) ([]byte, error) {
	return os.ReadFile(fs.itemPath(userID, itemName))
}

func (fs *LocalFS) WriteItem(ctx context.Context, userID, itemName string, content []byte) error {
	dir := fs.userDir(userID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(fs.itemPath(userID, itemName), content, 0644)
}

func (fs *LocalFS) DeleteItem(ctx context.Context, userID, itemName string) error {
	err := os.Remove(fs.itemPath(userID, itemName))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
