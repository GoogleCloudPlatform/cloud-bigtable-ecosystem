package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseProtocolVersion(t *testing.T) {

	a := []string{"3", "4", "5", "65", "66", "invalid"}
	for _, val := range a {
		res, boolval := parseProtocolVersion(val)
		assert.NotNilf(t, res, "should not be nil")
		if val == "invalid" {
			assert.Equalf(t, false, boolval, "should be true")
			break
		}
		assert.Equalf(t, true, boolval, "should be true")
	}
}

func Test_maybeAddPort(t *testing.T) {
	res := maybeAddPort("127.0.0.1", "7000")
	assert.Equalf(t, res, "127.0.0.1:7000", "assert equal")
}

func TestBuildBindAndPort(t *testing.T) {
	tests := []struct {
		name     string
		bindPort string
		port     int
		want     string
	}{
		{
			name:     "default case",
			bindPort: "0.0.0.0:%s",
			port:     9042,
			want:     "0.0.0.0:9042",
		},
		{
			name:     "no string interpolation",
			bindPort: "0.0.0.0",
			port:     9042,
			want:     "0.0.0.0:9042",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildBindAndPort(tt.bindPort, tt.port)
			assert.Equal(t, tt.want, got)
		})
	}
}
