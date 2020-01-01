package itemsync

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/rafaelespinoza/standardfile/models"
)

const _MinConflictThreshold = 1 * time.Second

var (
	// errSyncConflict signals some kind of item conflict; usually when two
	// items have the same UUID but different updated at timestamps.
	errSyncConflict = errors.New("sync_conflict")
	// errUUIDConflict signals a UUID conflict, this might happen if a user
	// is importing data from another account.
	errUUIDConflict = errors.New("uuid_conflict")
)

// ItemConflict describes an item sync conflict. It's comprised of the Item and
// an error describing the conflict. Each implementation should also implement
// the json.Marshaler interface as per the client expectations.
type ItemConflict interface {
	Item() models.Item
	Conflict() error
	json.Marshaler
}

type uuidConflict struct {
	item models.Item
}

var _ ItemConflict = (*uuidConflict)(nil)

func (c *uuidConflict) Item() models.Item { return c.item }
func (c *uuidConflict) Conflict() error   { return errUUIDConflict }
func (c *uuidConflict) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"unsaved_item": c.Item(),
		"type":         c.Conflict().Error(),
	})
}

type syncConflict struct {
	item models.Item
}

var _ ItemConflict = (*syncConflict)(nil)

func (c *syncConflict) Item() models.Item { return c.item }
func (c *syncConflict) Conflict() error   { return errSyncConflict }
func (c *syncConflict) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"server_item": c.Item(),
		"type":        c.Conflict().Error(),
	})
}
