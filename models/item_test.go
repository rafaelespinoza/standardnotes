package models_test

import (
	"testing"

	"github.com/rafaelespinoza/standardfile/models"
)

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
