package env

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

type Unmarshaler interface {
	UnmarshalEnv(v string) error
}

// Unmarshal accepts a list of environment variables, typically sourced from [os.Environ], and attempts
// to unmarshal the provided variables into out, which must be a non-nil pointer to a struct.
// Assuming out is a valid pointer to a struct, the error returned by [Unmarshal] will always implement the [FieldParseError] interface.
//
// Unmarshal will attempt set values on a given struct field, according to the following ruleset:
//
//  1. Determine the correct environment variable for the struct field:
//
//     - Does the field have a name in the `env:""` tag? If yes, use this name.
//
//     - Construct the field name by inserting an underscore between any two letters where a lower case letter,
//     is immediately followed by an upper case letter. (e.g. fooBar -> FOO_BAR)
//
//     - OR, insert an underscore prior to any upper case letter that is
//     immediately followed by a lower case letter. (e.g. JSONString -> JSON_STRING)
//
//  2. Check if an environment variable exists with by the name determined in step 1.
//
//     - If yes, use this value in step 3.
//
//     - Otherwise, check if a default value was specified in the `env:",default="` tag.
//
//     -- If yes, use this value in step 3.
//
//     -- Otherwise, stop processing the field. (i.e. do not continue to step 3.)
//
//  3. Check if the field type implements the Unmarshaler interface.
//
//     - If yes, invoke the [Unmarshaler.UnmarshalEnv], returning the error if non-nil.
//
//     - Otherwise, check if the field is a struct.
//
//     -- If yes, parse the struct fields, starting back at step 1.
//
//     -- Otherwise, attempt to parse the environment variable value into the correct type, and set it on the field.
//
// # Supported field types
//
// The following is the list of supported types for struct fields:
//
//   - Unmarshaler
//   - struct
//   - string
//   - bool
//   - int8
//   - int16
//   - int32
//   - int64
//   - uint8
//   - uint16
//   - uint32
//   - uint64
//   - float32
//   - float64
//
// Note: pointers to any of the above values are NOT supported.
func Unmarshal(env []string, out any) error {
	if out == nil {
		return errors.New("out must be a non-nil pointer to a struct")
	}

	ptr := reflect.ValueOf(out)
	if ptr.Kind() != reflect.Pointer {
		return errors.New("out must be a non-nil pointer to a struct")
	}

	value := ptr.Elem()
	if value.Kind() != reflect.Struct {
		return errors.New("out must be a non-nil pointer to a struct")
	}

	envVars := parseEnv(env)
	return loadEnvVarsIntoStruct(value, envVars, "", "")
}

func parseEnv(vars []string) map[string]string {
	m := make(map[string]string, len(vars))
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			continue
		}
		m[parts[0]] = parts[1]
	}
	return m
}

func loadEnvVarsIntoStruct(out reflect.Value, envVars map[string]string, fieldPath, envVarPrefix string) error {
	numFields := out.NumField()
	outType := out.Type()
	if numFields == 0 {
		return nil
	}

	for i := 0; i < numFields; i++ {
		field := out.Field(i)
		fieldType := outType.Field(i)
		if !fieldType.IsExported() {
			continue
		}

		if err := processField(field, fieldType, envVars, fieldPath, envVarPrefix); err != nil {
			return err
		}
	}

	return nil
}

type fieldTag struct {
	Name       string
	Default    string
	HasDefault bool
}

func parseFieldTag(tag string) fieldTag {
	tagParts := strings.SplitN(tag, ",", 2)
	envName := strings.TrimSpace(tagParts[0])
	result := fieldTag{Name: envName}
	if len(tagParts) == 1 {
		return result
	}

	keyValPairs := make(map[string]string)
	for _, pair := range strings.Split(tagParts[1], " ") {
		keyVal := strings.SplitN(pair, "=", 2)
		if len(keyVal) != 2 {
			continue
		}

		keyValPairs[strings.ToLower(keyVal[0])] = keyVal[1]
	}

	result.Default, result.HasDefault = keyValPairs["default"]
	return result
}

func processField(field reflect.Value, fieldType reflect.StructField, envVars map[string]string, fieldPathPrefix, envVarPrefix string) error {
	fTag := parseFieldTag(fieldType.Tag.Get("env"))
	envName := fTag.Name
	if envName == "-" {
		return nil
	}

	if envName == "" {
		envName = envVarPrefix + fieldNameToEnvVariable(fieldType.Name)
	}

	var (
		envValue, envValueSet = envVars[envName]
		fieldPath             = fieldPathPrefix + fieldType.Name
	)

	if !envValueSet && fTag.HasDefault {
		envValue = fTag.Default
		envValueSet = true
	}

	if unmarshaler, isUnmarshaler := field.Addr().Interface().(Unmarshaler); isUnmarshaler {
		if envValueSet {
			if err := unmarshaler.UnmarshalEnv(envValue); err != nil {
				return newFieldParseError(err, fieldPath, envName)
			}
		}
		return nil
	}

	if field.Kind() == reflect.Struct {
		return loadEnvVarsIntoStruct(field, envVars, fmt.Sprintf("%s.", fieldPath), fmt.Sprintf("%s_", envName))
	}

	fieldValueParser, err := validateFieldAndReturnParser(field)
	if err != nil {
		return newFieldParseError(err, fieldPath, envName)
	}

	if !envValueSet {
		return nil
	}

	fieldValue, err := fieldValueParser.Parse(envValue)
	if err != nil {
		return newFieldParseError(err, fieldPath, envName)
	}

	field.Set(fieldValue.Convert(fieldType.Type))

	return nil
}

func fieldNameToEnvVariable(name string) string {
	var (
		sb                        strings.Builder
		lastIterDidWriteSeparator bool
	)

	for i, cur := range name {
		if i == len(name)-1 {
			sb.WriteRune(unicode.ToUpper(cur))
			continue
		}
		next := rune(name[i+1])

		if i > 0 && !lastIterDidWriteSeparator && unicode.IsUpper(cur) && unicode.IsLower(next) {
			lastIterDidWriteSeparator = true
			sb.WriteRune('_')
			sb.WriteRune(cur)
			continue
		}

		if unicode.IsLower(cur) && unicode.IsUpper(next) {
			lastIterDidWriteSeparator = true
			sb.WriteRune(unicode.ToUpper(cur))
			sb.WriteRune('_')
			continue
		}

		sb.WriteRune(unicode.ToUpper(cur))
		lastIterDidWriteSeparator = false
	}

	return sb.String()
}
