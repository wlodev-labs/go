package ptr

import "time"

// Bool returns a pointer to the given bool value.
func Bool(b bool) *bool {
	return &b
}

// String returns a pointer to the given string value.
func String(s string) *string {
	return &s
}

// Time returns a pointer to the given time.Time value.
func Time(t time.Time) *time.Time {
	return &t
}

// Int returns a pointer to the given int value.
func Int(i int) *int {
	return &i
}
