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

package collectiondecoder

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

const (
	Timestamp  int64 = 1704806400000000000
	Timestamp2 int64 = 1625247601000000000
)

func TestDecodeCollection(t *testing.T) {
	type args struct {
		dt      datatype.DataType
		version primitive.ProtocolVersion
		encoded []byte
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "Decode List of Bigints",
			args: args{
				dt:      datatype.NewListType(datatype.Bigint),
				version: primitive.ProtocolVersion4,
				encoded: []byte{0, 0, 0, 3, // list length: 3
					0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 1, // element 1: int64(1)
					0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 2, // element 2: int64(2)
					0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 3}, // element 3: int64(3)
			},
			want:    []int64{1, 2, 3},
			wantErr: false,
		},
		{
			name: "Decode Set of Bigints",
			args: args{
				dt:      datatype.NewSetType(datatype.Bigint),
				version: primitive.ProtocolVersion4,
				encoded: []byte{0, 0, 0, 3, // set length: 3
					0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 1, // element 1: int64(1)
					0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 2, // element 2: int64(2)
					0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 3}, // element 3: int64(3)
			},
			want:    []int64{1, 2, 3},
			wantErr: false,
		},
		{
			name: "Decode Map of Varchar to Bigint",
			args: args{
				dt:      datatype.NewMapType(datatype.Varchar, datatype.Bigint),
				version: primitive.ProtocolVersion4,
				encoded: []byte{0, 0, 0, 2, // map length: 2
					0, 0, 0, 3, 'o', 'n', 'e', // key 1: "one"
					0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 1, // value 1: int64(1)
					0, 0, 0, 6, 'n', 'e', 'w', 'k', 'e', 'y', // key 2: "two"
					0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 2}, // value 2: int64(2)
			},
			want:    map[string]int64{"one": 1, "newkey": 2},
			wantErr: false,
		},
		{
			name: "Decode List of Timestamps",
			args: args{
				dt:      datatype.NewListType(datatype.Bigint),
				version: primitive.ProtocolVersion4,
				encoded: []byte{
					0, 0, 0, 2, // List length: 2
					0, 0, 0, 8, 0, 0, 1, 142, 8, 246, 25, 208, // First timestamp (1709500000000 ms)
					0, 0, 0, 8, 0, 0, 1, 142, 9, 126, 248, 224, // Second timestamp (1709600000000 ms)
				},
			},
			want:    []int64{1709547330000, 1709556300000},
			wantErr: false,
		},
		{
			name: "Failure with Unsupported Type",
			args: args{
				dt: func() datatype.DataType {
					udt, _ := datatype.NewUserDefinedType("ks", "udt", nil, nil)
					return udt
				}(),
				version: primitive.ProtocolVersion4,
				encoded: []byte{0, 0, 0, 1, 0, 0, 0, 1, 'a'},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Failure with Invalid Encoded Data",
			args: args{
				dt:      datatype.NewListType(datatype.Bigint),
				version: primitive.ProtocolVersion4,
				encoded: []byte{0x01, 0x02},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Decode Empty List",
			args: args{
				dt:      datatype.NewListType(datatype.Bigint), // List of Bigints
				version: primitive.ProtocolVersion4,
				encoded: []byte{0, 0, 0, 0}, // List length = 0
			},
			want:    []int64{}, // Expecting an empty slice
			wantErr: false,
		},
		{
			name: "Decode Empty Set",
			args: args{
				dt:      datatype.NewSetType(datatype.Varchar), // Set of Varchar
				version: primitive.ProtocolVersion4,
				encoded: []byte{0, 0, 0, 0}, // Set length = 0
			},
			want:    []string{}, // Expecting an empty slice
			wantErr: false,
		},
		{
			name: "Decode Empty Map",
			args: args{
				dt:      datatype.NewMapType(datatype.Varchar, datatype.Bigint), // Map<Varchar, Bigint>
				version: primitive.ProtocolVersion4,
				encoded: []byte{0, 0, 0, 0}, // Map length = 0
			},
			want:    map[string]int64{}, // Expecting an empty map
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeCollection(tt.args.dt, tt.args.version, tt.args.encoded)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeCollection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DecodeCollection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecodeListOrSet(t *testing.T) {
	type args struct {
		elementType datatype.DataType
		version     primitive.ProtocolVersion
		reader      *bytes.Reader
		length      int32
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "Decode List of Integers",
			args: args{
				elementType: datatype.Int, // CQL 'int' is a 4-byte integer
				version:     primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 4, 0, 0, 0, 1, // int32(1)
					0, 0, 0, 4, 0, 0, 0, 2, // int32(2)
					0, 0, 0, 4, 0, 0, 0, 3, // int32(3)
				}),
				length: 3,
			},
			want:    []int32{1, 2, 3}, // Expecting int32 values
			wantErr: false,
		},
		{
			name: "Decode List of Bigints",
			args: args{
				elementType: datatype.Bigint,
				version:     primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 1, // int64(1)
					0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 2, // int64(2)
					0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 3}), // int64(3)
				length: 3,
			},
			want:    []int64{1, 2, 3},
			wantErr: false,
		},
		{
			name: "Decode List of Varchars",
			args: args{
				elementType: datatype.Varchar,
				version:     primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 5, 'h', 'e', 'l', 'l', 'o', // "hello"
					0, 0, 0, 5, 'w', 'o', 'r', 'l', 'd'}), // "world"
				length: 2,
			},
			want:    []string{"hello", "world"},
			wantErr: false,
		},
		{
			name: "Decode List of Timestamps",
			args: args{
				elementType: datatype.Bigint,
				version:     primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 8, 0, 0, 1, 142, 8, 246, 25, 208, // First timestamp (1709500000000 ms)
					0, 0, 0, 8, 0, 0, 1, 142, 9, 126, 248, 224, // Second timestamp (1709600000000 ms)
				}),
				length: 2,
			},
			want:    []int64{1709547330000, 1709556300000},
			wantErr: false,
		},
		{
			name: "Decode List of Booleans",
			args: args{
				elementType: datatype.Boolean,
				version:     primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 1, 0x01, // true
					0, 0, 0, 1, 0x00, // false
					0, 0, 0, 1, 0x01, // true
				}),
				length: 3,
			},
			want:    []bool{true, false, true},
			wantErr: false,
		},
		{
			name: "Decode List of Floats",
			args: args{
				elementType: datatype.Float, // Assuming datatype.Float is float32
				version:     primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 4, 0x3F, 0x80, 0x00, 0x00, // float 1.0
					0, 0, 0, 4, 0x40, 0x00, 0x00, 0x00, // float 2.0
					0, 0, 0, 4, 0x40, 0x40, 0x00, 0x00, // float 3.0
				}),
				length: 3,
			},
			want:    []float32{1.0, 2.0, 3.0},
			wantErr: false,
		},
		{
			name: "Decode List of Float64 (Double)",
			args: args{
				elementType: datatype.Double, // Assuming datatype.Double is float64
				version:     primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 8, 0x3F, 0xF0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // double 1.0
					0, 0, 0, 8, 0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // double 2.0
					0, 0, 0, 8, 0x40, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // double 3.0
				}),
				length: 3,
			},
			want:    []float64{1.0, 2.0, 3.0},
			wantErr: false,
		},
		{
			name: "Decode Set of Integers",
			args: args{
				elementType: datatype.Int, // CQL 'int' is a 4-byte integer
				version:     primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 4, 0, 0, 0, 1, // int32(1)
					0, 0, 0, 4, 0, 0, 0, 2, // int32(2)
					0, 0, 0, 4, 0, 0, 0, 3, // int32(3)
				}),
				length: 3,
			},
			want:    []int32{1, 2, 3}, // Expecting int32 values
			wantErr: false,
		},
		{
			name: "Decode Set of Bigints",
			args: args{
				elementType: datatype.Bigint,
				version:     primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 1, // int64(1)
					0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 2, // int64(2)
					0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 3}), // int64(3)
				length: 3,
			},
			want:    []int64{1, 2, 3}, // Expecting int64 values
			wantErr: false,
		},
		{
			name: "Decode Set of Varchars",
			args: args{
				elementType: datatype.Varchar,
				version:     primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 5, 'h', 'e', 'l', 'l', 'o', // "hello"
					0, 0, 0, 5, 'w', 'o', 'r', 'l', 'd'}), // "world"
				length: 2,
			},
			want:    []string{"hello", "world"},
			wantErr: false,
		},
		{
			name: "Decode Set of Timestamps",
			args: args{
				elementType: datatype.Bigint,
				version:     primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 8, 0, 0, 1, 142, 8, 246, 25, 208, // First timestamp (1709500000000 ms)
					0, 0, 0, 8, 0, 0, 1, 142, 9, 126, 248, 224, // Second timestamp (1709600000000 ms)
				}),
				length: 2,
			},
			want:    []int64{1709547330000, 1709556300000},
			wantErr: false,
		},
		{
			name: "Decode Set of Booleans",
			args: args{
				elementType: datatype.Boolean,
				version:     primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 1, 0x01, // true
					0, 0, 0, 1, 0x00, // false
				}),
				length: 2,
			},
			want:    []bool{true, false}, // SET should remove duplicates
			wantErr: false,
		},
		{
			name: "Decode Set of Floats",
			args: args{
				elementType: datatype.Float, // Assuming datatype.Float is float32
				version:     primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 4, 0x3F, 0x80, 0x00, 0x00, // float 1.0
					0, 0, 0, 4, 0x40, 0x00, 0x00, 0x00, // float 2.0
					0, 0, 0, 4, 0x40, 0x40, 0x00, 0x00, // float 3.0
				}),
				length: 3,
			},
			want:    []float32{1.0, 2.0, 3.0},
			wantErr: false,
		},
		{
			name: "Decode Set of Float64 (Double)",
			args: args{
				elementType: datatype.Double, // Assuming datatype.Double is float64
				version:     primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 8, 0x3F, 0xF0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // double 1.0
					0, 0, 0, 8, 0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // double 2.0
					0, 0, 0, 8, 0x40, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // double 3.0
				}),
				length: 3,
			},
			want:    []float64{1.0, 2.0, 3.0},
			wantErr: false,
		},
		{
			name: "Failure with Invalid Length",
			args: args{
				elementType: datatype.Bigint,
				version:     primitive.ProtocolVersion4,
				reader:      bytes.NewReader([]byte{0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 1}),
				length:      2,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Failure with Invalid Encoded Data",
			args: args{
				elementType: datatype.Bigint,
				version:     primitive.ProtocolVersion4,
				reader:      bytes.NewReader([]byte{0x01, 0x02}),
				length:      1,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeListOrSet(tt.args.elementType, tt.args.version, tt.args.reader, tt.args.length)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeListOrSet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decodeListOrSet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertToTypedSlice(t *testing.T) {
	type args struct {
		decodedElements []interface{}
		dt              datatype.DataType
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "Convert Empty Slice of Bigints",
			args: args{
				decodedElements: []interface{}{},
				dt:              datatype.Bigint,
			},
			want:    []int64{},
			wantErr: false,
		},
		{
			name: "Convert List of Uuids",
			args: args{
				decodedElements: []interface{}{"550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001"},
				dt:              datatype.Uuid,
			},
			want:    []string{"550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001"},
			wantErr: false,
		},
		{
			name: "Convert List of Timeuuid",
			args: args{
				decodedElements: []interface{}{"ddeb27fb-d9a0-4b10-9c6a-6f5f9afed0c2", "ddeb27fb-d9a0-4b10-9c6a-6f5f9afed0c3"},
				dt:              datatype.Timeuuid,
			},
			want:    []string{"ddeb27fb-d9a0-4b10-9c6a-6f5f9afed0c2", "ddeb27fb-d9a0-4b10-9c6a-6f5f9afed0c3"},
			wantErr: false,
		},
		{
			name: "Convert List of Timestamps",
			args: args{
				decodedElements: []interface{}{int64(1627846261000), int64(1627846262000)},
				dt:              datatype.Timestamp,
			},
			want:    []int64{1627846261000, 1627846262000},
			wantErr: false,
		},
		{
			name: "Convert List of Booleans",
			args: args{
				decodedElements: []interface{}{true, false, true},
				dt:              datatype.Boolean,
			},
			want:    []bool{true, false, true},
			wantErr: false,
		},
		{
			name: "Convert List of Doubles",
			args: args{
				decodedElements: []interface{}{float64(1.1), float64(2.2), float64(3.3)},
				dt:              datatype.Double,
			},
			want:    []float64{1.1, 2.2, 3.3},
			wantErr: false,
		},
		{
			name: "Convert List of Floats",
			args: args{
				decodedElements: []interface{}{float32(1.1), float32(2.2), float32(3.3)},
				dt:              datatype.Float,
			},
			want:    []float32{1.1, 2.2, 3.3},
			wantErr: false,
		},
		{
			name: "Convert List of ASCII Strings",
			args: args{
				decodedElements: []interface{}{"Hello", "World"},
				dt:              datatype.Ascii,
			},
			want:    []string{"Hello", "World"},
			wantErr: false,
		},
		{
			name: "Convert List of Blobs",
			args: args{
				decodedElements: []interface{}{[]byte{0x01, 0x02}, []byte{0x03, 0x04}},
				dt:              datatype.Blob,
			},
			want:    [][]byte{{0x01, 0x02}, {0x03, 0x04}},
			wantErr: false,
		},
		{
			name: "Convert List of Varchar",
			args: args{
				decodedElements: []interface{}{"string1", "string2", "string3"},
				dt:              datatype.Varchar,
			},
			want:    []string{"string1", "string2", "string3"},
			wantErr: false,
		},
		{
			name: "Convert List of Smallints",
			args: args{
				decodedElements: []interface{}{int16(100), int16(200), int16(300)},
				dt:              datatype.Smallint,
			},
			want:    []int16{100, 200, 300},
			wantErr: false,
		},
		{
			name: "Convert Empty List of Smallints",
			args: args{
				decodedElements: []interface{}{},
				dt:              datatype.Smallint,
			},
			want:    []int16{},
			wantErr: false,
		},
		{
			name: "Convert List of Int32s",
			args: args{
				decodedElements: []interface{}{int32(100), int32(200), int32(300)},
				dt:              datatype.Int,
			},
			want:    []int32{100, 200, 300},
			wantErr: false,
		},
		{
			name: "Convert List of Decimals",
			args: args{
				decodedElements: []interface{}{"123.45", "678.90", "123456789.0123456789"},
				dt:              datatype.Decimal,
			},
			want:    []string{"123.45", "678.90", "123456789.0123456789"},
			wantErr: false,
		},
		{
			name: "Convert List of Varints",
			args: args{
				decodedElements: []interface{}{"123", "456789", "-123456789"},
				dt:              datatype.Varint,
			},
			want:    []string{"123", "456789", "-123456789"},
			wantErr: false,
		},
		{
			name: "Convert List of Tinyints",
			args: args{
				decodedElements: []interface{}{int8(1), int8(2), int8(3)},
				dt:              datatype.Tinyint,
			},
			want:    []int8{1, 2, 3},
			wantErr: false,
		},
		{
			name: "Convert List of Durations",
			args: args{
				decodedElements: []interface{}{int64(1000), int64(2000), int64(3000)},
				dt:              datatype.Duration,
			},
			want:    []int64{1000, 2000, 3000},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertToTypedSlice(tt.args.decodedElements, tt.args.dt)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertToTypedSlice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConvertToTypedSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertToTypedMap(t *testing.T) {
	type args struct {
		decodedMap map[interface{}]interface{}
		keyType    datatype.DataType
		valueType  datatype.DataType
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "Convert Map of Varchar to Bigint",
			args: args{
				decodedMap: map[interface{}]interface{}{"one": int64(1), "two": int64(2)},
				keyType:    datatype.Varchar,
				valueType:  datatype.Bigint,
			},
			want:    map[string]int64{"one": 1, "two": 2},
			wantErr: false,
		},
		{
			name: "Convert Map of Varchar to Int",
			args: args{
				decodedMap: map[interface{}]interface{}{"one": int32(1), "two": int32(2)},
				keyType:    datatype.Varchar,
				valueType:  datatype.Int,
			},
			want:    map[string]int32{"one": 1, "two": 2},
			wantErr: false,
		},
		{
			name: "Convert Map of Varchar to Float",
			args: args{
				decodedMap: map[interface{}]interface{}{"one": float32(1.1), "two": float32(2.2)},
				keyType:    datatype.Varchar,
				valueType:  datatype.Float,
			},
			want:    map[string]float32{"one": 1.1, "two": 2.2},
			wantErr: false,
		},
		{
			name: "Convert Map of Varchar to Double",
			args: args{
				decodedMap: map[interface{}]interface{}{"one": float64(1.1), "two": float64(2.2)},
				keyType:    datatype.Varchar,
				valueType:  datatype.Double,
			},
			want:    map[string]float64{"one": 1.1, "two": 2.2},
			wantErr: false,
		},
		{
			name: "Convert Map of Varchar to Boolean",
			args: args{
				decodedMap: map[interface{}]interface{}{"one": true, "two": false},
				keyType:    datatype.Varchar,
				valueType:  datatype.Boolean,
			},
			want:    map[string]bool{"one": true, "two": false},
			wantErr: false,
		},
		{
			name: "Convert Map of Varchar to Timestamp",
			args: args{
				decodedMap: map[interface{}]interface{}{"one": int64(1625247600000), "two": int64(1625247601000)},
				keyType:    datatype.Varchar,
				valueType:  datatype.Timestamp,
			},
			want:    map[string]int64{"one": 1625247600000, "two": 1625247601000},
			wantErr: false,
		},
		{
			name: "Convert Map of Varchar to Uuid",
			args: args{
				decodedMap: map[interface{}]interface{}{"one": "550e8400-e29b-41d4-a716-446655440000"},
				keyType:    datatype.Varchar,
				valueType:  datatype.Uuid,
			},
			want:    map[string]string{"one": "550e8400-e29b-41d4-a716-446655440000"},
			wantErr: false,
		},
		{
			name: "Convert Map of Varchar to Blob",
			args: args{
				decodedMap: map[interface{}]interface{}{"one": []byte{0x01, 0x02}},
				keyType:    datatype.Varchar,
				valueType:  datatype.Blob,
			},
			want:    map[string][]byte{"one": {0x01, 0x02}},
			wantErr: false,
		},
		{
			name: "Failure with Unsupported Key Type",
			args: args{
				decodedMap: map[interface{}]interface{}{1: 1},
				keyType: func() datatype.DataType {
					udt, _ := datatype.NewUserDefinedType("ks", "udt", nil, nil)
					return udt
				}(),
				valueType: datatype.Bigint,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Failure with Unsupported Value Type",
			args: args{
				decodedMap: map[interface{}]interface{}{"one": 1},
				keyType:    datatype.Varchar,
				valueType: func() datatype.DataType {
					udt, _ := datatype.NewUserDefinedType("ks", "udt", nil, nil)
					return udt
				}(),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Convert Map of Varchar to Tinyint",
			args: args{
				decodedMap: map[interface{}]interface{}{"one": int8(1), "two": int8(2)},
				keyType:    datatype.Varchar,
				valueType:  datatype.Tinyint,
			},
			want:    map[string]int8{"one": 1, "two": 2},
			wantErr: false,
		},
		{
			name: "Convert Map of Varchar to Smallint",
			args: args{
				decodedMap: map[interface{}]interface{}{"one": int16(1), "two": int16(2)},
				keyType:    datatype.Varchar,
				valueType:  datatype.Smallint,
			},
			want:    map[string]int16{"one": 1, "two": 2},
			wantErr: false,
		},
		{
			name: "Convert Map of Varchar to Duration",
			args: args{
				decodedMap: map[interface{}]interface{}{"one": int64(3155760000), "two": int64(6311520000)},
				keyType:    datatype.Varchar,
				valueType:  datatype.Duration,
			},
			want:    map[string]int64{"one": 3155760000, "two": 6311520000},
			wantErr: false,
		},
		{
			name: "Convert Map of Varchar to Decimal",
			args: args{
				decodedMap: map[interface{}]interface{}{"one": "123.45", "two": "678.90"},
				keyType:    datatype.Varchar,
				valueType:  datatype.Decimal,
			},
			want:    map[string]string{"one": "123.45", "two": "678.90"},
			wantErr: false,
		},
		{
			name: "Convert Map of Timestamp to Varchar with invalid key type",
			args: args{
				decodedMap: map[interface{}]interface{}{
					"incorrect type": "a string",
				},
				keyType:   datatype.Timestamp,
				valueType: datatype.Varchar,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Convert Map of Timestamp to Boolean",
			args: args{
				decodedMap: map[interface{}]interface{}{
					time.Unix(0, Timestamp): true,
				},
				keyType:   datatype.Timestamp,
				valueType: datatype.Boolean,
			},
			want: map[time.Time]bool{
				time.Unix(0, Timestamp): true, // Adjust time as needed
			},
			wantErr: false,
		},
		{
			name: "Convert Map with Empty Input",
			args: args{
				decodedMap: map[interface{}]interface{}{},
				keyType:    datatype.Varchar,
				valueType:  datatype.Bigint,
			},
			want:    map[string]int64{},
			wantErr: false,
		},
		{
			name: "Unsupported Key Type for Timestamp conversion",
			args: args{
				decodedMap: map[interface{}]interface{}{1: "value"},
				keyType:    datatype.Timestamp,
				valueType:  datatype.Varchar,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Convert Map of Timestamp to Varchar",
			args: args{
				decodedMap: map[interface{}]interface{}{
					time.Unix(0, Timestamp):  "event_one",
					time.Unix(0, Timestamp2): "event_two",
				},
				keyType:   datatype.Timestamp,
				valueType: datatype.Varchar,
			},
			want: map[time.Time]string{
				time.Unix(0, Timestamp):  "event_one",
				time.Unix(0, Timestamp2): "event_two",
			},
			wantErr: false,
		},
		{
			name: "Convert Map of Timestamp to Boolean with Invalid Timestamp",
			args: args{
				decodedMap: map[interface{}]interface{}{
					"incorrect type": true,
				},
				keyType:   datatype.Timestamp,
				valueType: datatype.Boolean,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Convert Map of Timestamp to Boolean",
			args: args{
				decodedMap: map[interface{}]interface{}{
					time.Unix(0, Timestamp):  true,
					time.Unix(0, Timestamp2): false,
				},
				keyType:   datatype.Timestamp,
				valueType: datatype.Boolean,
			},
			want: map[time.Time]bool{
				time.Unix(0, Timestamp):  true,
				time.Unix(0, Timestamp2): false,
			},
			wantErr: false,
		},
		{
			name: "Convert Map with Empty Input",
			args: args{
				decodedMap: map[interface{}]interface{}{},
				keyType:    datatype.Varchar,
				valueType:  datatype.Bigint,
			},
			want:    map[string]int64{},
			wantErr: false,
		},
		{
			name: "Unsupported Key Type for Timestamp conversion",
			args: args{
				decodedMap: map[interface{}]interface{}{1: "value"},
				keyType:    datatype.Timestamp,
				valueType:  datatype.Varchar,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Convert Map with Unsupported Value Type",
			args: args{
				decodedMap: map[interface{}]interface{}{"one": "value"},
				keyType:    datatype.Varchar,
				valueType: func() datatype.DataType {
					udt, _ := datatype.NewUserDefinedType("ks", "udt", nil, nil)
					return udt
				}(),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Convert Map of Timestamp to Int with Invalid Value Type",
			args: args{
				decodedMap: map[interface{}]interface{}{
					time.Unix(0, Timestamp): "not an int",
				},
				keyType:   datatype.Timestamp,
				valueType: datatype.Int,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Convert Map of Timestamp to Bigint",
			args: args{
				decodedMap: map[interface{}]interface{}{
					time.Unix(0, Timestamp):  int64(100),
					time.Unix(0, Timestamp2): int64(200),
				},
				keyType:   datatype.Timestamp,
				valueType: datatype.Bigint,
			},
			want: map[time.Time]int64{
				time.Unix(0, Timestamp):  100,
				time.Unix(0, Timestamp2): 200,
			},
			wantErr: false,
		},
		{
			name: "Convert Map of Timestamp to Bigint with Invalid Key Type",
			args: args{
				decodedMap: map[interface{}]interface{}{
					"not a timestamp": int64(100),
				},
				keyType:   datatype.Timestamp,
				valueType: datatype.Bigint,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Convert Map of Timestamp to Bigint with Invalid Value Type",
			args: args{
				decodedMap: map[interface{}]interface{}{
					time.Unix(0, Timestamp): "not an int64",
				},
				keyType:   datatype.Timestamp,
				valueType: datatype.Bigint,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Convert Map of Timestamp to Float",
			args: args{
				decodedMap: map[interface{}]interface{}{
					time.Unix(0, Timestamp):  float32(3.14),
					time.Unix(0, Timestamp2): float32(2.71),
				},
				keyType:   datatype.Timestamp,
				valueType: datatype.Float,
			},
			want: map[time.Time]float32{
				time.Unix(0, Timestamp):  3.14,
				time.Unix(0, Timestamp2): 2.71,
			},
			wantErr: false,
		},
		{
			name: "Convert Map of Timestamp to Float with Invalid Key Type",
			args: args{
				decodedMap: map[interface{}]interface{}{
					"not a timestamp": float32(1.23),
				},
				keyType:   datatype.Timestamp,
				valueType: datatype.Float,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Convert Map of Timestamp to Float with Invalid Value Type",
			args: args{
				decodedMap: map[interface{}]interface{}{
					time.Unix(0, Timestamp): "not a float32",
				},
				keyType:   datatype.Timestamp,
				valueType: datatype.Float,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Convert Empty Map of Timestamp to Float",
			args: args{
				decodedMap: map[interface{}]interface{}{},
				keyType:    datatype.Timestamp,
				valueType:  datatype.Float,
			},
			want:    map[time.Time]float32{},
			wantErr: false,
		},
		{
			name: "Convert Map of Timestamp to Double",
			args: args{
				decodedMap: map[interface{}]interface{}{
					time.Unix(0, Timestamp):  float64(3.1415926535),
					time.Unix(0, Timestamp2): float64(2.7182818284),
				},
				keyType:   datatype.Timestamp,
				valueType: datatype.Double,
			},
			want: map[time.Time]float64{
				time.Unix(0, Timestamp):  3.1415926535,
				time.Unix(0, Timestamp2): 2.7182818284,
			},
			wantErr: false,
		},
		{
			name: "Convert Map of Timestamp to Double with Invalid Key Type",
			args: args{
				decodedMap: map[interface{}]interface{}{
					"not a timestamp": float64(1.23),
				},
				keyType:   datatype.Timestamp,
				valueType: datatype.Double,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Convert Map of Timestamp to Double with Invalid Value Type",
			args: args{
				decodedMap: map[interface{}]interface{}{
					time.Unix(0, Timestamp): "not a float64",
				},
				keyType:   datatype.Timestamp,
				valueType: datatype.Double,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Convert Empty Map of Timestamp to Double",
			args: args{
				decodedMap: map[interface{}]interface{}{},
				keyType:    datatype.Timestamp,
				valueType:  datatype.Double,
			},
			want:    map[time.Time]float64{},
			wantErr: false,
		},
		{
			name: "Convert Map of Timestamp to Timestamp",
			args: args{
				decodedMap: map[interface{}]interface{}{
					time.Unix(0, Timestamp):  time.Unix(0, 1625247605000000000),
					time.Unix(0, Timestamp2): time.Unix(0, 1625247606000000000),
				},
				keyType:   datatype.Timestamp,
				valueType: datatype.Timestamp,
			},
			want: map[time.Time]time.Time{
				time.Unix(0, Timestamp):  time.Unix(0, 1625247605000000000),
				time.Unix(0, Timestamp2): time.Unix(0, 1625247606000000000),
			},
			wantErr: false,
		},
		{
			name: "Convert Map of Timestamp to Timestamp with Invalid Key Type",
			args: args{
				decodedMap: map[interface{}]interface{}{
					"not a timestamp": time.Unix(0, 1625247605000000000),
				},
				keyType:   datatype.Timestamp,
				valueType: datatype.Timestamp,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Convert Map of Timestamp to Timestamp with Invalid Value Type",
			args: args{
				decodedMap: map[interface{}]interface{}{
					time.Unix(0, Timestamp): "not a time.Time",
				},
				keyType:   datatype.Timestamp,
				valueType: datatype.Timestamp,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Convert Empty Map of Timestamp to Timestamp",
			args: args{
				decodedMap: map[interface{}]interface{}{},
				keyType:    datatype.Timestamp,
				valueType:  datatype.Timestamp,
			},
			want:    map[time.Time]time.Time{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertToTypedMap(tt.args.decodedMap, tt.args.keyType, tt.args.valueType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertToTypedMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConvertToTypedMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecodeMap(t *testing.T) {
	type args struct {
		valueType datatype.DataType
		version   primitive.ProtocolVersion
		reader    *bytes.Reader
		keyType   datatype.DataType
		length    int32
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "Decode Map of Text to Bigint",
			args: args{
				valueType: datatype.Bigint,
				version:   primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 3, 'o', 'n', 'e',
					0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 1,
					0, 0, 0, 3, 't', 'w', 'o',
					0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 2,
				}),
				keyType: datatype.Varchar,
				length:  2,
			},
			want:    map[string]int64{"one": 1, "two": 2},
			wantErr: false,
		},
		{
			name: "Decode Map of Timestamp to Float",
			args: args{
				valueType: datatype.Float,
				version:   primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 8, 0, 0, 1, 142, 8, 246, 25, 208,
					0, 0, 0, 4, 0x3F, 0x80, 0x00, 0x00,
					0, 0, 0, 8, 0, 0, 1, 142, 9, 126, 248, 224,
					0, 0, 0, 4, 0x40, 0x00, 0x00, 0x00,
				}),
				keyType: datatype.Timestamp,
				length:  2,
			},
			want: map[time.Time]float32{
				time.UnixMilli(1709547330000).UTC(): 1.0,
				time.UnixMilli(1709556300000).UTC(): 2.0,
			},
			wantErr: false,
		},
		{
			name: "Decode Map of Text to Boolean",
			args: args{
				valueType: datatype.Boolean,
				version:   primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 3, 'a', 'c', 't',
					0, 0, 0, 1, 0x01,
					0, 0, 0, 4, 'f', 'a', 'l', 's',
					0, 0, 0, 1, 0x00,
				}),
				keyType: datatype.Varchar,
				length:  2,
			},
			want:    map[string]bool{"act": true, "fals": false},
			wantErr: false,
		},
		{
			name: "Decode Map of Text to Double",
			args: args{
				valueType: datatype.Double,
				version:   primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 3, 'k', 'e', 'y',
					0, 0, 0, 8, 0x3F, 0xF0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
					0, 0, 0, 3, 'n', 'u', 'm',
					0, 0, 0, 8, 0x40, 0x09, 0x21, 0xF9, 0xF0, 0x1B, 0x86, 0x6E,
				}),
				keyType: datatype.Varchar,
				length:  2,
			},
			want:    map[string]float64{"key": 1.0, "num": 3.14159},
			wantErr: false,
		},
		{
			name: "Decode Empty Map",
			args: args{
				valueType: datatype.Bigint,
				version:   primitive.ProtocolVersion4,
				reader:    bytes.NewReader([]byte{}), // Empty map
				keyType:   datatype.Varchar,
				length:    0,
			},
			want:    map[string]int64{},
			wantErr: false,
		},
		{
			name: "Decode Map with Duplicate Keys",
			args: args{
				valueType: datatype.Bigint,
				version:   primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 3, 'a', 'a', 'a',
					0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 5,
					0, 0, 0, 3, 'a', 'a', 'a',
					0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 9,
				}),
				keyType: datatype.Varchar,
				length:  2,
			},
			want:    map[string]int64{"aaa": 9}, // Last value should overwrite previous one
			wantErr: false,
		},
		{
			name: "Failure with Incomplete Encoded Map Data",
			args: args{
				valueType: datatype.Bigint,
				version:   primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 3, 'o', 'n', 'e',
					0, 0, 0, 8,
				}),
				keyType: datatype.Varchar,
				length:  1,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Failure with Corrupt Encoded Data",
			args: args{
				valueType: datatype.Boolean,
				version:   primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 4, 'b', 'a', 'd', '!',
					0, 0, 0, 2, 0x03, 0x07, // Wrong boolean encoding (should be 1 byte)
				}),
				keyType: datatype.Varchar,
				length:  1,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Failure with Partial Key",
			args: args{
				valueType: datatype.Float,
				version:   primitive.ProtocolVersion4,
				reader: bytes.NewReader([]byte{
					0, 0, 0, 2, 't', 'e', // Truncated key
				}),
				keyType: datatype.Varchar,
				length:  1,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeMap(tt.args.valueType, tt.args.version, tt.args.reader, tt.args.keyType, tt.args.length)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decodeMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
