package env

import (
	"fmt"
	"reflect"
	"strconv"
)

type fieldSetterFunc func(v string) (reflect.Value, error)

type fieldSetter interface {
	Set(v string, field reflect.Value) error
}

func (f fieldSetterFunc) Set(v string, field reflect.Value) error {
	value, err := f(v)
	if err != nil {
		return err
	}

	field.Set(value.Convert(field.Type()))
	return nil
}

func charSliceSetter(sliceType reflect.Type) fieldSetterFunc {
	return func(v string) (reflect.Value, error) {
		result := reflect.MakeSlice(sliceType, len(v), len(v))
		strValue := reflect.ValueOf(v)
		for i := 0; i < len(v); i++ {
			sliceEl := result.Index(i)
			char := strValue.Index(i)
			sliceEl.Set(char.Convert(sliceType.Elem()))
		}
		return result, nil
	}
}

var fieldKindToParser = map[reflect.Kind]fieldSetterFunc{
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

func validateFieldAndReturnSetter(field reflect.Value) (fieldSetter, error) {
	fieldType := field.Type()
	for fieldType.Kind() == reflect.Pointer {
		fieldType = fieldType.Elem()
	}

	if fieldType.Kind() == reflect.Slice {
		switch fieldType.Elem().Kind() {
		case reflect.Int32:
			return concreteFieldInitializer{charSliceSetter(fieldType)}, nil
		case reflect.Uint8:
			return concreteFieldInitializer{charSliceSetter(fieldType)}, nil
		default:
		}
	}

	parser, ok := fieldKindToParser[fieldType.Kind()]
	if !ok {
		return nil, fmt.Errorf("unsupported field type %s", field.Type().Name())
	}

	return concreteFieldInitializer{parser}, nil
}

type concreteFieldInitializer struct {
	next fieldSetter
}

func (c concreteFieldInitializer) Set(v string, field reflect.Value) error {
	fieldType := field.Type()
	for fieldType.Kind() == reflect.Pointer {
		fieldValue := reflect.New(fieldType.Elem())
		field.Set(fieldValue)
		field = fieldValue.Elem()
		fieldType = fieldValue.Elem().Type()
	}

	return c.next.Set(v, field)
}
