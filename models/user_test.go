package models_test

import (
	"testing"

	"github.com/rafaelespinoza/standardfile/db"
	"github.com/rafaelespinoza/standardfile/models"
)

func init() {
	db.Init(":memory:")
}

func TestUserLoadActiveItems(t *testing.T) {
	user := models.NewUser()
	user.Email = "foo@example.com"
	user.Password = "testpassword123"
	user.PwNonce = "stub_password_nonce"
	var err error
	if err = user.Create(); err != nil {
		t.Fatal(err)
	}
	initialItems := []models.Item{
		{UUID: "alfa", UserUUID: user.UUID, Content: "a", ContentType: "a", EncItemKey: "a", AuthHash: "a"},
		{UUID: "bravo", UserUUID: user.UUID, Content: "b", ContentType: "b", EncItemKey: "b", AuthHash: "b"},
	}
	for i, item := range initialItems {
		if err := item.Save(); err != nil {
			t.Fatalf("could not save existingItems[%d] during setup; %v", i, err)
		}
	}

	fetchedItems, err := user.LoadActiveItems()
	if err != nil {
		t.Error(err)
	}
	if len(fetchedItems) != len(initialItems) {
		t.Errorf(
			"wrong length for fetched items; got %d, expected %d",
			len(fetchedItems), len(initialItems),
		)
	}
}
