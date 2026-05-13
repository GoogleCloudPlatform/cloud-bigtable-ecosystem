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

func TestInsertTimeUuidLiteral(t *testing.T) {
	t.Parallel()
	require.NoError(t, session.Query(`INSERT INTO timeuuid_key (pk, id, val) VALUES ('timeuuid-literal', 6c9b87f6-d764-11f0-8f97-8e0ad7a51247, 'value')`).Exec())

	var pkGot string
	var idGot gocql.UUID
	var valGot string
	require.NoError(t, session.Query(`SELECT pk, id, val FROM bigtabledevinstance.timeuuid_key WHERE pk = 'timeuuid-literal'`).Scan(&pkGot, &idGot, &valGot))

	assert.Equal(t, "timeuuid-literal", pkGot)
	assert.Equal(t, "6c9b87f6-d764-11f0-8f97-8e0ad7a51247", idGot.String())
	assert.Equal(t, "value", valGot)
}
func TestInsertTimeUuidNull(t *testing.T) {
	t.Parallel()
	require.NoError(t, session.Query(`INSERT INTO timeuuid_key (pk, id, val, parentEvent) VALUES ('timeuuid-null', 6c9b87f6-d764-11f0-8f97-8e0ad7a51247, 'value', null)`).Exec())

	got := TimeUuidEvent{}
	require.NoError(t, session.Query(`SELECT pk, id, val, parentEvent FROM timeuuid_key WHERE pk = 'timeuuid-null'`).Scan(&got.pk, &got.id, &got.val, &got.parentEvent))

	assert.Equal(t, "timeuuid-null", got.pk)
	assert.Equal(t, "6c9b87f6-d764-11f0-8f97-8e0ad7a51247", got.id.String())
	assert.Equal(t, "value", got.val)
	assert.Equal(t, (*gocql.UUID)(nil), got.parentEvent)
}

func TestMaxAndMinTimestamp(t *testing.T) {
	t.Parallel()

	t1, _ := gocql.ParseUUID("7b287b9e-d769-11f0-b949-8e0ad7a51247")
	t2, _ := gocql.ParseUUID("7e2fcec8-d769-11f0-b94a-8e0ad7a51247")
	t3, _ := gocql.ParseUUID("8133fa68-d769-11f0-b94b-8e0ad7a51247")
	t4, _ := gocql.ParseUUID("843a7228-d769-11f0-b94c-8e0ad7a51247")
	t5, _ := gocql.ParseUUID("87575e30-d769-11f0-b94d-8e0ad7a51247")

	require.NoError(t, session.Query(`INSERT INTO timeuuid_key (pk, id, val, parentEvent) VALUES ('timeuuid-minmax', ?, '3', now())`, t1).Exec())
	require.NoError(t, session.Query(`INSERT INTO timeuuid_key (pk, id, val, parentEvent) VALUES ('timeuuid-minmax', ?, '3', now())`, t2).Exec())
	require.NoError(t, session.Query(`INSERT INTO timeuuid_key (pk, id, val, parentEvent) VALUES ('timeuuid-minmax', ?, '3', now())`, t3).Exec())
	require.NoError(t, session.Query(`INSERT INTO timeuuid_key (pk, id, val, parentEvent) VALUES ('timeuuid-minmax', ?, '3', now())`, t4).Exec())
	require.NoError(t, session.Query(`INSERT INTO timeuuid_key (pk, id, val, parentEvent) VALUES ('timeuuid-minmax', ?, '3', now())`, t5).Exec())

	t.Run("< timeuuid", func(t *testing.T) {
		t.Parallel()
		got, err := scanAllTimeuuids(session.Query(`SELECT id FROM timeuuid_key WHERE pk='timeuuid-minmax' AND id < ?`, t3).Iter().Scanner())
		require.NoError(t, err)
		assert.Equal(t, []gocql.UUID{t1, t2}, got)
	})

	t.Run("> timeuuid", func(t *testing.T) {
		t.Parallel()
		got, err := scanAllTimeuuids(session.Query(`SELECT id FROM timeuuid_key WHERE pk='timeuuid-minmax' AND id > ?`, t3).Iter().Scanner())
		require.NoError(t, err)
		assert.Equal(t, []gocql.UUID{t4, t5}, got)
	})

	t.Run(">= timeuuid", func(t *testing.T) {
		t.Parallel()
		got, err := scanAllTimeuuids(session.Query(`SELECT id FROM timeuuid_key WHERE pk='timeuuid-minmax' AND id >= ?`, t3).Iter().Scanner())
		require.NoError(t, err)
		assert.Equal(t, []gocql.UUID{t3, t4, t5}, got)
	})

	t.Run("<= timeuuid", func(t *testing.T) {
		t.Parallel()
		got, err := scanAllTimeuuids(session.Query(`SELECT id FROM timeuuid_key WHERE pk='timeuuid-minmax' AND id <= ?`, t3).Iter().Scanner())
		require.NoError(t, err)
		assert.Equal(t, []gocql.UUID{t1, t2, t3}, got)
	})

	t.Run("between t1 t3", func(t *testing.T) {
		t.Parallel()
		got, err := scanAllTimeuuids(session.Query(`SELECT id FROM timeuuid_key WHERE pk='timeuuid-minmax' AND id BETWEEN ? AND ?`, t1, t3).Iter().Scanner())
		require.NoError(t, err)
		// between is inclusive
		assert.Equal(t, []gocql.UUID{t1, t2, t3}, got)
	})
}

func scanAllTimeuuids(scanner gocql.Scanner) ([]gocql.UUID, error) {
	var results []gocql.UUID = nil
	for scanner.Next() {
		var t gocql.UUID
		err := scanner.Scan(
			&t,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, t)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func TestInsertTimeUuidPrepared(t *testing.T) {
	t.Parallel()

	t1 := gocql.TimeUUID()
	t2 := gocql.TimeUUID()
	require.NoError(t, session.Query(`INSERT INTO bigtabledevinstance.timeuuid_key (pk, id, val, parentEvent) VALUES ('timeuuid-prepared', ?, '3', ?)`, t1, t2).Exec())
	got := TimeUuidEvent{}
	require.NoError(t, session.Query(`SELECT pk, id, val, parentEvent FROM bigtabledevinstance.timeuuid_key WHERE pk = 'timeuuid-prepared'`).Scan(&got.pk, &got.id, &got.val, &got.parentEvent))
	assert.Equal(t, "timeuuid-prepared", got.pk)
	assert.Equal(t, t1, *got.id)
	assert.Equal(t, "3", got.val)
	assert.Equal(t, t2, *got.parentEvent)

	require.NoError(t, session.Query(`SELECT pk, id, val, parentEvent FROM bigtabledevinstance.timeuuid_key WHERE id=? AND parentEvent=?`, t1, t2).Scan(&got.pk, &got.id, &got.val, &got.parentEvent))
	assert.Equal(t, "timeuuid-prepared", got.pk)
	assert.Equal(t, t1, *got.id)
	assert.Equal(t, "3", got.val)
	assert.Equal(t, t2, *got.parentEvent)
}

type TimeUuidEvent struct {
	pk          string
	id          *gocql.UUID
	val         string
	parentEvent *gocql.UUID
}
