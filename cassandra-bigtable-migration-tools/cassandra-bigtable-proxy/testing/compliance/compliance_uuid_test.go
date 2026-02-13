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
	"testing"
	"time"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUuidAndTimeuuid(t *testing.T) {
	t.Parallel()

	u, _ := gocql.RandomUUID()
	tu := gocql.TimeUUID()

	name := "uuid_test_user"

	// 1. Insert data
	err := session.Query(`INSERT INTO bigtabledevinstance.all_columns (name, uuid_col, timeuuid_col) VALUES (?, ?, ?)`,
		name, u, tu).Exec()
	require.NoError(t, err)

	// 2. Select back
	var nameGot string
	var uGot gocql.UUID
	var tuGot gocql.UUID

	err = session.Query(`SELECT name, uuid_col, timeuuid_col FROM bigtabledevinstance.all_columns WHERE name = ?`, name).Scan(&nameGot, &uGot, &tuGot)
	require.NoError(t, err)

	assert.Equal(t, name, nameGot)
	assert.Equal(t, u, uGot)
	assert.Equal(t, tu, tuGot)
}

func TestNowFunction(t *testing.T) {
	t.Parallel()

	name := "now_test"

	// 1. Insert using now()
	err := session.Query(`INSERT INTO bigtabledevinstance.all_columns (name, timeuuid_col) VALUES (?, now())`, name).Exec()
	require.NoError(t, err)

	// 2. Select back and verify it's a version 1 or 7 UUID
	var tuGot gocql.UUID
	err = session.Query(`SELECT timeuuid_col FROM bigtabledevinstance.all_columns WHERE name = ?`, name).Scan(&tuGot)
	require.NoError(t, err)

	assert.True(t, tuGot.Version() == 1 || tuGot.Version() == 7, "Expected version 1 or 7 UUID from now(), got %d", tuGot.Version())
}

func TestToTimestampFunction(t *testing.T) {
	t.Parallel()

	name := "totimestamp_test"
	tu := gocql.TimeUUID()
	expectedTime := tu.Time().UTC()

	// 1. Insert
	err := session.Query(`INSERT INTO bigtabledevinstance.all_columns (name, timeuuid_col) VALUES (?, ?)`, name, tu).Exec()
	require.NoError(t, err)

	// 2. Select using totimestamp()
	var tsGot time.Time
	err = session.Query(`SELECT totimestamp(timeuuid_col) FROM bigtabledevinstance.all_columns WHERE name = ?`, name).Scan(&tsGot)
	require.NoError(t, err)

	// Compare with some tolerance (1ms) because of potential rounding in different layers,
	// though it should be exact if using milliseconds throughout.
	assert.WithinDuration(t, expectedTime, tsGot.UTC(), time.Millisecond)
}

func TestUuidAsPrimaryKey(t *testing.T) {
	t.Parallel()

	u, _ := gocql.RandomUUID()
	val := "some value"

	// 1. Insert
	err := session.Query(`INSERT INTO bigtabledevinstance.uuid_key (id, val) VALUES (?, ?)`, u, val).Exec()
	require.NoError(t, err)

	// 2. Select back using the key
	var valGot string
	err = session.Query(`SELECT val FROM bigtabledevinstance.uuid_key WHERE id = ?`, u).Scan(&valGot)
	require.NoError(t, err)

	assert.Equal(t, val, valGot)
}

func TestTimeuuidAsClusteringKey(t *testing.T) {
	t.Parallel()

	pk := "partition1"
	tu1 := gocql.TimeUUID()
	time.Sleep(10 * time.Millisecond) // ensure different timestamps
	tu2 := gocql.TimeUUID()

	// 1. Insert
	err := session.Query(`INSERT INTO bigtabledevinstance.timeuuid_key (pk, id, val) VALUES (?, ?, ?)`, pk, tu1, "val1").Exec()
	require.NoError(t, err)
	err = session.Query(`INSERT INTO bigtabledevinstance.timeuuid_key (pk, id, val) VALUES (?, ?, ?)`, pk, tu2, "val2").Exec()
	require.NoError(t, err)

	// 2. Select and verify ordering or just retrieval
	iter := session.Query(`SELECT id, val FROM bigtabledevinstance.timeuuid_key WHERE pk = ? ORDER BY id ASC`, pk).Iter()

	var idGot gocql.UUID
	var valGot string

	require.True(t, iter.Scan(&idGot, &valGot))
	assert.Equal(t, tu1, idGot)
	assert.Equal(t, "val1", valGot)

	require.True(t, iter.Scan(&idGot, &valGot))
	assert.Equal(t, tu2, idGot)
	assert.Equal(t, "val2", valGot)

	require.False(t, iter.Scan(&idGot, &valGot))
	require.NoError(t, iter.Close())
}
