package pinata

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var pinataAPIKey string
var pinataAPISecret string

func init() {
	pinataAPIKey = os.Getenv("PINATA_API_KEY")
	pinataAPISecret = os.Getenv("PINATA_API_SECRET")
}

func TestNewProvider(t *testing.T) {
	if pinataAPIKey == "" || pinataAPISecret == "" {
		t.Skip("no api key or secret; cannot test")
	}

	tests := []struct {
		key    string
		secret string
		err    error
	}{
		{ // 0
			key:    pinataAPIKey,
			secret: pinataAPISecret,
		},
		{ // 1
			key:    "",
			secret: pinataAPISecret,
			err:    errors.New("Invalid API key / secret key combo"),
		},
		{ // 2
			key:    pinataAPIKey,
			secret: "",
			err:    errors.New("Invalid API key / secret key combo"),
		},
		{ // 3
			key:    "",
			secret: "",
			err:    errors.New("Invalid API key / secret key combo"),
		},
		{ // 4
			key:    pinataAPIKey,
			secret: pinataAPISecret + "!",
			err:    errors.New("Invalid API key / secret key combo"),
		},
	}

	for i, test := range tests {
		_, err := NewProvider(test.key, test.secret)
		if test.err != nil {
			assert.NotNil(t, err, fmt.Sprintf("missing expected error at test %d", i))
			assert.Equal(t, test.err.Error(), err.Error(), fmt.Sprintf("unexpected error value at test %d", i))
		} else {
			assert.Nil(t, err, fmt.Sprintf("unexpected error at test %d", i))
		}
	}
}
