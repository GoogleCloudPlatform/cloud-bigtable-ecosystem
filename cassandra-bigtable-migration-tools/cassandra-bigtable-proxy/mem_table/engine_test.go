package mem_table

import (
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/parser"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/translators/select_translator"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log"
	"testing"
	"time"
)

func Test_SelectEngine(t *testing.T) {

	userInfoTable := schemaMapping.NewTableConfig(
		"test_keyspace",
		"user_info",
		"cf1",
		types.OrderedCodeEncoding,
		[]*types.Column{
			{Name: "name", CQLType: types.TypeVarchar, KeyType: types.KeyTypePartition, IsPrimaryKey: true, PkPrecedence: 1},
			{Name: "age", CQLType: types.TypeBigInt, KeyType: types.KeyTypeClustering, IsPrimaryKey: true, PkPrecedence: 2},
			{Name: "email", CQLType: types.TypeText},
			{Name: "username", CQLType: types.TypeText},
		},
	)
	schemas := schemaMapping.NewSchemaMetadata("cf1", []*schemaMapping.TableSchema{
		userInfoTable,
	})

	engine := NewInMemEngine()
	err := engine.SetData(userInfoTable, []types.GoRow{
		{
			"name":     "u1",
			"age":      int64(32),
			"email":    "foo@bar.co",
			"username": "foo1992",
		},
		{
			"name":     "u2",
			"age":      int64(51),
			"email":    "fizz@buzz.net",
			"username": "fizzle2",
		},
		{
			"name":     "u3",
			"age":      int64(80),
			"email":    "turkey@gmail.com",
			"username": "turkey3",
		},
	})
	if err != nil {
		log.Fatal(err.Error())
	}

	tests := []struct {
		name            string
		sessionKeyspace types.Keyspace
		query           string
		want            []types.GoRow
		wantErr         string
	}{
		{
			name:  "select *",
			query: "select * from test_keyspace.user_info",
			want: []types.GoRow{
				{
					"name":     "u1",
					"age":      int64(32),
					"email":    "foo@bar.co",
					"username": "foo1992",
				},
				{
					"name":     "u2",
					"age":      int64(51),
					"email":    "fizz@buzz.net",
					"username": "fizzle2",
				},
				{
					"name":     "u3",
					"age":      int64(80),
					"email":    "turkey@gmail.com",
					"username": "turkey3",
				},
			},
		},
		{
			name:  "select name and age where",
			query: "select age, name from test_keyspace.user_info where age=32",
			want: []types.GoRow{
				{
					"name": "u1",
					"age":  int64(32),
				},
			},
		},
		{
			name:  "with aliases",
			query: "select age as a, name as n from test_keyspace.user_info where age=80",
			want: []types.GoRow{
				{
					"n": "u3",
					"a": int64(80),
				},
			},
		},
		{
			name:  "where lt",
			query: "select age, name from test_keyspace.user_info where age<80",
			want: []types.GoRow{
				{
					"name": "u1",
					"age":  int64(32),
				},
				{
					"name": "u2",
					"age":  int64(51),
				},
			},
		},
		{
			name:  "where lte",
			query: "select age, name from test_keyspace.user_info where age<=51",
			want: []types.GoRow{
				{
					"name": "u1",
					"age":  int64(32),
				},
				{
					"name": "u2",
					"age":  int64(51),
				},
			},
		},
		{
			name:  "where gt",
			query: "select age, name from test_keyspace.user_info where age>51",
			want: []types.GoRow{
				{
					"name": "u3",
					"age":  int64(80),
				},
			},
		},
		{
			name:  "where gte",
			query: "select age, name from test_keyspace.user_info where age>=51",
			want: []types.GoRow{
				{
					"name": "u2",
					"age":  int64(51),
				},
				{
					"name": "u3",
					"age":  int64(80),
				},
			},
		},
		{
			name:  "limit",
			query: "select age, name from test_keyspace.user_info where age>=51 limit 1",
			want: []types.GoRow{
				{
					"name": "u2",
					"age":  int64(51),
				},
			},
		},
		{
			name:  "count(*)",
			query: "select count(*) from test_keyspace.user_info",
			want: []types.GoRow{
				{
					"system.count(*)": int64(3),
				},
			},
		},
		{
			name:  "count(*) alias",
			query: "select count(*) as c from test_keyspace.user_info",
			want: []types.GoRow{
				{
					"c": int64(3),
				},
			},
		},
		{
			name:  "count(*) where",
			query: "select count(*) from test_keyspace.user_info where age < 80",
			want: []types.GoRow{
				{
					"system.count(*)": int64(2),
				},
			},
		},
		{
			name:  "multiple where conditions",
			query: "select * from test_keyspace.user_info where name='u2' and age=51",
			want: []types.GoRow{
				{
					"name":     "u2",
					"age":      int64(51),
					"email":    "fizz@buzz.net",
					"username": "fizzle2",
				},
			},
		},
		{
			name:  "multiple where conditions no match",
			query: "select * from test_keyspace.user_info where name='u2' and age=80",
			want:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			tr := select_translator.NewSelectTranslator(schemas)
			preparedQuery, err := tr.Translate(types.NewRawQuery(nil, tt.sessionKeyspace, tt.query, parser.NewParser(tt.query), types.QueryTypeSelect), tt.sessionKeyspace)
			require.NoError(t, err)

			values := types.NewQueryParameterValues(preparedQuery.Parameters(), time.Now())

			executableQuery, err := tr.Bind(preparedQuery, values, primitive.ProtocolVersion4)
			require.NoError(t, err)

			executableSelectQuery := executableQuery.(*types.ExecutableSelectQuery)
			require.NotNil(t, executableSelectQuery)

			results, err := engine.Execute(executableSelectQuery)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, results)
		})
	}
}
