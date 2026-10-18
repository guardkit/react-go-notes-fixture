package notes

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
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

// maxDoneBodyBytes bounds the body of a done-flag request. One JSON boolean is
// the whole request, so a body over this size is never a valid toggle.
const maxDoneBodyBytes = 1024

// errInvalidDone reports a body that is not one JSON object at all: the wrong
// media type, invalid JSON, trailing content, a field the request does not
// define, or a body over maxDoneBodyBytes. The PATCH handler answers it with
// the generic "Invalid request", exactly as the create handler does.
var errInvalidDone = errors.New("invalid note done flag")

// errDoneNotBoolean is the refusal of the completion flag itself: a done field
// that is absent, null, or anything but the JSON boolean true or false. Its
// message names the field, so the PATCH handler answers 400 with it rather than
// the generic "Invalid request".
var errDoneNotBoolean = errors.New("done must be the JSON boolean true or false")

// ValidateDone applies the wire contract to the flag exactly as the body carried
// it: only the two JSON booleans are accepted. A field the body never named, a
// null, and every non-boolean - a string, a number, an object, an array - all
// answer errDoneNotBoolean, so no caller can toggle a note with a value that is
// not a flag.
func ValidateDone(raw *json.RawMessage) (bool, error) {
	if raw == nil {
		return false, errDoneNotBoolean
	}
	flag := bytes.TrimSpace(*raw)
	if len(flag) == 0 || bytes.Equal(flag, []byte("null")) {
		return false, errDoneNotBoolean
	}
	var done bool
	if err := json.Unmarshal(flag, &done); err != nil {
		return false, errDoneNotBoolean
	}
	return done, nil
}

// decodeDone reads the completion flag out of a PATCH body before anything is
// written. The body must be one application/json object carrying done as the
// JSON boolean true or false: a missing, null or non-boolean flag answers
// errDoneNotBoolean, and a body that is not such an object answers
// errInvalidDone or the decoder's own failure. Either refusal reaches the caller
// as a 400 with no storage call made, so a note is never toggled by a body the
// contract does not accept.
func decodeDone(w http.ResponseWriter, r *http.Request) (bool, error) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return false, errInvalidDone
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxDoneBodyBytes))
	if err != nil {
		return false, err
	}
	if !utf8.Valid(body) {
		return false, errInvalidDone
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return false, errInvalidDone
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	var input struct {
		Done *json.RawMessage `json:"done"`
	}
	if err := decoder.Decode(&input); err != nil {
		return false, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return false, errInvalidDone
	}
	return ValidateDone(input.Done)
}
