package itemsync

import (
	"os"
	"testing"
	"time"

	"github.com/rafaelespinoza/standardfile/db"
	"github.com/rafaelespinoza/standardfile/models"
)

const (
	testDBName        = "standardfile_test"
	baseTestOutputDir = "/tmp/standardfile_test"
)

func TestMain(m *testing.M) {
	os.MkdirAll(baseTestOutputDir, 0755)
	m.Run()
	os.RemoveAll(baseTestOutputDir)
}

func TestSyncUserItems(t *testing.T) {
	t.Skip()
	// TODO: change job queueing stuff to an interface and just check that
	// they're called. The most important stuff happens before the job queueing.
}

func TestDoItemSync(t *testing.T) {
	// NOTE: I don't think testing the user load item cases is that important
	// right now because it's not clear why a cursor token, a sync token or no
	// token is there. The queries are not fully-developed anyways (ignores
	// content type and limit inputs), so just test with all user items for now.

	pathToTestDir := baseTestOutputDir + "/" + t.Name()
	if err := os.MkdirAll(pathToTestDir, 0755); err != nil {
		t.Fatalf("error creating test directory %s", pathToTestDir)
		return
	}
	defer os.RemoveAll(pathToTestDir)
	db.Init(pathToTestDir + "/" + testDBName)

	user := models.User{UUID: t.Name()}
	unchangedItem := makeItem(t.Name()+"/unchanged", user.UUID)
	itemToChange := makeItem(t.Name()+"/change", user.UUID)
	itemWithSyncConflict := makeItem(t.Name()+"/sync_conflict", user.UUID)
	itemToMarkDeleted := makeItem(t.Name()+"/deleted", user.UUID)
	existingItems := []models.Item{
		unchangedItem,
		itemToChange,
		itemWithSyncConflict,
		itemToMarkDeleted,
	}
	for i, item := range existingItems {
		if err := item.Save(); err != nil {
			t.Fatalf("could not save existingItems[%d] during setup; %v", i, err)
			return
		}
	}

	// simulate item updates or staleness from client
	itemToChange.Content = "bravo"
	itemWithSyncConflict.UpdatedAt = time.Now().Add(time.Hour * -1)
	itemWithSyncConflict.CreatedAt = time.Now().Add(time.Hour * -2)
	itemToMarkDeleted.Deleted = true
	newItem := makeItem(t.Name()+"/new_item", user.UUID)
	incomingItems := []models.Item{
		itemToChange,
		itemWithSyncConflict,
		itemToMarkDeleted,
		newItem,
	}
	res := &Response{}
	if err := res.doItemSync(user, Request{Items: incomingItems}); err != nil {
		t.Errorf("did not expect error; got %v", err)
	}

	// test Retrieved field
	expectedRetrievedUUIDs := []string{itemToChange.UUID, unchangedItem.UUID, itemToMarkDeleted.UUID}
	if len(res.Retrieved) != len(expectedRetrievedUUIDs) {
		t.Errorf(
			"unexpected length for Retrieved; got %d, expected %d",
			len(res.Retrieved), len(expectedRetrievedUUIDs),
		)
	}
	for i, item := range res.Retrieved {
		if found := member(expectedRetrievedUUIDs, item.UUID); !found {
			t.Errorf("could not find item at Retrieved[%d]", i)
		}
	}

	// test Saved field
	expectedSavedUUIDs := []string{itemToChange.UUID, itemToMarkDeleted.UUID, newItem.UUID}
	if len(res.Saved) != len(expectedSavedUUIDs) {
		t.Errorf(
			"unexpected length for Saved; got %d, expected %d",
			len(res.Saved), len(expectedSavedUUIDs),
		)
	}
	for i, item := range res.Saved {
		if found := member(expectedSavedUUIDs, item.UUID); !found {
			t.Errorf("could not find item at Saved[%d]", i)
		}
	}

	// test Conflicts field
	expectedConflictsUUIDs := []string{itemWithSyncConflict.UUID}
	if len(res.Conflicts) != len(expectedConflictsUUIDs) {
		t.Errorf(
			"unexpected length for Conflicts; got %d, expected %d",
			len(res.Conflicts), len(expectedConflictsUUIDs),
		)
	}
	for i, conflict := range res.Conflicts {
		if found := member(expectedConflictsUUIDs, conflict.Item().UUID); !found {
			t.Errorf("could not find item at Conflicts[%d]", i)
		}
	}

	var err error

	// test Update behavior
	var updatedItemToChange *models.Item
	if updatedItemToChange, err = models.LoadItemByUUID(itemToChange.UUID); err != nil {
		t.Fatalf("did not expect error fetching 'changed' item; got %v", err)
	}
	if updatedItemToChange.Content != "bravo" {
		t.Errorf(
			"did not update item Content; got %q, expected %q",
			updatedItemToChange.Content, "bravo",
		)
	}

	// test Delete behavior
	var updatedDeletedItem *models.Item
	if updatedDeletedItem, err = models.LoadItemByUUID(itemToMarkDeleted.UUID); err != nil {
		t.Fatalf("did not expect error fetching 'deleted' item; got %v", err)
	}
	if !updatedDeletedItem.Deleted {
		t.Error("expected deleted item Deleted to be true")
	}
	if updatedDeletedItem.Content != "" {
		t.Error("expected deleted item Content to be empty")
	}
	if updatedDeletedItem.EncItemKey != "" {
		t.Error("expected deleted item EncItemKey to be empty")
	}
	if updatedDeletedItem.AuthHash != "" {
		t.Error("expected deleted item AuthHash to be empty")
	}

	// TODO: check error handling from checkItemConflicts
}

func TestFindCheckItem(t *testing.T) {
	pathToTestDir := baseTestOutputDir + "/" + t.Name()
	if err := os.MkdirAll(pathToTestDir, 0755); err != nil {
		t.Fatalf("error creating test directory %s", pathToTestDir)
		return
	}
	defer os.RemoveAll(pathToTestDir)
	db.Init(pathToTestDir + "/" + testDBName)

	t.Run("item does not exist in DB", func(t *testing.T) {
		incomingItem := makeItem("alpha", "alpha")
		if item, err := findCheckItem(incomingItem); err != nil {
			t.Errorf("did not expect error, got %v", err)
		} else if *item != incomingItem {
			t.Errorf("output item did not equal expected item")
		}
	})

	t.Run("item exists in DB", func(t *testing.T) {
		t.Run("same timestamps", func(t *testing.T) {
			existingItem := makeItem(t.Name(), "")
			if err := existingItem.Save(); err != nil {
				t.Fatal(err)
			}
			incomingItem := makeItem(t.Name(), "")
			incomingItem.UpdatedAt = existingItem.UpdatedAt
			incomingItem.CreatedAt = existingItem.CreatedAt

			if item, err := findCheckItem(incomingItem); err != nil {
				t.Errorf("did not expect error, got %v", err)
			} else if ok := compareItems(t, item, &existingItem, true); !ok {
				t.Errorf("actual did not equal expected")
			}
		})

		t.Run("incoming item is stale", func(t *testing.T) {
			existingItem := makeItem(t.Name(), "")
			if err := existingItem.Save(); err != nil {
				t.Fatal(err)
			}
			incomingItem := makeItem(t.Name(), "")
			incomingItem.UpdatedAt = existingItem.UpdatedAt.Add(time.Hour * -1)
			incomingItem.CreatedAt = existingItem.CreatedAt.Add(time.Hour * -2)

			item, err := findCheckItem(incomingItem)
			if err != ErrConflictingSync {
				t.Errorf("unexpected error, expected %v got %v", ErrConflictingSync, err)
			}
			if ok := compareItems(t, item, &existingItem, false); !ok {
				t.Errorf("actual did not equal expected")
			}
		})

		t.Run("db item is stale", func(t *testing.T) {
			existingItem := makeItem(t.Name(), "")
			existingItem.UUID = t.Name()
			if err := existingItem.Save(); err != nil {
				t.Fatal(err)
			}
			incomingItem := makeItem(t.Name(), "")
			incomingItem.UpdatedAt = existingItem.UpdatedAt.Add(time.Hour * 1)
			incomingItem.CreatedAt = existingItem.CreatedAt.Add(time.Hour * 2)

			item, err := findCheckItem(incomingItem)
			if err != ErrConflictingSync {
				t.Errorf("unexpected error, expected %v got %v", ErrConflictingSync, err)
			}
			if ok := compareItems(t, item, &existingItem, false); !ok {
				t.Errorf("actual did not equal expected")
			}
		})
	})
}

func makeItem(uuid, userUUID string) models.Item {
	return models.Item{
		UUID:        uuid,
		UserUUID:    userUUID,
		Content:     "alpha",
		ContentType: "alpha",
		EncItemKey:  "alpha",
		AuthHash:    "alpha",
		Deleted:     false,
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

func member(items []string, val string) (ok bool) {
	for _, it := range items {
		if it == val {
			ok = true
			return
		}
	}
	return
}
