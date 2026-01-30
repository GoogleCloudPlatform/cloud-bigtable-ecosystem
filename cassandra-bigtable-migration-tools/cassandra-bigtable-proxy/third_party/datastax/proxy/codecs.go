// Copyright (c) DataStax, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
)

var codec = frame.NewRawCodec(&partialQueryCodec{}, &partialExecuteCodec{}, &partialBatchCodec{})

type partialQueryCodec struct{}

func (c *partialQueryCodec) Encode(_ message.Message, _ io.Writer, _ primitive.ProtocolVersion) error {
	panic("not implemented")
}

func (c *partialQueryCodec) EncodedLength(_ message.Message, _ primitive.ProtocolVersion) (int, error) {
	panic("not implemented")
}

func (c *partialQueryCodec) Decode(source io.Reader, _ primitive.ProtocolVersion) (message.Message, error) {
	if query, err := primitive.ReadLongString(source); err != nil {
		return nil, err
	} else {
		return &partialQuery{query}, nil
	}
}

func (c *partialQueryCodec) GetOpCode() primitive.OpCode {
	return primitive.OpCodeQuery
}

type partialQuery struct {
	query string
}

func (p *partialQuery) IsResponse() bool {
	return false
}

func (p *partialQuery) GetOpCode() primitive.OpCode {
	return primitive.OpCodeQuery
}

func (p *partialQuery) Clone() message.Message {
	return &partialQuery{p.query}
}

type partialExecute struct {
	queryId []byte
	// The positional values of the associated query. Positional values are designated in query strings with a
	// question mark bind marker ('?'). Positional values are valid for all protocol versions.
	// It is illegal to use both positional and named values at the same time. If this happens, positional values will
	// be used and named values will be silently ignored.
	PositionalValues []*primitive.Value

	// The named values of the associated query. Named values are designated in query strings with a
	// a bind marker starting with a colon (e.g. ':var1'). Named values can only be used with protocol version 3 or
	// higher.
	// It is illegal to use both positional and named values at the same time. If this happens, positional values will
	// be used and named values will be silently ignored.
	NamedValues map[string]*primitive.Value
}

func (m *partialExecute) IsResponse() bool {
	return false
}

func (m *partialExecute) GetOpCode() primitive.OpCode {
	return primitive.OpCodeExecute
}

func (m *partialExecute) Clone() message.Message {
	return &partialExecute{
		queryId: primitive.CloneByteSlice(m.queryId),
	}
}

func (m *partialExecute) String() string {
	return "EXECUTE " + hex.EncodeToString(m.queryId)
}

type partialExecuteCodec struct{}

func (c *partialExecuteCodec) Encode(_ message.Message, _ io.Writer, _ primitive.ProtocolVersion) error {
	panic("not implemented")
}

func (c *partialExecuteCodec) EncodedLength(_ message.Message, _ primitive.ProtocolVersion) (size int, err error) {
	panic("not implemented")
}

func (c *partialExecuteCodec) Decode(source io.Reader, protocolV primitive.ProtocolVersion) (msg message.Message, err error) {
	execute := &partialExecute{}
	if execute.queryId, err = primitive.ReadShortBytes(source); err != nil {
		return nil, fmt.Errorf("cannot read EXECUTE query id: %w", err)
	} else if len(execute.queryId) == 0 {
		return nil, errors.New("EXECUTE missing query id")
	}
	options, err := message.DecodeQueryOptions(source, protocolV)
	execute.PositionalValues = options.PositionalValues
	execute.NamedValues = options.NamedValues
	if err != nil {
		err = fmt.Errorf("error while decoding bytes for values : %s ", err.Error())
	}
	return execute, err
}

func (c *partialExecuteCodec) GetOpCode() primitive.OpCode {
	return primitive.OpCodeExecute
}

type partialBatch struct {
	queryOrIds []interface{}
	// The positional values of the associated query. Positional values are designated in query strings with a
	// question mark bind marker ('?'). Positional values are valid for all protocol versions.
	// It is illegal to use both positional and named values at the same time. If this happens, positional values will
	// be used and named values will be silently ignored.
	BatchPositionalValues [][]*primitive.Value
}

func (p partialBatch) IsResponse() bool {
	return false
}

func (p partialBatch) GetOpCode() primitive.OpCode {
	return primitive.OpCodeBatch
}

func (p partialBatch) Clone() message.Message {
	queryOrIds := make([]interface{}, len(p.queryOrIds))
	copy(queryOrIds, p.queryOrIds)
	PositionalValues := make([][]*primitive.Value, len(p.BatchPositionalValues))
	copy(PositionalValues, p.BatchPositionalValues)
	return &partialBatch{queryOrIds, PositionalValues}
}

type partialBatchCodec struct{}

func (p partialBatchCodec) Encode(msg message.Message, dest io.Writer, version primitive.ProtocolVersion) error {
	panic("not implemented")
}

func (p partialBatchCodec) EncodedLength(msg message.Message, version primitive.ProtocolVersion) (int, error) {
	panic("not implemented")
}

func (p partialBatchCodec) Decode(source io.Reader, version primitive.ProtocolVersion) (msg message.Message, err error) {
	var partialBatchExecute = &partialBatch{}
	var positionalValues []*primitive.Value
	var queryOrIds []interface{}
	var typ uint8
	if typ, err = primitive.ReadByte(source); err != nil {
		return nil, fmt.Errorf("cannot read BATCH type: %w", err)
	}
	if err = primitive.CheckValidBatchType(primitive.BatchType(typ)); err != nil {
		return nil, err
	}
	var count uint16
	if count, err = primitive.ReadShort(source); err != nil {
		return nil, fmt.Errorf("cannot read BATCH query count: %w", err)
	}
	queryOrIds = make([]interface{}, count)
	for i := 0; i < int(count); i++ {
		var queryTyp uint8
		if queryTyp, err = primitive.ReadByte(source); err != nil {
			return nil, fmt.Errorf("cannot read BATCH query type for child #%d: %w", i, err)
		}
		var queryOrId interface{}
		switch primitive.BatchChildType(queryTyp) {
		case primitive.BatchChildTypeQueryString:
			if queryOrId, err = primitive.ReadLongString(source); err != nil {
				return nil, fmt.Errorf("cannot read BATCH query string for child #%d: %w", i, err)
			}
		case primitive.BatchChildTypePreparedId:
			if queryOrId, err = primitive.ReadShortBytes(source); err != nil {
				return nil, fmt.Errorf("cannot read BATCH query id for child #%d: %w", i, err)
			}
		default:
			return nil, fmt.Errorf("unsupported BATCH child type for child #%d: %v", i, queryTyp)
		}
		if positionalValues, err = getPositionalValues(source); err != nil {
			return nil, fmt.Errorf("cannot read BATCH positional values for child #%d: %w", i, err)
		} else {
			partialBatchExecute.BatchPositionalValues = append(partialBatchExecute.BatchPositionalValues, positionalValues)
		}
		queryOrIds[i] = queryOrId
	}
	partialBatchExecute.queryOrIds = queryOrIds
	return partialBatchExecute, nil
}

func (p partialBatchCodec) GetOpCode() primitive.OpCode {
	return primitive.OpCodeBatch
}

func getPositionalValues(source io.Reader) ([]*primitive.Value, error) {
	var positionalValues []*primitive.Value
	if length, err := primitive.ReadShort(source); err != nil {
		return nil, fmt.Errorf("cannot read positional [value]s length: %w", err)
	} else {
		for i := uint16(0); i < length; i++ {
			if value, err := getParamValue(source); err != nil {
				return nil, fmt.Errorf("cannot read positional [value]s element %d content: %w", i, err)
			} else {
				positionalValues = append(positionalValues, value)
			}
		}
		return positionalValues, nil
	}
}

func getParamValue(source io.Reader) (*primitive.Value, error) {
	if length, err := primitive.ReadInt(source); err != nil {
		return nil, fmt.Errorf("cannot read [value] length: %w", err)
	} else if length < 0 {
		return primitive.NewValue(nil), nil
	} else if length == 0 {
		return primitive.NewValue([]byte{}), nil
	} else {
		buf := make([]byte, length)
		_, err := io.ReadFull(source, buf)
		if err != nil {
			return nil, fmt.Errorf("cannot read [value] content: %w", err)
		}
		return primitive.NewValue(buf), nil
	}
}
