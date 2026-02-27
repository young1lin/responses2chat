package storage

import (
	"encoding/json"

	"go.etcd.io/bbolt"
	"go.uber.org/zap"

	"github.com/young1lin/responses2chat/internal/models"
	"github.com/young1lin/responses2chat/pkg/logger"
)

var bucketName = []byte("conversations")

// ConversationStore provides persistent storage for conversation history using BBolt
type ConversationStore struct {
	db *bbolt.DB
}

// NewConversationStore creates a new conversation store with the given database path
func NewConversationStore(path string) (*ConversationStore, error) {
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}

	// Create bucket if not exists
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	logger.Info("conversation store initialized", zap.String("path", path))
	return &ConversationStore{db: db}, nil
}

// Store saves a conversation history with the given response ID
func (s *ConversationStore) Store(responseID string, messages []models.ChatMessage) error {
	data, err := json.Marshal(messages)
	if err != nil {
		return err
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketName)
		return b.Put([]byte(responseID), data)
	})
}

// Get retrieves a conversation history by response ID
// Returns the messages and true if found, nil and false otherwise
func (s *ConversationStore) Get(responseID string) ([]models.ChatMessage, bool) {
	var messages []models.ChatMessage

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketName)
		data := b.Get([]byte(responseID))
		if data == nil {
			return nil
		}
		return json.Unmarshal(data, &messages)
	})

	if err != nil || len(messages) == 0 {
		return nil, false
	}

	return messages, true
}

// Delete removes a conversation history by response ID
func (s *ConversationStore) Delete(responseID string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketName)
		return b.Delete([]byte(responseID))
	})
}

// Close closes the database connection
func (s *ConversationStore) Close() error {
	return s.db.Close()
}
