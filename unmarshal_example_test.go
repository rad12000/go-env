package env_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rad12000/go-env"
	"os"
)

type foo byte

func ExampleUnmarshal_plainStruct() {
	var plainStruct struct {
		URL        string
		DeleteUser *bool
		Bytes      []foo
		Auth       struct {
			SigningKey string
			TTLSeconds uint
		}
	}

	revert := Must(
		SetEnv(
			"URL", "https://example.com",
			"AUTH_SIGNING_KEY", "signing_key",
			"AUTH_TTL_SECONDS", "60",
			"BYTES", "these are bytes",
			"DELETE_USER", "true",
		),
	)
	defer revert()

	err := env.Unmarshal(os.Environ(), &plainStruct)
	fmt.Println(err)
	fmt.Println("url =", plainStruct.URL)
	fmt.Println("signing key =", plainStruct.Auth.SigningKey)
	fmt.Println("ttl seconds =", plainStruct.Auth.TTLSeconds)
	fmt.Println("delete user =", *plainStruct.DeleteUser)
	fmt.Println("bytes =", string(plainStruct.Bytes))

	// Output:
	// <nil>
	// url = https://example.com
	// signing key = signing_key
	// ttl seconds = 60
	// delete user = true
	// bytes = these are bytes
}

func ExampleUnmarshal_envTags() {
	var plainStruct struct {
		UnsupportedType chan struct{} `env:"-"`
		Name            string        `env:",required default=John\\sDoe"`
		URL             string
		FavoriteColor   string `env:",default=blue"`
		Authentication  struct {
			SigningKey string
			TTLSeconds uint `env:"JWT_TTL"`
			MaxAge     uint
		} `env:"AUTH"`
	}

	revert := Must(
		SetEnv(
			"URL", "https://example.com",
			"AUTH_SIGNING_KEY", "signing_key",
			"JWT_TTL", "60",
		),
	)
	defer revert()

	err := env.Unmarshal(os.Environ(), &plainStruct)
	fmt.Println(err)
	fmt.Println("url =", plainStruct.URL)
	fmt.Println("signing key =", plainStruct.Authentication.SigningKey)
	fmt.Println("ttl seconds =", plainStruct.Authentication.TTLSeconds)
	fmt.Println("favorite color =", plainStruct.FavoriteColor)
	fmt.Println("name =", plainStruct.Name)

	// Output:
	// <nil>
	// url = https://example.com
	// signing key = signing_key
	// ttl seconds = 60
	// favorite color = blue
	// name = John Doe
}

func ExampleUnmarshal_error() {
	var plainStruct struct {
		UnsupportedType chan struct{}
	}

	err := env.Unmarshal(os.Environ(), &plainStruct)
	var fieldErr env.FieldParseError
	fmt.Println(errors.As(err, &fieldErr))
	fmt.Println(fieldErr.Field())
	fmt.Println(fieldErr.EnvVar())
	fmt.Println(fieldErr.Error())

	// Output:
	// true
	// UnsupportedType
	// UNSUPPORTED_TYPE
	// failed to unmarshal environment variable "UNSUPPORTED_TYPE" into field "UnsupportedType": unsupported field type
}

func ExampleUnmarshal_customTypes() {
	type fooInt int64
	var out struct {
		Foo fooInt
	}

	revert := Must(SetEnv("FOO", "1234"))
	defer revert()

	fmt.Println(env.Unmarshal(os.Environ(), &out))
	fmt.Println(out.Foo)

	// Output:
	// <nil>
	// 1234
}

func ExampleUnmarshal_implementUnmarshaler() {
	var out struct {
		ValidIDs  sliceUnmarshaler `env:"VALID_IDS"`
		PrimaryID string
	}

	ids := `["id1", "id2"]`
	revert := Must(SetEnv("VALID_IDS", ids, "PRIMARY_ID", "4321"))
	defer revert()

	fmt.Println(env.Unmarshal(os.Environ(), &out))
	fmt.Println(out.ValidIDs)
	fmt.Println(out.PrimaryID)

	// Output:
	// <nil>
	// [id1 id2]
	// 4321
}

type sliceUnmarshaler []string

func (s *sliceUnmarshaler) UnmarshalEnv(value string) error {
	return json.Unmarshal([]byte(value), s)
}

func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func SetEnv(values ...string) (revert func(), err error) {
	if len(values) < 2 {
		return func() {}, nil
	}

	var originalValues []string
	for i := 1; i < len(values); i += 2 {
		key := values[i-1]
		value := values[i]
		currentValue, exists := os.LookupEnv(key)
		if exists {
			originalValues = append(originalValues, key, currentValue)
		}

		if err := os.Setenv(key, value); err != nil {
			return nil, err
		}
	}

	return func() {
		if _, err := SetEnv(originalValues...); err != nil {
			panic(err)
		}
	}, nil
}
