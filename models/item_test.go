package models_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/rafaelespinoza/standardfile/db"
	"github.com/rafaelespinoza/standardfile/models"
)

func init() {
	db.Init(":memory:")
}

func TestLoadItemByUUID(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		saved := &models.Item{
			UserUUID:    "1234",
			Content:     "alpha",
			ContentType: "alpha",
			EncItemKey:  "alpha",
			AuthHash:    "alpha",
			Deleted:     false,
		}
		if err := saved.Save(); err != nil {
			t.Fatalf("got expected error; %v", err)
		}

		loaded, err := models.LoadItemByUUID(saved.UUID)
		if err != nil {
			t.Errorf("did not expect error; got %v", err)
		}
		if !compareItems(t, loaded, saved, true) {
			t.Error("items not equal")
		}
	})

	t.Run("errors", func(t *testing.T) {
		if item, err := models.LoadItemByUUID(""); err == nil {
			t.Error("expected an error")
		} else if item != nil {
			t.Errorf("item should be nil")
		}

		if item, err := models.LoadItemByUUID("not-in-the-db"); err != sql.ErrNoRows {
			t.Errorf("expected %v; got %v", sql.ErrNoRows, err)
		} else if item != nil {
			t.Errorf("item should be nil")
		}
	})
}

func TestItemUpdate(t *testing.T) {
	item := &models.Item{
		UserUUID:    "1234",
		Content:     "alpha",
		ContentType: "alpha",
		EncItemKey:  "alpha",
		AuthHash:    "alpha",
		Deleted:     false,
	}
	if err := item.Save(); err != nil {
		t.Fatalf("unexpected setup error while saving item; %v", err)
	}

	item.Content = "bravo"
	item.ContentType = "bravo"
	item.EncItemKey = "bravo"
	item.AuthHash = "bravo"

	if err := item.Update(); err != nil {
		t.Errorf("unexpected error; %v", err)
	}
	expectedItem := &models.Item{
		UUID:        item.UUID,
		UserUUID:    item.UserUUID,
		Content:     "bravo",
		ContentType: "bravo",
		EncItemKey:  "bravo",
		AuthHash:    "bravo",
		Deleted:     false,
	}
	// the changed fields remain changed in-memory
	if !compareItems(t, item, expectedItem, false) {
		t.Error("items not equal")
	}
	// the changed fields are reflected in db.
	loadedItem, err := models.LoadItemByUUID(item.UUID)
	if err != nil {
		t.Errorf("did not expect error; got %v", err)
	}
	if !compareItems(t, loadedItem, expectedItem, false) {
		t.Error("items not equal")
	}
}

func TestItemDelete(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		item := &models.Item{
			UserUUID:    "1234",
			Content:     "alpha",
			ContentType: "alpha",
			EncItemKey:  "alpha",
			AuthHash:    "alpha",
			Deleted:     false,
		}
		if err := item.Save(); err != nil {
			t.Fatalf("unexpected setup error while saving item; %v", err)
		}
		item.Deleted = true // client marks an item for deletion during a sync.
		if err := item.Delete(); err != nil {
			t.Errorf("unexpected error; %v", err)
		}

		expectedItem := &models.Item{
			UUID:        item.UUID,
			UserUUID:    item.UserUUID,
			Content:     "",
			ContentType: "alpha",
			EncItemKey:  "",
			AuthHash:    "",
			Deleted:     true,
		}
		// the changed fields remain changed in-memory
		if !compareItems(t, item, expectedItem, false) {
			t.Error("items not equal")
		}

		// the changed fields are reflected in db.
		loadedItem, err := models.LoadItemByUUID(item.UUID)
		if err != nil {
			t.Errorf("did not expect error; got %v", err)
		}
		if !compareItems(t, loadedItem, expectedItem, false) {
			t.Error("items not equal")
		}
	})

	t.Run("errors", func(t *testing.T) {
		item := &models.Item{
			UserUUID:    "1234",
			Content:     "alpha",
			ContentType: "alpha",
			EncItemKey:  "alpha",
			AuthHash:    "alpha",
			Deleted:     false,
		}
		if err := item.Delete(); err == nil {
			t.Errorf("expected error; got %v", err)
		}
	})
}

func TestItemCopy(t *testing.T) {
	item := &models.Item{
		UserUUID:    "1234",
		Content:     "alpha",
		ContentType: "alpha",
		EncItemKey:  "alpha",
		AuthHash:    "alpha",
		Deleted:     false,
	}
	if err := item.Save(); err != nil {
		t.Fatalf("unexpected setup error while saving item; %v", err)
	}
	time.Sleep(100)
	dupe, err := item.Copy()
	if err != nil {
		t.Errorf("unexpected error; got %v", err)
	}
	if dupe.UUID == "" {
		t.Error("output should have a UUID")
	} else if dupe.UUID == item.UUID {
		t.Error("output should have different UUID than source")
	}
	if !item.UpdatedAt.Before(dupe.UpdatedAt) {
		t.Error("expected source to have earlier timestamp than output")
	}
}

func TestItemMergeProtected(t *testing.T) {
	const (
		itemUUID    = "1234"
		userUUID    = "5678"
		contentType = "alpha"
	)
	t.Run("ok", func(t *testing.T) {
		tests := []struct{ item, updates, expected *models.Item }{
			{
				item: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "alpha",
					ContentType: contentType,
					EncItemKey:  "alpha",
					AuthHash:    "alpha",
					Deleted:     false,
				},
				updates: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "bravo",
					ContentType: contentType,
					EncItemKey:  "bravo",
					AuthHash:    "bravo",
					Deleted:     true,
				},
				expected: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "bravo",
					ContentType: contentType,
					EncItemKey:  "bravo",
					AuthHash:    "bravo",
					Deleted:     true,
				},
			},
			{
				item: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "alpha",
					ContentType: contentType,
					EncItemKey:  "alpha",
					AuthHash:    "alpha",
					Deleted:     false,
				},
				updates: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "bravo",
					ContentType: contentType,
				},
				expected: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "bravo",
					ContentType: contentType,
					EncItemKey:  "alpha",
					AuthHash:    "alpha",
					Deleted:     false,
				},
			},
			{
				item: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "alpha",
					ContentType: contentType,
					EncItemKey:  "alpha",
					AuthHash:    "alpha",
					Deleted:     false,
				},
				updates: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					ContentType: contentType,
					EncItemKey:  "bravo",
				},
				expected: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "alpha",
					ContentType: contentType,
					EncItemKey:  "bravo",
					AuthHash:    "alpha",
					Deleted:     false,
				},
			},
			{
				item: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "alpha",
					ContentType: contentType,
					EncItemKey:  "alpha",
					AuthHash:    "alpha",
					Deleted:     false,
				},
				updates: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					ContentType: contentType,
					AuthHash:    "bravo",
				},
				expected: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "alpha",
					ContentType: contentType,
					EncItemKey:  "alpha",
					AuthHash:    "bravo",
					Deleted:     false,
				},
			},
			{
				item: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "alpha",
					ContentType: contentType,
					EncItemKey:  "alpha",
					AuthHash:    "alpha",
					Deleted:     false,
				},
				updates: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					ContentType: contentType,
					Deleted:     true,
				},
				expected: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "alpha",
					ContentType: contentType,
					EncItemKey:  "alpha",
					AuthHash:    "alpha",
					Deleted:     true,
				},
			},
		}

		for i, test := range tests {
			if err := test.item.MergeProtected(test.updates); err != nil {
				t.Errorf("test [%d]; unexpected error; %v", i, err)
			}

			if !compareItems(t, test.item, test.expected, false) {
				t.Errorf("test [%d], items not equal", i)
			}
		}
	})

	t.Run("errors", func(t *testing.T) {
		// here, the `expected` field is a deep copy of the `item` field.
		tests := []struct{ item, updates, expected *models.Item }{
			{
				item: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "alpha",
					ContentType: contentType,
					EncItemKey:  "alpha",
					AuthHash:    "alpha",
					Deleted:     false,
				},
				updates: &models.Item{
					UUID:        "bravo",
					UserUUID:    userUUID,
					Content:     "bravo",
					ContentType: contentType,
					EncItemKey:  "bravo",
					AuthHash:    "bravo",
					Deleted:     true,
				},
				expected: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "alpha",
					ContentType: contentType,
					EncItemKey:  "alpha",
					AuthHash:    "alpha",
					Deleted:     false,
				},
			},
			{
				item: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "alpha",
					ContentType: contentType,
					EncItemKey:  "alpha",
					AuthHash:    "alpha",
					Deleted:     false,
				},
				updates: &models.Item{
					UUID:        itemUUID,
					UserUUID:    "bravo",
					Content:     "bravo",
					ContentType: contentType,
					EncItemKey:  "bravo",
					AuthHash:    "bravo",
					Deleted:     true,
				},
				expected: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "alpha",
					ContentType: contentType,
					EncItemKey:  "alpha",
					AuthHash:    "alpha",
					Deleted:     false,
				},
			},
			{
				item: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "alpha",
					ContentType: contentType,
					EncItemKey:  "alpha",
					AuthHash:    "alpha",
					Deleted:     false,
				},
				updates: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "bravo",
					ContentType: "bravo",
					EncItemKey:  "bravo",
					AuthHash:    "bravo",
					Deleted:     true,
				},
				expected: &models.Item{
					UUID:        itemUUID,
					UserUUID:    userUUID,
					Content:     "alpha",
					ContentType: contentType,
					EncItemKey:  "alpha",
					AuthHash:    "alpha",
					Deleted:     false,
				},
			},
		}

		for i, test := range tests {
			if err := test.item.MergeProtected(test.updates); err == nil {
				t.Errorf("test [%d]; expected error; got %v", i, err)
			}

			if !compareItems(t, test.item, test.expected, false) {
				t.Errorf("test [%d], items not equal", i)
			}
		}
	})
}

func TestItemsDelete(t *testing.T) {
	tests := []struct {
		items    models.Items
		uuid     string
		expected models.Items
	}{
		{
			items:    make(models.Items, 0),
			uuid:     "foo",
			expected: make(models.Items, 0),
		},
		{
			items:    []models.Item{{UUID: "foo"}},
			uuid:     "bar",
			expected: []models.Item{{UUID: "foo"}},
		},
		{
			items:    []models.Item{{UUID: "alfa"}, {UUID: "bravo"}, {UUID: "charlie"}},
			uuid:     "alfa",
			expected: []models.Item{{UUID: "bravo"}, {UUID: "charlie"}},
		},
		{
			items:    []models.Item{{UUID: "alfa"}, {UUID: "bravo"}, {UUID: "charlie"}},
			uuid:     "bravo",
			expected: []models.Item{{UUID: "alfa"}, {UUID: "charlie"}},
		},
		{
			items:    []models.Item{{UUID: "alfa"}, {UUID: "bravo"}, {UUID: "charlie"}},
			uuid:     "charlie",
			expected: []models.Item{{UUID: "alfa"}, {UUID: "bravo"}},
		},
	}

	for i, test := range tests {
		test.items.Delete(test.uuid)
		if len(test.items) != len(test.expected) {
			t.Errorf(
				"test [%d]; wrong length; got %d, expected %d",
				i, len(test.items), len(test.expected),
			)
			continue
		}
		for j, item := range test.items {
			if item.UUID != test.expected[j].UUID {
				t.Errorf(
					"test [%d][%d]; got %q, expected %q",
					i, j, item.UUID, test.expected[j].UUID,
				)
			}
		}
	}
}

func compareItems(t *testing.T, a, b *models.Item, checkTimestamps bool) (ok bool) {
	t.Helper()
	ok = true
	if a == nil && b != nil {
		t.Errorf("a is nil, but b is not nil")
		ok = false
	} else if a != nil && b == nil {
		t.Errorf("a not nil, but b is nil")
		ok = false
	} else if a == nil && b == nil {
		t.Logf("both items are nil, probably not what you want?")
		ok = false
	}
	if !ok {
		return
	}

	if a.UUID != b.UUID {
		t.Errorf("UUID different; a: %q, b: %q", a.UUID, b.UUID)
		ok = false
	}
	if a.UserUUID != b.UserUUID {
		t.Errorf("UserUUID different; a: %q, b: %q", a.UserUUID, b.UserUUID)
		ok = false
	}
	if a.Content != b.Content {
		t.Errorf("Content different; a: %q, b: %q", a.Content, b.Content)
		ok = false
	}
	if a.ContentType != b.ContentType {
		t.Errorf("ContentType different; a: %q, b: %q", a.ContentType, b.ContentType)
		ok = false
	}
	if a.EncItemKey != b.EncItemKey {
		t.Errorf("EncItemKey different; a: %q, b: %q", a.EncItemKey, b.EncItemKey)
		ok = false
	}
	if a.AuthHash != b.AuthHash {
		t.Errorf("AuthHash different; a: %q, b: %q", a.AuthHash, b.AuthHash)
		ok = false
	}
	if a.Deleted != b.Deleted {
		t.Errorf("Deleted different; a: %t, b: %t", a.Deleted, b.Deleted)
		ok = false
	}
	if checkTimestamps && !a.CreatedAt.Equal(b.CreatedAt) {
		t.Errorf("CreatedAt different;\na: %v\nb: %v", a.CreatedAt, b.CreatedAt)
		ok = false
	}
	if checkTimestamps && !a.UpdatedAt.Equal(b.UpdatedAt) {
		t.Errorf("UpdatedAt different;\na: %v\nb: %v", a.UpdatedAt, b.UpdatedAt)
		ok = false
	}
	return
}
