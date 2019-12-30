package models_test

import (
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/rafaelespinoza/standardfile/db"
	"github.com/rafaelespinoza/standardfile/models"
)

func init() {
	db.Init(":memory:")
}

func TestLoadUser(t *testing.T) {
	type userLoader func() (*models.User, error)

	t.Run("ok", func(t *testing.T) {
		saved := models.NewUser()
		saved.Email = t.Name() + "@example.com"
		saved.Password = "testpassword123"
		saved.PwNonce = "stub_password_nonce"
		if err := saved.Create(); err != nil {
			t.Fatalf("unexpected error; %v", err)
		}

		tests := []userLoader{
			func() (*models.User, error) {
				return models.LoadUserByUUID(saved.UUID)
			},
			func() (*models.User, error) {
				return models.LoadUserByEmail(saved.Email)
			},
			func() (*models.User, error) {
				return models.LoadUserByEmailAndPassword(saved.Email, saved.Password)
			},
		}

		for i, loadUser := range tests {
			loaded, err := loadUser()
			if err != nil {
				t.Errorf("test [%d]; unexpected error; %v", i, err)
			}
			if !compareUsers(t, loaded, saved, true) {
				t.Errorf("test [%d]; users not equal", i)
			}
			if !loaded.PwHashState().Hashed {
				t.Errorf("test [%d]; password should be marked as hashed", i)
			}
		}
	})

	t.Run("errors", func(t *testing.T) {
		t.Run("empty inputs", func(t *testing.T) {
			tests := []userLoader{
				func() (*models.User, error) {
					return models.LoadUserByUUID("")
				},
				func() (*models.User, error) {
					return models.LoadUserByEmail("")
				},
				func() (*models.User, error) {
					return models.LoadUserByEmailAndPassword("foo@example.com", "")
				},
				func() (*models.User, error) {
					return models.LoadUserByEmailAndPassword("", "testpassword123")
				},
				func() (*models.User, error) {
					return models.LoadUserByEmailAndPassword("", "")
				},
			}
			// The exact error wording is not too important, but it should
			// mention that something is wrong with the input.
			errorMessagePattern := regexp.MustCompile(`(?i)empty|invalid|missing|require`)

			for i, loadUser := range tests {
				user, err := loadUser()
				if err == nil {
					t.Errorf("test [%d]; expected error", i)
				}
				if !errorMessagePattern.MatchString(err.Error()) {
					t.Errorf(
						"test [%d]; expected error message to match %q",
						i, errorMessagePattern,
					)
				}
				if user != nil {
					t.Errorf("test [%d]; user should be nil", i)
				}
			}
		})

		t.Run("not found", func(t *testing.T) {
			tests := []userLoader{
				func() (*models.User, error) {
					return models.LoadUserByUUID("not-in-the-db")
				},
			}

			for i, loadUser := range tests {
				if user, err := loadUser(); err != sql.ErrNoRows {
					t.Errorf("test [%d]; expected %v; got %v", i, sql.ErrNoRows, err)
				} else if user != nil {
					t.Errorf("test [%d]; user should be nil; %v", i, user)
				}
			}
		})
	})
}

func TestUserExists(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		user := models.NewUser()
		user.Email = t.Name() + "@example.com"
		user.Password = "testpassword123"
		user.PwNonce = "stub_password_nonce"
		if err := user.Create(); err != nil {
			t.Fatalf("unexpected error; %v", err)
		}
		exists, err := user.Exists()
		if err != nil {
			t.Errorf("unexpected error; got %v", err)
		}
		if !exists {
			t.Error("expected user to exist in db")
		}
	})

	t.Run("false", func(t *testing.T) {
		t.Run("invalid email", func(t *testing.T) {
			user := models.NewUser()
			exists, err := user.Exists()
			if err != nil {
				t.Errorf("unexpected error; got %v", err)
			}
			if exists {
				t.Error("user should not exist in db")
			}
		})

		t.Run("no uuid", func(t *testing.T) {
			user := models.NewUser()
			user.Email = t.Name() + "@example.com"
			exists, err := user.Exists()
			if err != nil {
				t.Errorf("unexpected error; got %v", err)
			}
			if exists {
				t.Error("user should not exist in db")
			}
		})

		t.Run("does not exist", func(t *testing.T) {
			user := models.NewUser()
			user.UUID = "1234"
			exists, err := user.Exists()
			if err != nil {
				t.Errorf("unexpected error; got %v", err)
			}
			if exists {
				t.Error("user should not exist in db")
			}
		})
	})
}

func TestUserMakeSaferCopy(t *testing.T) {
	user := &models.User{
		UUID:      "1234",
		Email:     t.Name() + "@example.com",
		Password:  "testpassword123",
		PwFunc:    "foo",
		PwAlg:     "bar",
		PwCost:    123,
		PwKeySize: 456,
		PwNonce:   "stub_password_nonce",
		PwSalt:    "stub_password_salt",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	dupe := user.MakeSaferCopy()
	expected := models.User{
		UUID:      "1234",
		Email:     t.Name() + "@example.com",
		Password:  "",
		PwFunc:    "foo",
		PwAlg:     "bar",
		PwCost:    123,
		PwKeySize: 456,
		PwNonce:   "",
		PwSalt:    "stub_password_salt",
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
	if !compareUsers(t, &dupe, &expected, true) {
		t.Error("users not equal")
	}
	if dupe.PwHashState().Hashed {
		t.Error("password should not be marked as hashed")
	}
}

func TestUserCreate(t *testing.T) {
	const plaintextPassword = "testpassword123"

	t.Run("ok", func(t *testing.T) {
		user := models.NewUser()
		user.Email = t.Name() + "@example.com"
		user.Password = plaintextPassword
		if err := user.Create(); err != nil {
			t.Error(err)
			return
		}
		if user.UUID == "" {
			t.Error("UUID should not be empty")
		}
		if user.Password != models.Hash(plaintextPassword) {
			t.Errorf(
				"should Hash password;\ngot: %q\nexp: %q",
				user.Password, models.Hash(plaintextPassword),
			)
		}
		if user.CreatedAt.IsZero() {
			t.Error("should set CreatedAt")
		}
		if !user.PwHashState().Hashed {
			t.Errorf("password should be marked as hashed")
		}
	})

	t.Run("errors", func(t *testing.T) {
		tests := []struct{ user *models.User }{
			// UUID must be empty
			{
				&models.User{
					UUID:     "1234",
					Email:    t.Name() + "uuid" + "@example.com",
					Password: "testpassword123",
				},
			},
			// email required
			{
				&models.User{
					Password: "testpassword123",
				},
			},
			// password required
			{
				&models.User{
					Email: t.Name() + "pw" + "@example.com",
				},
			},
		}
		for i, test := range tests {
			err := test.user.Create()
			if err == nil {
				t.Errorf("test [%d]; expected error", i)
			}
		}

		t.Run("already registered", func(t *testing.T) {
			email := t.Name() + "@example.com"
			existingUser := models.NewUser()
			existingUser.Email = email
			existingUser.Password = plaintextPassword
			if err := existingUser.Create(); err != nil {
				t.Fatal(err)
				return
			}

			user := models.NewUser()
			user.Email = email
			user.Password = plaintextPassword
			if err := user.Create(); err == nil {
				t.Error("expected error")
			}
			if user.UUID == "" {
				return
			}

			// further troubleshooting...
			t.Error("user UUID should be empty")
			if wtf, err := models.LoadUserByUUID(user.UUID); err != sql.ErrNoRows {
				t.Errorf("unexpected error; %v", err)
			} else {
				t.Errorf("did not expect to find user in db; got %v", wtf)
			}
		})
	})
}

func TestUserLoadActiveItems(t *testing.T) {
	user := models.NewUser()
	user.Email = t.Name() + "@example.com"
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

func compareUsers(t *testing.T, a, b *models.User, checkTimestamps bool) (ok bool) {
	t.Helper()
	ok = true
	if a == nil && b != nil {
		t.Errorf("a is nil, but b is not nil")
		ok = false
	} else if a != nil && b == nil {
		t.Errorf("a not nil, but b is nil")
		ok = false
	} else if a == nil && b == nil {
		t.Logf("both users are nil, probably not what you want?")
		ok = false
	}
	if !ok {
		return
	}

	if a.UUID != b.UUID {
		t.Errorf("UUID different; a: %q, b: %q", a.UUID, b.UUID)
		ok = false
	}
	if a.Email != b.Email {
		t.Errorf("Email different; a: %q, b: %q", a.Email, b.Email)
		ok = false
	}
	if a.Password != b.Password {
		t.Errorf("Password different; a: %q, b: %q", a.Password, b.Password)
		ok = false
	}
	if a.PwFunc != b.PwFunc {
		t.Errorf("PwFunc different; a: %q, b: %q", a.PwFunc, b.PwFunc)
		ok = false
	}
	if a.PwAlg != b.PwAlg {
		t.Errorf("PwAlg different; a: %q, b: %q", a.PwAlg, b.PwAlg)
		ok = false
	}
	if a.PwCost != b.PwCost {
		t.Errorf("PwCost different; a: %q, b: %q", a.PwCost, b.PwCost)
		ok = false
	}
	if a.PwKeySize != b.PwKeySize {
		t.Errorf("PwKeySize different; a: %d, b: %d", a.PwKeySize, b.PwKeySize)
		ok = false
	}
	if a.PwNonce != b.PwNonce {
		t.Errorf("PwNonce different; a: %q, b: %q", a.PwNonce, b.PwNonce)
		ok = false
	}
	if a.PwSalt != b.PwSalt {
		t.Errorf("PwSalt different; a: %q, b: %q", a.PwSalt, b.PwSalt)
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
