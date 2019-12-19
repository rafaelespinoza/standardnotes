package models

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set"
	"github.com/rafaelespinoza/standardfile/logger"

	// "github.com/kisielk/sqlstruct"
	"github.com/rafaelespinoza/standardfile/db"
	uuid "github.com/satori/go.uuid"
)

// Item - is an item type
type Item struct {
	UUID        string    `json:"uuid"`
	UserUUID    string    `json:"user_uuid"    sql:"user_uuid"`
	Content     string    `json:"content"`
	ContentType string    `json:"content_type" sql:"content_type"`
	EncItemKey  string    `json:"enc_item_key" sql:"enc_item_key"`
	AuthHash    string    `json:"auth_hash"    sql:"auth_hash"`
	Deleted     bool      `json:"deleted"`
	CreatedAt   time.Time `json:"created_at" sql:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" sql:"updated_at"`
}

type it interface {
	create() error
	update() error
	delete() error
}

//Items - is an items slice
type Items []Item

//SyncRequest - type for incoming sync request
type SyncRequest struct {
	Items            Items  `json:"items"`
	SyncToken        string `json:"sync_token"`
	CursorToken      string `json:"cursor_token"`
	Limit            int    `json:"limit"`
	ComputeIntegrity bool   `json:"compute_integrity"`
}

type Unsaved struct {
	Item
	error
}

//SyncResponse - type for response
type SyncResponse struct {
	Retrieved     Items     `json:"retrieved_items"`
	Saved         Items     `json:"saved_items"`
	Unsaved       []Unsaved `json:"unsaved"`
	SyncToken     string    `json:"sync_token"`
	CursorToken   string    `json:"cursor_token,omitempty"`
	IntegrityHash string    `json:"integrity_hash"`
}

const minConflictInterval = 20.0

//LoadValue - hydrate struct from map
func (r *SyncRequest) LoadValue(name string, value []string) {
	switch name {
	case "items":
		r.Items = Items{}
	case "sync_token":
		r.SyncToken = value[0]
	case "cursor_token":
		r.CursorToken = value[0]
	case "limit":
		r.Limit, _ = strconv.Atoi(value[0])
	}
}

//LoadValue - hydrate struct from map
func (i *Item) LoadValue(name string, value []string) {
	switch name {
	case "uuid":
		i.UUID = value[0]
	case "user_uuid":
		i.UserUUID = value[0]
	case "content":
		i.Content = value[0]
	case "enc_item_key":
		i.EncItemKey = value[0]
	case "content_type":
		i.ContentType = value[0]
	case "auth_hash":
		i.ContentType = value[0]
	case "deleted":
		i.Deleted = (value[0] == "true")
	}
}

//Save - save current item into DB
func (i *Item) save() error {
	if i.UUID == "" || !i.Exists() {
		return i.create()
	}
	return i.update()
}

func (i *Item) create() error {
	if i.UUID == "" {
		id := uuid.NewV4()
		i.UUID = uuid.Must(id, nil).String()
	}
	i.CreatedAt = time.Now()
	i.UpdatedAt = time.Now()
	logger.Log("Create:", i.UUID)
	return db.Query(`
		INSERT INTO 'items' (
			'uuid', 'user_uuid', content, content_type, enc_item_key, auth_hash, deleted, created_at, updated_at
		) VALUES(?,?,?,?,?,?,?,?,?)`,
		i.UUID, i.UserUUID, i.Content, i.ContentType, i.EncItemKey, i.AuthHash, i.Deleted, i.CreatedAt, i.UpdatedAt,
	)
}

func (i *Item) update() error {
	i.UpdatedAt = time.Now()
	logger.Log("Update:", i.UUID)
	return db.Query(`
		UPDATE 'items'
		SET 'content'=?, 'enc_item_key'=?, 'auth_hash'=?, 'deleted'=?, 'updated_at'=?
		WHERE 'uuid'=? AND 'user_uuid'=?`,
		i.Content, i.EncItemKey, i.AuthHash, i.Deleted, i.UpdatedAt,
		i.UUID, i.UserUUID,
	)
}

func (i *Item) delete() error {
	if i.UUID == "" {
		return fmt.Errorf("Trying to delete unexisting item")
	}
	i.Content = ""
	i.EncItemKey = ""
	i.AuthHash = ""
	i.UpdatedAt = time.Now()

	return db.Query(`
		UPDATE 'items'
		SET 'content'='', 'enc_item_key'='', 'auth_hash'='','deleted'=1, 'updated_at'=?
		WHERE 'uuid'=? AND 'user_uuid'=?`,
		i.UpdatedAt, i.UUID, i.UserUUID,
	)
}

func (i Item) copy() (Item, error) {
	out := uuid.NewV4()
	i.UUID = uuid.Must(out, nil).String()
	i.UpdatedAt = time.Now()
	err := i.create()
	if err != nil {
		logger.Log(err)
		return Item{}, err
	}
	return i, nil
}

//Exists - checks if current user exists in DB
func (i Item) Exists() bool {
	if i.UUID == "" {
		return false
	}
	uuid, err := db.SelectFirst("SELECT 'uuid' FROM 'items' WHERE 'uuid'=?", i.UUID)

	if err != nil {
		logger.Log(err)
		return false
	}
	logger.Log("Exists:", uuid)
	return uuid != ""
}

//LoadByUUID - loads item info from DB
func (i *Item) LoadByUUID(uuid string) bool {
	_, err := db.SelectStruct("SELECT * FROM 'items' WHERE 'uuid'=?", i, uuid)

	if err != nil {
		logger.Log(err)
		return false
	}

	return true
}

type Frequency uint8

const (
	frequencyNever Frequency = iota
	FrequencyRealtime
	FrequencyHourly
	FrequencyDaily
)

type ContentMetadata struct {
	Frequency Frequency // hourly, daily, weekly, monthly
	SubType   string    // backup.email_archive
	URL       string
}

// TODO: implement, should return the metadata only.
func (i *Item) DecodedContentMetadata() (out *ContentMetadata) {
	if i.Content == "" {
		return
	}
	return
}

func (i *Item) IsDailyBackupExtension() bool {
	if i.ContentType != "SF|Extension" {
		return false
	}
	content := i.DecodedContentMetadata()
	return content != nil && content.Frequency == FrequencyDaily
}

// GetTokenFromTime generates sync token for current time. TODO: rename to TokenizeTime
func GetTokenFromTime(date time.Time) string {
	return base64.URLEncoding.EncodeToString(
		[]byte(
			fmt.Sprintf(
				"1:%d", // TODO: make use of "version" 1 and 2. (part before :)
				date.UnixNano(),
			),
		),
	)
}

// GetTimeFromToken - retrieve datetime from sync token
func GetTimeFromToken(token string) time.Time {
	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		logger.Log(err)
		return time.Now()
	}
	parts := strings.Split(string(decoded), ":")
	str, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		logger.Log(err)
		return time.Now()
	}
	// TODO: output "version" 1, 2 differently. See
	// `lib/sync_engine/abstract/sync_manager.rb` in the ruby sync-server
	return time.Time(time.Unix(0, int64(str)))
}

func (items Items) CheckForConflicts(existing *Items) {
	logger.Log("Saved len:", len(items))
	logger.Log("Retrieved len:", len(*existing))
	saved := mapset.NewSet()
	for _, item := range items {
		saved.Add(item.UUID)
	}
	retrieved := mapset.NewSet()
	for _, item := range *existing {
		retrieved.Add(item.UUID)
	}
	conflicts := saved.Intersect(retrieved)
	logger.Log("Conflicts", conflicts)
	// saved items take precedence, retrieved items are duplicated with a new uuid
	for _, uuid := range conflicts.ToSlice() {
		// if changes are greater than minConflictInterval seconds apart, create conflicted copy, otherwise discard conflicted
		logger.Log(uuid)
		savedCopy := items.find(uuid.(string))
		retrievedCopy := existing.find(uuid.(string))

		if savedCopy.isConflictedWith(retrievedCopy) {
			log.Printf("Creating conflicted copy of %v\n", uuid)
			dupe, err := retrievedCopy.copy()
			if err != nil {
				logger.Log(err)
			} else {
				*existing = append(*existing, dupe)
			}
		}
		existing.delete(uuid.(string))
	}
}

func (i Item) isConflictedWith(copy Item) bool {
	diff := math.Abs(float64(i.UpdatedAt.Unix() - copy.UpdatedAt.Unix()))
	logger.Log("Conflict diff, min interval:", diff, minConflictInterval)
	return diff > minConflictInterval
}

func (items Items) Save(userUUID string) (Items, []Unsaved, error) {
	savedItems := Items{}
	unsavedItems := []Unsaved{}

	if len(items) == 0 {
		return savedItems, unsavedItems, nil
	}

	for _, item := range items {
		var err error
		item.UserUUID = userUUID
		if item.Deleted {
			err = item.delete()
		} else {
			err = item.save()
		}
		if err != nil {
			unsavedItems = append(unsavedItems, Unsaved{item, err})
			logger.Log("Unsaved:", item)
		} else {
			item.load() //reloading item info from DB
			savedItems = append(savedItems, item)
			logger.Log("Saved:", item)
		}
	}
	return savedItems, unsavedItems, nil
}

func (i *Item) load() bool {
	return i.LoadByUUID(i.UUID)
}

func (u User) LoadItems(request SyncRequest) (items Items, cursorTime time.Time, err error) {
	if request.CursorToken != "" {
		logger.Log("loadItemsFromDate")
		items, err = u.loadItemsFromDate(GetTimeFromToken(request.CursorToken))
	} else if request.SyncToken != "" {
		logger.Log("loadItemsOlder")
		items, err = u.loadItemsOlder(GetTimeFromToken(request.SyncToken))
	} else {
		logger.Log("loadItems")
		items, err = u.loadAllItems(request.Limit)
		if len(items) > 0 {
			cursorTime = items[len(items)-1].UpdatedAt
		}
	}
	return items, cursorTime, err
}

func (u User) loadItemsFromDate(date time.Time) ([]Item, error) {
	items := []Item{}
	err := db.Select(`
		SELECT *
		FROM 'items'
		WHERE 'user_uuid'=? AND 'updated_at' >= ?
		ORDER BY 'updated_at' DESC`,
		&items, u.UUID, date,
	)
	return items, err
}

func (u User) loadItemsOlder(date time.Time) ([]Item, error) {
	items := []Item{}
	err := db.Select(`
		SELECT *
		FROM 'items'
		WHERE 'user_uuid'=? AND 'updated_at' > ?
		ORDER BY 'updated_at' DESC`,
		&items, u.UUID, date,
	)
	return items, err
}

func (u User) loadAllItems(limit int) ([]Item, error) {
	items := []Item{}
	err := db.Select(
		"SELECT * FROM 'items' WHERE 'user_uuid'=? ORDER BY 'updated_at' DESC",
		&items, u.UUID,
	)
	return items, err
}

func (u User) LoadActiveItems() (items []Item, err error) {
	err = db.Select(`
		SELECT * FROM 'items'
		WHERE 'user_uuid'=? AND 'content_type' IS NOT '' AND deleted = ?
		ORDER BY 'updated_at' DESC`,
		&items,
		u.UUID, "SF|Extension", false,
	)
	return
}

func (u User) LoadActiveExtensionItems() (items []Item, err error) {
	err = db.Select(`
		SELECT * FROM 'items'
		WHERE 'user_uuid'=? AND 'content_type' = ? AND deleted = ?
		ORDER BY 'updated_at' DESC`,
		&items,
		u.UUID, "SF|Extension", false,
	)
	return
}

func (items Items) find(uuid string) Item {
	for _, item := range items {
		if item.UUID == uuid {
			return item
		}
	}
	return Item{}
}

func (items *Items) delete(uuid string) {
	position := 0
	for i, item := range *items {
		if item.UUID == uuid {
			position = i
			break
		}
	}
	(*items) = (*items)[:position:position]
}

func (items Items) ComputeHashDigest() string {
	timestamps := make([]string, len(items))
	for i, item := range items {
		timestamps[i] = strconv.FormatInt(item.UpdatedAt.Unix(), 10)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(timestamps)))
	input := strings.Join(timestamps, ",")
	return fmt.Sprintf("%x", sha256.Sum256([]byte(input)))
}

// computeHashDigestAlt differs from the other one in how it sorts. This one
// sorts by time values, which are int64, while the other sorts the int64 after
// it's been casted to a string.
func (items Items) computeHashDigestAlt() string {
	timestamps := make([]time.Time, len(items))
	for i, item := range items {
		timestamps[i] = item.UpdatedAt
	}
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[j].Before(timestamps[i])
	})
	var buf bytes.Buffer
	var i int
	for ; i < len(timestamps)-1; i++ {
		buf.Write([]byte(strconv.FormatInt(timestamps[i].Unix(), 10) + ","))
	}
	buf.Write([]byte(strconv.FormatInt(timestamps[i].Unix(), 10)))
	return fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))
}
