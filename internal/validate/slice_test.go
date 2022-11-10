package validate

import (
	"testing"

	"gotest.tools/v3/assert"
)

type StringSliceExample struct {
	Field []string
}

func (s StringSliceExample) ValidationRules() []ValidationRule {
	return []ValidationRule{
		&StringSliceRule{
			Value:     s.Field,
			Name:      "strField",
			MaxLength: 10,
			ItemRule: StringRule{
				Name:      "allowedDomains.values",
				MaxLength: 254,
				CharacterRanges: []CharRange{
					AlphabetLower,
					AlphabetUpper,
					Numbers,
					Dash,
					Dot,
					Underscore,
				},
				FirstCharacterRange: AlphaNumeric,
			},
		},
	}
}

func TestSliceRule_Validate(t *testing.T) {
	t.Run("max length", func(t *testing.T) {
		r := StringSliceExample{Field: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "111"}}
		err := Validate(r)
		assert.ErrorContains(t, err, "max length of strField is 10")
	})
	t.Run("contains comma", func(t *testing.T) {
		r := StringSliceExample{Field: []string{"hello", "hello, world"}}
		err := Validate(r)
		assert.ErrorContains(t, err, "list values cannot contain commas")
	})
	t.Run("contains string which starts with illegal character", func(t *testing.T) {
		r := StringSliceExample{Field: []string{"@example.com", "hello, world"}}
		err := Validate(r)
		assert.ErrorContains(t, err, "first character '@' is not allowed")
	})
	t.Run("contains string which contains an illegal character", func(t *testing.T) {
		r := StringSliceExample{Field: []string{"example!.com", "hello, world"}}
		err := Validate(r)
		assert.ErrorContains(t, err, "character '!' at position 7 is not allowed")
	})
}
