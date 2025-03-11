// Copyright (c) DataStax, Inc.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//      http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datastax/go-cassandra-native-protocol/datatype"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	bt "github.com/ollionorg/cassandra-to-bigtable-proxy/bigtable"
	otelgo "github.com/ollionorg/cassandra-to-bigtable-proxy/otel"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/proxycore"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/responsehandler"
	schemaMapping "github.com/ollionorg/cassandra-to-bigtable-proxy/schema-mapping"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/translator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

func TestConnect(t *testing.T) {
	var logger *zap.Logger
	logger = proxycore.GetOrCreateNopLogger(logger)
	mp := make(map[*client]struct{})
	prox := &Proxy{
		clients: mp,
		logger:  logger,
	}
	err := prox.Connect()
	assert.NoError(t, err, "function should return no error")
}

func Test_GetCqlQueryType(t *testing.T) {
	str := getCqlQueryType("select * from xyx")
	assert.NotEmptyf(t, str, "should not be empty")

	str = getCqlQueryType("update * from xyx")
	assert.NotEmptyf(t, str, "should not be empty")

	str = getCqlQueryType("insert * into xyx")
	assert.NotEmptyf(t, str, "should not be empty")

	str = getCqlQueryType("delete * from xyx")
	assert.NotEmptyf(t, str, "should not be empty")

	str = getCqlQueryType("negative scenario")
	assert.Emptyf(t, str, "should be empty")
}

func TestNameBasedUUID(t *testing.T) {
	uuid := nameBasedUUID("test")
	assert.NotNilf(t, uuid, "should not be nil")
}

func Test_addclient(t *testing.T) {
	relativePath := "fakedata/service_account.json"

	// Get the absolute path by resolving the relative path
	absolutePath, err := filepath.Abs(relativePath)
	if err != nil {
		assert.NoErrorf(t, err, "should not through an error")
	}
	newPath := strings.Replace(absolutePath, "/proxy/", "/", 1)
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", newPath)

	mp := make(map[*client]struct{})
	prox := &Proxy{
		clients: mp,
	}

	assert.NoErrorf(t, err, "no error expected")
	cl := &client{
		ctx:                 prox.ctx,
		proxy:               prox,
		preparedSystemQuery: make(map[[preparedIdSize]byte]interface{}),
		preparedQuerys:      make(map[[preparedIdSize]byte]interface{}),
	}
	prox.addClient(cl)
	prox.registerForEvents(cl)
}

func Test_OnEvent(t *testing.T) {
	var logger *zap.Logger
	relativePath := "fakedata/service_account.json"

	// Get the absolute path by resolving the relative path
	absolutePath, err := filepath.Abs(relativePath)
	if err != nil {
		assert.NoErrorf(t, err, "should not through an error")
	}
	newPath := strings.Replace(absolutePath, "/proxy/", "/", 1)
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", newPath)

	logger = proxycore.GetOrCreateNopLogger(logger)
	mp := make(map[*client]struct{})
	prox := &Proxy{
		clients: mp,
		logger:  logger,
	}
	assert.NoErrorf(t, err, "no error expected")
	prox.OnEvent(&proxycore.SchemaChangeEvent{Message: &message.SchemaChangeEvent{ChangeType: primitive.SchemaChangeType(primitive.EventTypeSchemaChange)}})

}

func Test_Serve(t *testing.T) {
	var logger *zap.Logger
	logger = proxycore.GetOrCreateNopLogger(logger)
	relativePath := "fakedata/service_account.json"

	// Get the absolute path by resolving the relative path
	absolutePath, err := filepath.Abs(relativePath)
	if err != nil {
		assert.NoErrorf(t, err, "should not through an error")
	}
	newPath := strings.Replace(absolutePath, "/proxy/", "/", 1)
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", newPath)

	logger = proxycore.GetOrCreateNopLogger(logger)
	mp := make(map[*client]struct{})
	ls := make(map[*net.Listener]struct{})
	prox := &Proxy{
		clients:   mp,
		logger:    logger,
		listeners: ls,
	}
	assert.NoErrorf(t, err, "no error expected")

	var httpListener net.Listener
	httpListener, err = resolveAndListen(":7777", "", "")
	assert.NoErrorf(t, err, "no error expected")

	prox.isConnected = true
	go func() {
		time.Sleep(5 * time.Second)
		prox.isClosing = true
		httpListener.Close()
	}()
	err = prox.Serve(httpListener)
	assert.Errorf(t, err, "error expected")

}

func Test_ServeScenario2(t *testing.T) {
	var logger *zap.Logger
	relativePath := "fakedata/service_account.json"

	// Get the absolute path by resolving the relative path
	absolutePath, err := filepath.Abs(relativePath)
	if err != nil {
		assert.NoErrorf(t, err, "should not through an error")
	}
	newPath := strings.Replace(absolutePath, "/proxy/", "/", 1)
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", newPath)

	logger = proxycore.GetOrCreateNopLogger(logger)
	mp := make(map[*client]struct{})
	ls := make(map[*net.Listener]struct{})
	cl := make(chan struct{})
	prox := &Proxy{
		clients:   mp,
		logger:    logger,
		listeners: ls,
		closed:    cl,
	}
	assert.NoErrorf(t, err, "no error expected")
	assert.NoErrorf(t, err, "no error expected")

	var httpListener net.Listener
	httpListener, err = resolveAndListen(":7777", "", "")
	assert.NoErrorf(t, err, "no error expected")

	prox.isConnected = true
	go func() {
		time.Sleep(5 * time.Second)
		close(prox.closed)
		httpListener.Close()
	}()
	err = prox.Serve(httpListener)
	assert.Errorf(t, err, "error expected")

}

type MockQueryPreparer struct {
	DeleteCalled bool
	DeleteParams struct {
		Raw *frame.RawFrame
		Msg *message.Prepare
		Id  [16]byte
	}
	InsertCalled bool
	InsertParams struct {
		Raw *frame.RawFrame
		Msg *message.Prepare
		Id  [16]byte
	}
	SelectCalled bool
	SelectParams struct {
		Raw *frame.RawFrame
		Msg *message.Prepare
		Id  [16]byte
	}
}

type MockClient struct {
	SendFunc func(header *frame.RawFrame, preparedResult *message.PreparedResult) error
}

func (m *MockClient) Send(header *frame.RawFrame, preparedResult *message.PreparedResult) error {
	if m.SendFunc != nil {
		return m.SendFunc(header, preparedResult)
	}
	return nil
}

type mockSender struct {
	SendCalled bool
	SendParams struct {
		Hdr *frame.Header
		Msg message.Message
	}
}

func (m *mockSender) Send(hdr *frame.Header, msg message.Message) {
	m.SendCalled = true
	m.SendParams.Hdr = hdr
	m.SendParams.Msg = msg
}

var mockRawFrame = &frame.RawFrame{
	Header: &frame.Header{
		Version:  primitive.ProtocolVersion4,
		Flags:    0,
		StreamId: 0,
		OpCode:   primitive.OpCodePrepare,
	},
	Body: []byte{},
}
var schemaConfigs = &schemaMapping.SchemaMappingConfig{
	TablesMetaData:  mockTableSchemaConfig,
	PkMetadataCache: mockPkMetadata,
	Logger:          zap.NewNop(),
}

var mockProxy = &Proxy{
	schemaMapping: schemaConfigs,
	translator:    &translator.Translator{SchemaMappingConfig: schemaConfigs},
	logger:        zap.NewNop(),
}

// Create mock for handleExecutionForDeletePreparedQuery/handleExecutionForSelectPreparedQuery/handleExecutionForInsertPreparedQuery functions.
type MockBigtableClient struct {
	InsertRowFunc          func(ctx context.Context, data *translator.InsertQueryMap) error
	UpdateRowFunc          func(ctx context.Context, data *translator.UpdateQueryMap) error
	DeleteRowFunc          func(ctx context.Context, data *translator.DeleteQueryMap) error
	GetSchemaConfigsFunc   func(ctx context.Context, tableName string) (map[string]map[string]*schemaMapping.Column, map[string][]schemaMapping.Column, error)
	InsertErrorDetailsFunc func(ctx context.Context, query responsehandler.ErrorDetail)
	ApplyBulkMutationFunc  func(ctx context.Context, tableName string, mutationData []bt.MutationData) (bt.BulkOperationResponse, error)
}

func (m *MockBigtableClient) InsertRow(ctx context.Context, data *translator.InsertQueryMap) error {
	if m.InsertRowFunc != nil {
		return m.InsertRowFunc(ctx, data)
	}
	return nil
}

func (m *MockBigtableClient) UpdateRow(ctx context.Context, data *translator.UpdateQueryMap) error {
	if m.UpdateRowFunc != nil {
		return m.UpdateRowFunc(ctx, data)
	}
	return nil
}

func (m *MockBigtableClient) DeleteRow(ctx context.Context, data *translator.DeleteQueryMap) error {
	if m.DeleteRowFunc != nil {
		return m.DeleteRowFunc(ctx, data)
	}
	return nil
}
func (m *MockBigtableClient) GetSchemaConfigs(ctx context.Context, tableName string) (map[string]map[string]*schemaMapping.Column, map[string][]schemaMapping.Column, error) {
	if m.GetSchemaConfigsFunc != nil {
		return m.GetSchemaConfigsFunc(ctx, tableName)
	}
	return nil, nil, nil
}

func (m *MockBigtableClient) InsertErrorDetails(ctx context.Context, query responsehandler.ErrorDetail) {
	if m.InsertErrorDetailsFunc != nil {
		m.InsertErrorDetailsFunc(ctx, query)
	}
}
func (m *MockBigtableClient) ApplyBulkMutation(ctx context.Context, tableName string, mutationData []bt.MutationData) (bt.BulkOperationResponse, error) {
	if m.ApplyBulkMutationFunc != nil {
		return m.ApplyBulkMutationFunc(ctx, tableName, mutationData)
	}
	return bt.BulkOperationResponse{}, nil
}

var positionValues = []*primitive.Value{
	{Type: primitive.ValueTypeNull, Contents: []byte("value1")},
	{Type: primitive.ValueTypeNull, Contents: []byte("value2")},
}

var namedValues = map[string]*primitive.Value{
	"test_id":   {Type: primitive.ValueTypeNull, Contents: []byte("value1")},
	"test_hash": {Type: primitive.ValueTypeNull, Contents: []byte("value2")},
}

func Test_handleExecutionForDeletePreparedQuery(t *testing.T) {

	mockProxy := &Proxy{
		schemaMapping: schemaConfigs,
		translator:    &translator.Translator{SchemaMappingConfig: schemaConfigs},
		logger:        zap.NewNop(),
		ctx:           context.Background(),
		bClient:       nil, // update client to include all function
		otelInst: &otelgo.OpenTelemetry{Config: &otelgo.OTelConfig{
			ServiceName: "test",
			OTELEnabled: false,
		}},
	}

	type fields struct {
		ctx                 context.Context
		proxy               *Proxy
		conn                *proxycore.Conn
		keyspace            string
		preparedSystemQuery map[[16]byte]interface{}
		preparedQuerys      map[[16]byte]interface{}
	}
	type args struct {
		raw           *frame.RawFrame
		msg           *partialExecute
		customPayload map[string][]byte
		st            *translator.DeleteQueryMap
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "Test handleExecutionForDeletePreparedQuery Query",
			fields: fields{
				ctx:   context.Background(),
				proxy: mockProxy,
			},

			args: args{
				raw: mockRawFrame,
				msg: &partialExecute{
					PositionalValues: positionValues,
					NamedValues:      namedValues,
				},
				customPayload: map[string][]byte{
					"test_id":   []byte("1234"),
					"test_hash": []byte("1234-2345"),
				},
				st: &translator.DeleteQueryMap{
					Table:     "test_table",
					Query:     "DELETE FROM key_space.test_table WHERE test_id = '?'",
					ParamKeys: []string{"test_id", "test_hash"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSender := &mockSender{}
			c := &client{
				ctx:                 tt.fields.ctx,
				proxy:               tt.fields.proxy,
				conn:                tt.fields.conn,
				keyspace:            tt.fields.keyspace,
				preparedSystemQuery: tt.fields.preparedSystemQuery,
				preparedQuerys:      tt.fields.preparedQuerys,
				sender:              mockSender,
			}
			c.handleExecuteForDelete(tt.args.raw, tt.args.msg, tt.args.st, tt.fields.ctx)
			assert.Nil(t, c.preparedSystemQuery)
			assert.Nil(t, c.preparedQuerys)
		})
	}
}

type MockTranslator struct {
	mock.Mock
}

func (m *MockTranslator) TranslateInsertQuerytoBigtable(queryStr string, protocolV primitive.ProtocolVersion) (*translator.InsertQueryMap, error) {
	args := m.Called(queryStr)
	return args.Get(0).(*translator.InsertQueryMap), args.Error(1)
}

func (m *MockTranslator) TranslateDeleteQuerytoBigtable(query string) (*translator.DeleteQueryMap, error) {
	args := m.Called(query)
	return args.Get(0).(*translator.DeleteQueryMap), args.Error(1)
}
func (m *MockTranslator) TranslateUpdateQuerytoBigtable(query string) (*translator.UpdateQueryMap, error) {
	args := m.Called(query)
	return args.Get(0).(*translator.UpdateQueryMap), args.Error(1)
}

type mockQuery struct{}

func TestHandleExecute(t *testing.T) {
	mockSender := &mockSender{}
	mockPartialExecute := &partialExecute{
		PositionalValues: positionValues,
		NamedValues:      namedValues,
	}
	mockSystemQueryID := preparedIdKey([]byte("some_prepared_query_id"))

	tests := []struct {
		name           string
		client         *client
		raw            *frame.RawFrame
		msg            *partialExecute
		setupClient    func(c *client)
		wantSendCalled bool
		wantSendMsg    message.Message
	}{
		{
			name: "System Query Execution",
			raw:  mockRawFrame,
			msg:  &partialExecute{queryId: []byte("some_prepared_query_id")},
			setupClient: func(c *client) {
				c.preparedSystemQuery = map[[16]byte]interface{}{
					mockSystemQueryID: &mockQuery{},
				}
			},
			wantSendCalled: true,
		},
		{
			name: "Prepared Query Execution",
			raw:  mockRawFrame,
			msg:  &partialExecute{queryId: []byte("some_other_prepared_query_id")},
			setupClient: func(c *client) {
				c.preparedQuerys = map[[16]byte]interface{}{
					preparedIdKey([]byte("some_other_prepared_query_id")): &translator.SelectQueryMap{},
				}
			},
			wantSendCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &client{
				proxy:               mockProxy,
				preparedSystemQuery: make(map[[16]byte]interface{}),
				preparedQuerys:      make(map[[16]byte]interface{}),
				sender:              mockSender,
			}
			if tt.setupClient != nil {
				tt.setupClient(c)
			}

			c.handleExecute(mockRawFrame, mockPartialExecute)
			if tt.wantSendCalled {
				if !mockSender.SendCalled {
					t.Errorf("Send was not called when expected for test: %s", tt.name)
				}
			} else {
				if mockSender.SendCalled {
					t.Errorf("Send was called when not expected for test: %s", tt.name)
				}
			}
		})
	}
}

func Test_client_handlePrepare(t *testing.T) {
	mockSender := &mockSender{}
	selectQuery := &message.Prepare{
		Query: "SELECT * FROM system.local WHERE key = 'local'",
	}
	useQuery := &message.Prepare{
		Query: "USE test_keyspace",
	}
	invalidQuery := &message.Prepare{
		Query: "INVALID QUERY",
	}

	tests := []struct {
		name           string
		client         *client
		raw            *frame.RawFrame
		msg            *message.Prepare
		setupClient    func(c *client)
		wantSendCalled bool
		wantSendMsg    message.Message
	}{
		{
			name: "Select Query",
			raw:  mockRawFrame,
			msg:  selectQuery,
			setupClient: func(c *client) {
				c.preparedSystemQuery = make(map[[16]byte]interface{})
				c.preparedQuerys = make(map[[16]byte]interface{})
			},
			wantSendCalled: true,
		},
		{
			name: "Use Query",
			raw:  mockRawFrame,
			msg:  useQuery,
			setupClient: func(c *client) {
				c.preparedSystemQuery = make(map[[16]byte]interface{})
				c.preparedQuerys = make(map[[16]byte]interface{})
			},
			wantSendCalled: true,
		},
		{
			name: "Invalid Query",
			raw:  mockRawFrame,
			msg:  invalidQuery,
			setupClient: func(c *client) {
				c.preparedSystemQuery = make(map[[16]byte]interface{})
				c.preparedQuerys = make(map[[16]byte]interface{})
			},
			wantSendCalled: true,
		},
		{
			name: "Select Statement with Invalid Table",
			raw:  mockRawFrame,
			msg:  selectQuery,
			setupClient: func(c *client) {
				c.preparedSystemQuery = make(map[[16]byte]interface{})
				c.preparedQuerys = make(map[[16]byte]interface{})
			},
			wantSendCalled: true,
			wantSendMsg:    &message.Invalid{},
		},
		{
			name: "Query Parsing Error",
			raw:  mockRawFrame,
			msg:  &message.Prepare{Query: "invalid query syntax"},
			setupClient: func(c *client) {
				c.preparedSystemQuery = make(map[[16]byte]interface{})
				c.preparedQuerys = make(map[[16]byte]interface{})
			},
			wantSendCalled: true,
			wantSendMsg:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &client{
				proxy:               mockProxy,
				preparedSystemQuery: make(map[[16]byte]interface{}),
				preparedQuerys:      make(map[[16]byte]interface{}),
				sender:              mockSender,
			}
			if tt.setupClient != nil {
				tt.setupClient(c)
			}

			c.handlePrepare(tt.raw, tt.msg)
			if tt.wantSendCalled {
				if !mockSender.SendCalled {
					t.Errorf("Send was not called when expected for test: %s", tt.name)
				}
			} else {
				if mockSender.SendCalled {
					t.Errorf("Send was called when not expected for test: %s", tt.name)
				}
			}
		})
	}
}

func createSchemaConfig() *schemaMapping.SchemaMappingConfig {
	return &schemaMapping.SchemaMappingConfig{
		TablesMetaData:  mockTableSchemaConfig,
		PkMetadataCache: mockPkMetadata,
	}
}

var mockTableSchemaConfig = map[string]map[string]map[string]*schemaMapping.Column{

	"keyspace": {
		"test_table": {
			"test_id": {
				ColumnName: "test_id",
				ColumnType: "text",
				Metadata: message.ColumnMetadata{
					Keyspace: "",
					Table:    "test_table",
					Name:     "test_id",
					Type:     datatype.Varchar,
					Index:    0,
				},
				IsPrimaryKey: true,
			},
			"test_hash": {
				ColumnName: "test_hash",
				ColumnType: "text",
				Metadata: message.ColumnMetadata{
					Keyspace: "",
					Table:    "test_table",
					Name:     "test_hash",
					Type:     datatype.Varchar,
					Index:    1,
				},
			},

			"column1": &schemaMapping.Column{
				ColumnName:   "column1",
				ColumnType:   "text",
				IsPrimaryKey: true,
				PkPrecedence: 1,
			},
			"column2": &schemaMapping.Column{
				ColumnName:   "column2",
				ColumnType:   "blob",
				IsPrimaryKey: false,
			},
			"column3": &schemaMapping.Column{
				ColumnName:   "column3",
				ColumnType:   "boolean",
				IsPrimaryKey: false,
			},

			"column10": &schemaMapping.Column{
				ColumnName:   "column10",
				ColumnType:   "text",
				IsPrimaryKey: true,
				PkPrecedence: 2,
			},
		},
		"user_info": {
			"name": &schemaMapping.Column{
				ColumnName:   "name",
				ColumnType:   "text",
				IsPrimaryKey: true,
				PkPrecedence: 0,
				IsCollection: false,
			},
		},
	},
}

var mockPkMetadata = map[string]map[string][]schemaMapping.Column{
	"keyspace": {
		"test_table": {
			{
				ColumnName:   "test_id",
				CQLType:      "text",
				IsPrimaryKey: true,
				PkPrecedence: 1,
			},
			{
				ColumnName:   "test_hash",
				ColumnType:   "text",
				IsPrimaryKey: false,
				PkPrecedence: 2,
			},
		},
		"user_info": {
			{
				ColumnName:   "name",
				CQLType:      "text",
				IsPrimaryKey: true,
				PkPrecedence: 0,
			},
		},
	},
}

func Test_validatePrimaryKey(t *testing.T) {
	type args struct {
		query *translator.InsertQueryMap
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Primary key present and not null",
			args: args{
				query: &translator.InsertQueryMap{
					Columns: []translator.Column{
						{Name: "id", IsPrimaryKey: true},
						{Name: "name", IsPrimaryKey: false},
					},
					Values: []interface{}{[]uint8{0, 0, 1}, "Alice"},
				},
			},
			want: "",
		},
		{
			name: "Primary key missing value",
			args: args{
				query: &translator.InsertQueryMap{
					Columns: []translator.Column{
						{Name: "id", IsPrimaryKey: true},
						{Name: "name", IsPrimaryKey: false},
					},
					Values: []interface{}{nil, "Bob"},
				},
			},
			want: "id",
		},
		{
			name: "No primary key column",
			args: args{
				query: &translator.InsertQueryMap{
					Columns: []translator.Column{
						{Name: "age", IsPrimaryKey: false},
						{Name: "name", IsPrimaryKey: false},
					},
					Values: []interface{}{25, "Eve"},
				},
			},
			want: "",
		},
		{
			name: "Multiple primary keys, one missing value",
			args: args{
				query: &translator.InsertQueryMap{
					Columns: []translator.Column{
						{Name: "user_id", IsPrimaryKey: true},
						{Name: "product_id", IsPrimaryKey: true},
					},
					Values: []interface{}{nil, 101},
				},
			},
			want: "user_id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validatePrimaryKey(tt.args.query); got != tt.want {
				t.Errorf("validatePrimaryKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
