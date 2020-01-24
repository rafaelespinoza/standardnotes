package itemsync

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/rafaelespinoza/standardnotes/internal/models"
)

func TestItemConflict(t *testing.T) {
	type TestCase struct {
		inputItem    models.Item
		expectedKeys []string // put common keys first, put different keys after.
		expectedType string
	}
	tests := map[error]TestCase{
		errUUIDConflict: {
			inputItem: models.Item{
				UUID:      "foo",
				CreatedAt: time.Now().UTC().Add(-time.Hour),
				UpdatedAt: time.Now().UTC().Add(-time.Minute),
			},
			expectedKeys: []string{"type", "unsaved_item"},
			expectedType: "uuid_conflict",
		},
		errSyncConflict: {
			inputItem: models.Item{
				UUID:      "foo",
				CreatedAt: time.Now().UTC().Add(-time.Hour),
				UpdatedAt: time.Now().UTC().Add(-time.Minute),
			},
			expectedKeys: []string{"type", "server_item"},
			expectedType: "sync_conflict",
		},
	}
	// test the JSON shape, which entails testing the other interface methods.
	testItemConflict := func(t *testing.T, conflict ItemConflict, test TestCase) (ok bool) {
		t.Helper()
		ok = true
		out, err := json.Marshal(conflict)
		if err != nil {
			t.Fatal(err)
			ok = false
			return
		}
		var unmarshaledOut map[string]interface{}
		if err = json.Unmarshal(out, &unmarshaledOut); err != nil {
			t.Fatal(err)
			ok = false
			return
		}
		var keys []string
		for key := range unmarshaledOut {
			keys = append(keys, key)
		}
		if len(keys) != len(test.expectedKeys) {
			t.Errorf(
				"wrong number of keys in unmarshaled JSON; got %d, expected %d",
				len(keys), len(test.expectedKeys),
			)
			ok = false
		}
		if unmarshaledOut["type"] != test.expectedType {
			t.Errorf(
				"wrong value for key, %q; got %v, expected %v",
				"type", unmarshaledOut["type"], test.expectedType,
			)
			ok = false
		}
		var unmarshaledItem models.Item
		in, ok := unmarshaledOut[test.expectedKeys[1]].(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected type %T at %q", in, test.expectedKeys[1])
			return
		}
		if unmarshaledItem, err = itemFromJSON(in); err != nil {
			t.Fatalf("error unmarshaling item; %v", err)
			ok = false
		}
		if ok := compareItems(t, &unmarshaledItem, &test.inputItem, true); !ok {
			t.Errorf("actual did not equal expected")
			ok = false
		}
		return
	}

	t.Run("uuid_conflict", func(t *testing.T) {
		test := tests[errUUIDConflict]
		if ok := testItemConflict(t, &uuidConflict{item: test.inputItem}, test); !ok {
			t.Error("item conflict incorrect")
		}
	})
	t.Run("sync_conflict", func(t *testing.T) {
		test := tests[errSyncConflict]
		if ok := testItemConflict(t, &syncConflict{item: test.inputItem}, test); !ok {
			t.Error("item conflict incorrect")
		}
	})
}

func itemFromJSON(in map[string]interface{}) (out models.Item, err error) {
	var ok bool
	if out.UUID, ok = in["uuid"].(string); !ok {
		// no op
	}
	if out.UserUUID, ok = in["user_uuid"].(string); !ok {
		// no op
	}
	if out.Content, ok = in["content"].(string); !ok {
		// no op
	}
	if out.ContentType, ok = in["content_type"].(string); !ok {
		// no op
	}
	if out.EncItemKey, ok = in["enc_item_key"].(string); !ok {
		// return
	}
	if out.AuthHash, ok = in["auth_hash"].(string); !ok {
		// no op
	}
	if out.Deleted, ok = in["deleted"].(bool); !ok {
		// no op
	}
	if timestamp, ok := in["created_at"].(string); !ok {
		err = fmt.Errorf("expected %q to be string", "created_at")
		return
	} else if out.CreatedAt, err = time.Parse(time.RFC3339, timestamp); err != nil {
		// no op
	}
	if timestamp, ok := in["updated_at"].(string); !ok {
		err = fmt.Errorf("expected %q to be string", "updated_at")
		return
	} else if out.UpdatedAt, err = time.Parse(time.RFC3339, timestamp); err != nil {
		// no op
	}
	return
}
