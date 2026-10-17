package notes

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// maxLabelChars is the inclusive note-title limit, counted in Unicode code
// points (ASSUM-002). It is enforced on note creation only (ASSUM-003).
const maxLabelChars = 200

// errInvalidBody is the refusal every note write shares for a body that is not
// one JSON object of the accepted size: the create decoder and the update
// decoder both answer with it, and the handler keeps it behind the generic
// "Invalid request" so an undecodable body discloses nothing about the field
// that broke it.
var errInvalidBody = errors.New("invalid request body")

// errLabelBlank is the blank-title refusal: an absent, empty or whitespace-only
// title. Its message names the title field, so the create handler answers 400
// with it instead of the generic "Invalid request" used for undecodable bodies.
var errLabelBlank = errors.New("label (title) must not be blank")

// errLabelTooLong is the over-long title refusal. Its message names the title
// field and the character limit, so the create handler answers 400 with it
// instead of the generic "Invalid request" used for undecodable bodies.
var errLabelTooLong = fmt.Errorf("label (title) must be %d characters or fewer", maxLabelChars)

// errDoneMissing is the refusal of a completion flag that was never given: an
// absent done key and a done set to null both say nothing about completion, and
// neither is a flag the note can be stored with.
var errDoneMissing = errors.New("done (completion flag) must be true or false")

// errDoneNotBoolean is the refusal of a completion flag that is not a boolean:
// a string, a number, an object or an array is not a completion state, and the
// update handler will not guess which one the caller meant.
var errDoneNotBoolean = errors.New("done (completion flag) must be the JSON boolean true or false")

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

// ValidateDone applies the wire contract of the one mutable field of a note.
// The flag arrives as the raw JSON value the caller sent, so the three refusals
// stay distinguishable: no value at all and JSON null are a missing flag, and
// anything that is not the literal true or false is not a flag either. Only
// true and false are accepted, and they are the whole domain of the field.
func ValidateDone(value *json.RawMessage) (bool, error) {
	if value == nil {
		return false, errDoneMissing
	}
	switch flag := strings.TrimSpace(string(*value)); flag {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "", "null":
		return false, errDoneMissing
	default:
		return false, errDoneNotBoolean
	}
}
