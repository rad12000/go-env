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
//   - []byte
//   - []rune
//
// Note: pointers to [Unmarshaler] implementations are supported.
func Unmarshal(env []string, out any) error {
	return UnmarshalPrefix(env, out, "")
}

// UnmarshalPrefix is just like [Unmarshal], but allows the caller to provide a prefix, which will be prepended to
// field environment variable names (excepting those that are explicitly set via the `env` tag.
func UnmarshalPrefix(env []string, out any, prefix string) error {
	if out == nil {
		return errors.New("env: out must be a non-nil pointer to a struct")
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
	if err := loadEnvVarsIntoStruct(value, envVars, "", prefix); err != nil {
		return fmt.Errorf("failed to unmarshal environment variables into struct %T: %w", out, err)
	}

	return nil
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
	Required   bool
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
		standardName := strings.ToLower(strings.TrimSpace(keyVal[0]))
		if strings.EqualFold(standardName, "required") {
			result.Required = true
		}

		if len(keyVal) != 2 {
			continue
		}

		keyValPairs[standardName] = strings.ReplaceAll(keyVal[1], "\\s", " ")
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

	if !envValueSet && fTag.Required {
		return newFieldParseError(errors.New("missing required value"), fieldPath, envName)
	}

	didUnmarshal, err := attemptUnmarshal(field, envValue, envValueSet)
	if err != nil {
		return newFieldParseError(err, fieldPath, envName)
	}

	if didUnmarshal {
		return nil
	}

	if field.Kind() == reflect.Struct {
		return loadEnvVarsIntoStruct(field, envVars, fmt.Sprintf("%s.", fieldPath), fmt.Sprintf("%s_", envName))
	}

	fieldValueSetter, err := validateFieldAndReturnSetter(field)
	if err != nil {
		return newFieldParseError(err, fieldPath, envName)
	}

	if !envValueSet {
		return nil
	}

	err = fieldValueSetter.Set(envValue, field)
	if err != nil {
		return newFieldParseError(err, fieldPath, envName)
	}

	return nil
}

var unmarshalerType = reflect.TypeOf((*Unmarshaler)(nil)).Elem()

func attemptUnmarshal(field reflect.Value, envValue string, envValueSet bool) (bool, error) {
	field = field.Addr()
	fieldType := field.Type()
	var (
		unmarshalerDepth int
		foundUnmarshaler = true
	)

	for !fieldType.Implements(unmarshalerType) {
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
			unmarshalerDepth++
			continue
		}

		foundUnmarshaler = false
		break
	}

	if !foundUnmarshaler {
		return false, nil
	}

	if !envValueSet {
		return true, nil
	}

	unmarshalerValue := field
	for i := 0; i < unmarshalerDepth; i++ {
		val := reflect.New(unmarshalerValue.Type().Elem().Elem())
		unmarshalerValue.Elem().Set(val)
		unmarshalerValue = unmarshalerValue.Elem()
	}

	unmarshaler, isUnmarshaler := unmarshalerValue.Interface().(Unmarshaler)
	if !isUnmarshaler {
		panic("unreachable case: must be unmarshaler")
	}

	return true, unmarshaler.UnmarshalEnv(envValue)
}

func isNum(r rune) bool {
	return r >= '0' && r <= '9'
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func fieldNameToEnvVariable(name string) string {
	var (
		sb        strings.Builder
		nameRunes = []rune(name)
		prevRune  = '_' // Initially an underscore to avoid ever prefixing an env name with an underscore.
	)

	// writeRune will only write an underscore if the previously written rune was not also an underscore.
	// It also updates the prevRune.
	writeRune := func(r rune) {
		if r == '_' && r == prevRune {
			return
		}

		prevRune = r
		sb.WriteRune(r)
	}

	for i := 0; i < len(name); i++ {
		cur := nameRunes[i]
		if i == len(name)-1 {
			writeRune(unicode.ToUpper(cur))
			continue
		}

		var (
			next                   = rune(name[i+1])
			isLetterFollowedByNum  = isLetter(cur) && isNum(next)
			isNumFollowedByALetter = isNum(cur) && isLetter(next)
			isUpperFollowedByLower = unicode.IsUpper(cur) && unicode.IsLower(next)
			isLowerFollowedByUpper = unicode.IsLower(cur) && unicode.IsUpper(next)
		)

		switch {
		case isUpperFollowedByLower:
			writeRune('_')
			writeRune(cur)
		case isLetterFollowedByNum:
			fallthrough
		case isNumFollowedByALetter:
			fallthrough
		case isLowerFollowedByUpper:
			writeRune(unicode.ToUpper(cur))
			writeRune('_')
		default:
			writeRune(unicode.ToUpper(cur))
		}
	}

	return sb.String()
}
