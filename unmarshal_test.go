package env

import (
	"testing"
)

func TestFieldNameToEnvVariable(t *testing.T) {
	tt := [][2]string{
		{"JSONString", "JSON_STRING"},
		{"fooBar", "FOO_BAR"},
		{"fooJSON", "FOO_JSON"},
		{"MagicMike", "MAGIC_MIKE"},
		{"JSON1String", "JSON1_STRING"},
	}

	for _, tc := range tt {
		t.Run(tc[0], func(t *testing.T) {
			if fieldNameToEnvVariable(tc[0]) != tc[1] {
				t.Fatalf("Expected %s to equal %s", tc[0], tc[1])
			}
		})
	}
}
