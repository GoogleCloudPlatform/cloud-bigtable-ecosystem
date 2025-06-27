/*
 * Copyright (C) 2025 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy of
 * the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
 * License for the specific language governing permissions and limitations under
 * the License.
 */

package translator

import (
	"math"
	"testing"
)

func TestAppend(t *testing.T) {
	buf := []byte{}

	// Append float64
	buf, err := Append(buf, 3.14)
	if err != nil {
		t.Fatal(err)
	}

	// Append int64
	buf, err = Append(buf, int64(12345))
	if err != nil {
		t.Fatal(err)
	}

	// Append uint64
	buf, err = Append(buf, uint64(98765))
	if err != nil {
		t.Fatal(err)
	}

	// Error test: unsupported type
	_, err = Append(buf, struct{}{})
	if err == nil {
		t.Fatal("expected error for unsupported type, got nil")
	}
}
func TestParseInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		dir      byte
		expected int64
		remain   string
		wantErr  bool
	}{
		// Single byte cases
		{
			name:     "single byte positive",
			input:    "\x40\x00", // 0x40 ^ 0x00 = 0x40 (64) ^ 0x80 = -64
			dir:      0x00,
			expected: -64,
			remain:   "\x00",
			wantErr:  false,
		},
		{
			name:     "single byte with dir",
			input:    "\x85\x00", // 0x85 ^ 0x05 = 0x80 (0) ^ 0x80 = 0
			dir:      0x05,
			expected: 0,
			remain:   "\x00",
			wantErr:  false,
		},
		{
			name:     "invalid max leading",
			input:    "\xff\x40\x00", // 0x40 > 0x3f
			dir:      0x00,
			expected: 0,
			remain:   "",
			wantErr:  true,
		},
		{
			name:     "empty input",
			input:    "",
			dir:      0x00,
			expected: 0,
			remain:   "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dst int64
			remain, err := parseInt64(&dst, tt.input, tt.dir)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseInt64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if dst != tt.expected {
					t.Errorf("parseInt64() got = %v, want %v", dst, tt.expected)
				}
				if remain != tt.remain {
					t.Errorf("parseInt64() remain = %q, want %q", remain, tt.remain)
				}
			}
		})
	}
}
func TestParseUint64(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		dir      byte
		expected uint64
		remain   string
		wantErr  bool
	}{
		// Valid single-byte cases
		{
			name:     "zero value",
			input:    "\x00\xff", // n=0, value=0
			dir:      0x00,
			expected: 0,
			remain:   "\xff",
			wantErr:  false,
		},
		{
			name:     "single byte value",
			input:    "\x01\xab\xff", // n=1, value=0xab
			dir:      0x00,
			expected: 0xab,
			remain:   "\xff",
			wantErr:  false,
		},

		// Valid multi-byte cases
		{
			name:     "two byte value",
			input:    "\x02\xab\xcd\xff", // n=2, value=0xabcd
			dir:      0x00,
			expected: 0xabcd,
			remain:   "\xff",
			wantErr:  false,
		},
		{
			name:     "two byte with dir",
			input:    "\x07\xa5\xcd\xff", // n=2^0x05=0x07, value bytes XORed with 0x05
			dir:      0x05,
			expected: 0xa0c8, // (0xa5^0x05)<<8 | (0xcd^0x05)
			remain:   "\xff",
			wantErr:  false,
		},
		{
			name:     "max size (8 bytes)",
			input:    "\x08\x01\x02\x03\x04\x05\x06\x07\x08\xff",
			dir:      0x00,
			expected: 0x0102030405060708,
			remain:   "\xff",
			wantErr:  false,
		},

		// Error cases
		{
			name:     "empty input",
			input:    "",
			dir:      0x00,
			expected: 0,
			remain:   "",
			wantErr:  true,
		},
		{
			name:     "n too large",
			input:    "\x09\x01\x02\x03\x04\x05\x06\x07\x08\x09\xff", // n=9 > 8
			dir:      0x00,
			expected: 0,
			remain:   "",
			wantErr:  true,
		},
		{
			name:     "incomplete input",
			input:    "\x02\xab", // needs 2 bytes but only has 1
			dir:      0x00,
			expected: 0,
			remain:   "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dst uint64
			remain, err := parseUint64(&dst, tt.input, tt.dir)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseUint64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if dst != tt.expected {
					t.Errorf("parseUint64() got = %#x, want %#x", dst, tt.expected)
				}
				if remain != tt.remain {
					t.Errorf("parseUint64() remain = %q, want %q", remain, tt.remain)
				}
			}
		})
	}
}

func TestParseFloat64(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		dir      byte
		expected float64
		remain   string
		wantErr  bool
	}{
		// Positive numbers
		{
			name:     "positive zero",
			input:    "\x80\x00", // 0 in parseInt64 encoding
			dir:      0x00,
			expected: 0.0,
			remain:   "\x00",
			wantErr:  false,
		},
		{
			name:     "positive one",
			input:    "\x81\x00", // 1 in parseInt64 encoding
			dir:      0x00,
			expected: math.Float64frombits(1),
			remain:   "\x00",
			wantErr:  false,
		},
		{
			name:     "positive with dir",
			input:    "\x85\x00", // 0x85 ^ 0x05 = 0x80 (0) -> 0
			dir:      0x05,
			expected: 0.0,
			remain:   "\x00",
			wantErr:  false,
		},

		// Error cases
		{
			name:     "empty input",
			input:    "",
			dir:      0x00,
			expected: 0,
			remain:   "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dst float64
			remain, err := parseFloat64(&dst, tt.input, tt.dir)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseFloat64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if tt.name == "NaN" {
					if !math.IsNaN(dst) {
						t.Error("parseFloat64() expected NaN, got", dst)
					}
				} else if dst != tt.expected {
					t.Errorf("parseFloat64() got = %v (%#x), want %v (%#x)",
						dst, math.Float64bits(dst),
						tt.expected, math.Float64bits(tt.expected))
				}
				if remain != tt.remain {
					t.Errorf("parseFloat64() remain = %q, want %q", remain, tt.remain)
				}
			}
		})
	}
}

func TestParseString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		dir      byte
		expected string
		remain   string
		wantErr  bool
	}{
		// Simple cases with no escaping
		{
			name:     "simple string",
			input:    "hello\x00\x01",
			dir:      increasing,
			expected: "hello",
			remain:   "",
			wantErr:  false,
		},
		{
			name:     "empty string",
			input:    "\x00\x01",
			dir:      increasing,
			expected: "",
			remain:   "",
			wantErr:  false,
		},

		// Escaping cases
		{
			name:     "escaped 0x00",
			input:    "hello\x00\xffworld\x00\x01",
			dir:      increasing,
			expected: "hello\x00world",
			remain:   "",
			wantErr:  false,
		},
		{
			name:     "escaped 0xff",
			input:    "hello\xff\x00world\x00\x01",
			dir:      increasing,
			expected: "hello\xffworld",
			remain:   "",
			wantErr:  false,
		},
		{
			name:     "multiple escapes",
			input:    "\x00\xff\xff\x00\x00\x01",
			dir:      increasing,
			expected: "\x00\xff",
			remain:   "",
			wantErr:  false,
		},

		// Error cases
		{
			name:     "missing terminator",
			input:    "hello",
			dir:      increasing,
			expected: "",
			remain:   "",
			wantErr:  true,
		},
		{
			name:     "invalid escape sequence",
			input:    "hello\x00\x02",
			dir:      increasing,
			expected: "",
			remain:   "",
			wantErr:  true,
		},
		{
			name:     "incomplete escape",
			input:    "hello\x00",
			dir:      increasing,
			expected: "",
			remain:   "",
			wantErr:  true,
		},
		{
			name:     "invalid 0xff escape",
			input:    "hello\xff\x02",
			dir:      increasing,
			expected: "",
			remain:   "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dst string
			remain, err := parseString(&dst, tt.input, tt.dir)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if dst != tt.expected {
					t.Errorf("parseString() got = %q, want %q", dst, tt.expected)
				}
				if remain != tt.remain {
					t.Errorf("parseString() remain = %q, want %q", remain, tt.remain)
				}
			}
		})
	}
}

func TestParse(t *testing.T) {

	tests := []struct {
		name      string
		encoded   string
		items     []interface{}
		remaining string
		wantErr   bool
		checkFn   func() bool // Optional custom verification
	}{
		// String tests
		{
			name:      "parse string",
			encoded:   "hello\x00\x01world",
			items:     []interface{}{new(string)},
			remaining: "world",
		},

		// TrailingString tests
		{
			name:      "parse trailing string",
			encoded:   "remainder",
			items:     []interface{}{new(TrailingString)},
			remaining: "",
		},
		{
			name:      "parse trailing string with dir",
			encoded:   "\x9a\x9a\x9a", // "abc" inverted
			items:     []interface{}{decr{new(TrailingString)}},
			remaining: "",
		},

		// Error cases
		{
			name:    "invalid type",
			encoded: "data",
			items:   []interface{}{new(bool)},
			wantErr: true,
		},
		{
			name:    "corrupt string",
			encoded: "hello\x00", // missing terminator
			items:   []interface{}{new(string)},
			wantErr: true,
		},
		{
			name:    "empty input for numeric",
			encoded: "",
			items:   []interface{}{new(int64)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make copies of the items for this test run
			items := make([]interface{}, len(tt.items))
			for i, item := range tt.items {
				switch v := item.(type) {
				case decr:
					items[i] = decr{clonePtr(v.val)}
				default:
					items[i] = clonePtr(item)
				}
			}

			remaining, err := Parse(tt.encoded, items...)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if remaining != tt.remaining {
					t.Errorf("Parse() remaining = %q, want %q", remaining, tt.remaining)
				}
			}
		})
	}
}

// // Helper functions

// type decr struct{ val interface{} }

func clonePtr(src interface{}) interface{} {
	switch v := src.(type) {
	case *string:
		var s string
		return &s
	case *TrailingString:
		var s TrailingString
		return &s
	case *float64:
		var f float64
		return &f
	case *int64:
		var i int64
		return &i
	case *uint64:
		var u uint64
		return &u
	default:
		return v
	}
}
