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

package compliance

import (
	"cloud.google.com/go/bigtable"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaAutoRefresh(t *testing.T) {
	if testTarget != TestTargetProxy {
		t.Skip("skipping schema refresh test because it's only relevant for the proxy")
	}

	tableName := uniqueTableName("refresh_test")
	err := session.Query(fmt.Sprintf("CREATE TABLE %s (id text PRIMARY KEY, original_col text)", tableName)).Exec()
	require.NoError(t, err)
	defer cleanupTable(t, tableName)

	newColumn := "external_col"

	// Manually add the column to the schema_mapping table in Bigtable, bypassing the proxy.
	// This simulates another proxy instance making a schema change.
	ctx := context.Background()
	// The default schema mapping table name is "schema_mapping".
	tbl := client.Open("schema_mapping")
	ts := bigtable.Now()
	mut := bigtable.NewMutation()
	mut.Set("cf", "ColumnName", ts, []byte(newColumn))
	mut.Set("cf", "ColumnType", ts, []byte("text"))
	mut.Set("cf", "TableName", ts, []byte(tableName))

	err = tbl.Apply(ctx, tableName+"#"+newColumn, mut)
	require.NoError(t, err)

	// Attempt to insert data into the new column until it succeeds (max 60 seconds).
	// The proxy should pick up the change within 30 seconds by default.
	success := false
	startTime := time.Now()
	var lastErr error
	for time.Since(startTime) < 60*time.Second {
		err = session.Query(fmt.Sprintf("INSERT INTO %s (id, original_col, %s) VALUES ('1', 'foo', 'bar')", tableName, newColumn)).Exec()
		if err == nil {
			success = true
			break
		}
		lastErr = err
		time.Sleep(2 * time.Second)
	}

	require.True(t, success, "Proxy should have picked up the new column within the refresh period. Last error: %v", lastErr)

	// Verify the data can be read back.
	var id, originalCol, externalCol string
	err = session.Query(fmt.Sprintf("SELECT id, original_col, %s FROM %s WHERE id='1'", newColumn, tableName)).Scan(&id, &originalCol, &externalCol)
	assert.NoError(t, err)
	assert.Equal(t, "1", id)
	assert.Equal(t, "foo", originalCol)
	assert.Equal(t, "bar", externalCol)
}
