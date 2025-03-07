package env

import (
	"fmt"
	"reflect"
	"strconv"
)

type fieldParserFunc func(v string) (reflect.Value, error)

type fieldParser interface {
	Parse(v string) (reflect.Value, error)
}

func (f fieldParserFunc) Parse(v string) (reflect.Value, error) {
	return f(v)
}

var fieldKindToParser = map[reflect.Kind]fieldParserFunc{
	reflect.String: func(v string) (reflect.Value, error) {
		return reflect.ValueOf(v), nil
	},
	reflect.Bool: func(v string) (reflect.Value, error) {
		return asReflectValue(strconv.ParseBool(v))
	},
	reflect.Int: func(v string) (reflect.Value, error) {
		return asReflectValue(strconv.Atoi(v))
	},
	reflect.Int8: func(v string) (reflect.Value, error) {
		return asReflectValueAndCast[int8](strconv.ParseInt(v, 10, 8))
	},
	reflect.Int16: func(v string) (reflect.Value, error) {
		return asReflectValueAndCast[int16](strconv.ParseInt(v, 10, 16))
	},
	reflect.Int32: func(v string) (reflect.Value, error) {
		return asReflectValueAndCast[int32](strconv.ParseInt(v, 10, 32))
	},
	reflect.Int64: func(v string) (reflect.Value, error) {
		return asReflectValueAndCast[int64](strconv.ParseInt(v, 10, 64))
	},
	reflect.Uint: func(v string) (reflect.Value, error) {
		return asReflectValueAndCast[uint](strconv.ParseUint(v, 10, 0))
	},
	reflect.Uint8: func(v string) (reflect.Value, error) {
		return asReflectValueAndCast[uint8](strconv.ParseUint(v, 10, 8))
	},
	reflect.Uint16: func(v string) (reflect.Value, error) {
		return asReflectValueAndCast[uint16](strconv.ParseUint(v, 10, 16))
	},
	reflect.Uint32: func(v string) (reflect.Value, error) {
		return asReflectValueAndCast[uint32](strconv.ParseUint(v, 10, 32))
	},
	reflect.Uint64: func(v string) (reflect.Value, error) {
		return asReflectValueAndCast[uint64](strconv.ParseUint(v, 10, 64))
	},
	reflect.Float32: func(v string) (reflect.Value, error) {
		return asReflectValueAndCast[float32](strconv.ParseFloat(v, 32))
	},
	reflect.Float64: func(v string) (reflect.Value, error) {
		return asReflectValueAndCast[float64](strconv.ParseFloat(v, 64))
	},
}

func asReflectValue[T any](v T, err error) (reflect.Value, error) {
	return reflect.ValueOf(v), err
}

func asReflectValueAndCast[C any, T any](v T, err error) (reflect.Value, error) {
	var c C
	return reflect.ValueOf(v).Convert(reflect.TypeOf(c)), err
}

func validateFieldAndReturnParser(field reflect.Value) (fieldParser, error) {
	parser, ok := fieldKindToParser[field.Kind()]
	if !ok {
		return nil, fmt.Errorf("unsupported field type %s", field.Type().Name())
	}

	return parser, nil
}
