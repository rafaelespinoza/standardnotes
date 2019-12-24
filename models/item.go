package models

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rafaelespinoza/standardfile/logger"

	// "github.com/kisielk/sqlstruct"
	"github.com/google/uuid"
	"github.com/rafaelespinoza/standardfile/db"
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

// Save either adds a new Item to the DB or updates an existing Item in the DB.
func (i *Item) Save() error {
	if i.UUID == "" {
		return i.Create()
	}
	if exists, err := i.Exists(); err != nil {
		return err
	} else if !exists {
		return i.Create()
	}
	return i.Update()
}

func (i *Item) Create() error {
	if i.UUID == "" {
		id := uuid.New()
		i.UUID = uuid.Must(id, nil).String()
	}
	i.CreatedAt = time.Now()
	i.UpdatedAt = time.Now()
	logger.LogIfDebug("Create:", i.UUID)
	return db.Query(`
		INSERT INTO 'items' (
			'uuid', 'user_uuid', content, content_type, enc_item_key, auth_hash, deleted, created_at, updated_at
		) VALUES(?,?,?,?,?,?,?,?,?)`,
		i.UUID, i.UserUUID, i.Content, i.ContentType, i.EncItemKey, i.AuthHash, i.Deleted, i.CreatedAt, i.UpdatedAt,
	)
}

func (i *Item) Update() error {
	i.UpdatedAt = time.Now()
	logger.LogIfDebug("Update:", i.UUID)
	return db.Query(`
		UPDATE 'items'
		SET 'content'=?, 'enc_item_key'=?, 'auth_hash'=?, 'deleted'=?, 'updated_at'=?
		WHERE 'uuid'=? AND 'user_uuid'=?`,
		i.Content, i.EncItemKey, i.AuthHash, i.Deleted, i.UpdatedAt,
		i.UUID, i.UserUUID,
	)
}

func (i *Item) Delete() error {
	if i.UUID == "" {
		return fmt.Errorf("attempted to delete non-existent item")
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

func (i Item) Copy() (Item, error) {
	out := uuid.New()
	i.UUID = uuid.Must(out, nil).String()
	i.UpdatedAt = time.Now()
	err := i.Create()
	if err != nil {
		logger.LogIfDebug(err)
		return Item{}, err
	}
	return i, nil
}

// Exists checks if an item exists in the DB.
func (i *Item) Exists() (exists bool, err error) {
	if i.UUID == "" {
		return
	}
	uuid, err := db.SelectFirst("SELECT 'uuid' FROM 'items' WHERE 'uuid'=?", i.UUID)
	if err != nil {
		return
	}
	exists = uuid != ""
	return
}

// LoadByUUID populates the Item's fields by querying the DB.
func (i *Item) LoadByUUID(uuid string) (err error) {
	_, err = db.SelectStruct("SELECT * FROM 'items' WHERE 'uuid'=?", i, uuid)
	return
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
		logger.LogIfDebug(err)
		return time.Now()
	}
	parts := strings.Split(string(decoded), ":")
	str, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		logger.LogIfDebug(err)
		return time.Now()
	}
	// TODO: output "version" 1, 2 differently. See
	// `lib/sync_engine/abstract/sync_manager.rb` in the ruby sync-server
	return time.Time(time.Unix(0, int64(str)))
}

// Items is a collection of Item values.
type Items []Item

func (items *Items) Delete(uuid string) {
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
