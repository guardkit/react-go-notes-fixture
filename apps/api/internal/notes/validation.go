package notes

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var errInvalidLabel = errors.New("invalid note label")

// ValidateLabel applies the wire contract before any persistence call.
func ValidateLabel(value string) (string, error) {
	label := strings.TrimSpace(value)
	if label == "" || utf8.RuneCountInString(label) > 80 {
		return "", errInvalidLabel
	}
	return label, nil
}
