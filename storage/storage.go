package storage

import "context"

type Storage interface {
	ReadItem(ctx context.Context, userID, itemName string) ([]byte, error)
	WriteItem(ctx context.Context, userID, itemName string, content []byte) error
	DeleteItem(ctx context.Context, userID, itemName string) error
	DeleteUser(ctx context.Context, userID string) error
}
