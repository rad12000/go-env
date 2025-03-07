package env

import "fmt"

type FieldParseError interface {
	EnvVar() string
	Field() string
	Unwrap() error
	Error() string
}

func newFieldParseError(err error, field, envVar string) FieldParseError {
	return fieldParseError{
		envVar: envVar,
		err:    err,
		field:  field,
	}
}

type fieldParseError struct {
	envVar string
	err    error
	field  string
}

func (l fieldParseError) EnvVar() string {
	return l.envVar
}

func (l fieldParseError) Field() string {
	return l.field
}

func (l fieldParseError) Unwrap() error {
	return l.err
}

func (l fieldParseError) Error() string {
	return fmt.Sprintf("failed to set value of field %s, mapping to env var %s: %s", l.field, l.envVar, l.err)
}
