package storage

import (
	"path/filepath"
	"testing"

	"github.com/young1lin/responses2chat/internal/models"
)

func TestConversationStore(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewConversationStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	t.Run("Store and Get", func(t *testing.T) {
		messages := []models.ChatMessage{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi!"},
		}

		err := store.Store("resp-test-123", messages)
		if err != nil {
			t.Fatalf("Failed to store: %v", err)
		}

		got, found := store.Get("resp-test-123")
		if !found {
			t.Fatal("Expected to find stored messages")
		}

		if len(got) != 2 {
			t.Fatalf("Expected 2 messages, got %d", len(got))
		}

		if got[0].Role != "user" || got[1].Role != "assistant" {
			t.Errorf("Unexpected message roles: %s, %s", got[0].Role, got[1].Role)
		}
	})

	t.Run("Get non-existent", func(t *testing.T) {
		_, found := store.Get("resp-nonexistent")
		if found {
			t.Error("Expected not to find non-existent message")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		store.Store("resp-to-delete", []models.ChatMessage{{Role: "user", Content: "test"}})

		_, found := store.Get("resp-to-delete")
		if !found {
			t.Fatal("Expected to find message before delete")
		}

		err := store.Delete("resp-to-delete")
		if err != nil {
			t.Fatalf("Failed to delete: %v", err)
		}

		_, found = store.Get("resp-to-delete")
		if found {
			t.Error("Expected not to find deleted message")
		}
	})

	t.Run("Multimodal content", func(t *testing.T) {
		messages := []models.ChatMessage{
			{
				Role: "user",
				Content: []models.ChatContentPart{
					{Type: "text", Text: "What's in this image?"},
					{Type: "image_url", ImageURL: struct {
						URL string `json:"url"`
					}{URL: "https://example.com/image.png"}},
				},
			},
		}

		err := store.Store("resp-multimodal", messages)
		if err != nil {
			t.Fatalf("Failed to store multimodal: %v", err)
		}

		got, found := store.Get("resp-multimodal")
		if !found {
			t.Fatal("Expected to find multimodal messages")
		}

		content, ok := got[0].Content.([]interface{})
		if !ok {
			t.Fatalf("Expected Content to be []interface{}, got %T", got[0].Content)
		}

		if len(content) != 2 {
			t.Errorf("Expected 2 content parts, got %d", len(content))
		}
	})

	t.Run("Tool calls", func(t *testing.T) {
		messages := []models.ChatMessage{
			{Role: "user", Content: "What's the weather?"},
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []models.ToolCall{
					{
						ID:   "call_123",
						Type: "function",
						Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{
							Name:      "get_weather",
							Arguments: `{"location": "Beijing"}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				Content:    `{"temperature": 25}`,
				ToolCallID: "call_123",
			},
		}

		err := store.Store("resp-tool-calls", messages)
		if err != nil {
			t.Fatalf("Failed to store tool calls: %v", err)
		}

		got, found := store.Get("resp-tool-calls")
		if !found {
			t.Fatal("Expected to find tool call messages")
		}

		if len(got) != 3 {
			t.Fatalf("Expected 3 messages, got %d", len(got))
		}

		if len(got[1].ToolCalls) != 1 {
			t.Errorf("Expected 1 tool call, got %d", len(got[1].ToolCalls))
		}

		if got[1].ToolCalls[0].Function.Name != "get_weather" {
			t.Errorf("Expected function name 'get_weather', got '%s'", got[1].ToolCalls[0].Function.Name)
		}
	})
}

func TestConversationStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "persist.db")

	// Store data
	store1, err := NewConversationStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store1: %v", err)
	}

	messages := []models.ChatMessage{
		{Role: "user", Content: "Test persistence"},
	}
	store1.Store("resp-persist-test", messages)
	store1.Close()

	// Verify persistence
	store2, err := NewConversationStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store2: %v", err)
	}
	defer store2.Close()

	got, found := store2.Get("resp-persist-test")
	if !found {
		t.Fatal("Expected to find persisted messages after reopening")
	}

	if got[0].Content != "Test persistence" {
		t.Errorf("Expected 'Test persistence', got '%v'", got[0].Content)
	}
}
