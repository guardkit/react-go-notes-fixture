package notes

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// maxLabelChars is the inclusive note-title limit, counted in Unicode code
// points (ASSUM-002). It is enforced on note creation only (ASSUM-003).
const maxLabelChars = 200

var errInvalidLabel = errors.New("invalid note label")

// errLabelBlank is the blank-title refusal: an absent, empty or whitespace-only
// title. Its message names the title field, so the create handler answers 400
// with it instead of the generic "Invalid request" used for undecodable bodies.
var errLabelBlank = errors.New("label (title) must not be blank")

// errLabelTooLong is the over-long title refusal. Its message names the title
// field and the character limit, so the create handler answers 400 with it
// instead of the generic "Invalid request" used for undecodable bodies.
var errLabelTooLong = fmt.Errorf("label (title) must be %d characters or fewer", maxLabelChars)

// ValidateLabel applies the wire contract before any persistence call.
func ValidateLabel(value string) (string, error) {
	label := strings.TrimSpace(value)
	if label == "" {
		return "", errLabelBlank
	}
	if utf8.RuneCountInString(label) > maxLabelChars {
		return "", errLabelTooLong
	}
	return label, nil
}
