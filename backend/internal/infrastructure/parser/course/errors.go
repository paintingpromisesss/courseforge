package course

import "fmt"

// Error locates a failure at a manifest path; Field is set when a specific
// field is at fault (e.g. "week-1/slices/topic.yaml": field "slug": …).
type Error struct {
	Path  string
	Field string
	Msg   string
}

func (e *Error) Error() string {
	switch {
	case e.Path != "" && e.Field != "":
		return fmt.Sprintf("%s: field %q: %s", e.Path, e.Field, e.Msg)
	case e.Path != "":
		return fmt.Sprintf("%s: %s", e.Path, e.Msg)
	default:
		return e.Msg
	}
}

func errAt(path, msg string) *Error {
	return &Error{Path: path, Msg: msg}
}

func errField(path, field, msg string) *Error {
	return &Error{Path: path, Field: field, Msg: msg}
}
