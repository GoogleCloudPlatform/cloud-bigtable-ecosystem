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

package it

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

var session *gocql.Session

func TestMain(m *testing.M) {
	cluster := gocql.NewCluster("127.0.0.1")
	cluster.Port = 9042
	cluster.Keyspace = "bigtabledevinstance"
	cluster.ProtoVersion = 4

	var err error
	session, err = cluster.CreateSession()

	if err != nil {
		panic(err)
	}

	defer session.Close()

	var tablesSql = []string{
		"CREATE TABLE IF NOT EXISTS bigtabledevinstance.intkeys (num INT, big_num BIGINT, name VARCHAR, PRIMARY KEY (num, big_num));",
	}

	for i, tableSql := range tablesSql {
		err = session.Query(tableSql).Exec()
		if err != nil {
			panic(fmt.Errorf("error applying create table sql %d: %w", i, err))
		}
	}

	code := m.Run()

	os.Exit(code)
}

func TestIntMaxAndMinInKey(t *testing.T) {
	err := session.Query("insert into bigtabledevinstance.intkeys (num, big_num, name) values (?, ?, ?)", int32(math.MaxInt32), int64(math.MaxInt64), "bob").Exec()
	assert.NoError(t, err)
	scanner := session.Query("select num, big_num, name from bigtabledevinstance.intkeys where num=? and big_num=?", int32(math.MaxInt32), int64(math.MaxInt64)).Iter().Scanner()

	var (
		num       int32
		bigNum    int64
		name      string
		scanCount int
	)

	for scanner.Next() {
		scanCount++
		err = scanner.Scan(&num, &bigNum, &name)
		assert.NoError(t, err)
	}
	err = scanner.Err()
	assert.NoError(t, err)

	assert.Equal(t, 1, scanCount)
	assert.Equal(t, int32(math.MaxInt32), num)
	assert.Equal(t, int64(math.MaxInt64), bigNum)
	assert.Equal(t, "bob", name)
}

func TestSelectSingleMapValue(t *testing.T) {
	err := session.Query("CREATE TABLE IF NOT EXISTS bigtabledevinstance.map_test (id VARCHAR PRIMARY KEY, zips map<TEXT,BIGINT>);").Exec()
	assert.NoError(t, err)

	id := uuid.New().String()

	err = session.Query("insert into bigtabledevinstance.map_test (id, zips) values (?, ?)", id, map[string]int64{"chelsea": 10011, "burlington": 5401}).Exec()
	assert.NoError(t, err)

	itr := session.Query("select id, zips['chelsea'] from bigtabledevinstance.map_test where id=?", id).Iter()

	scanner := itr.Scanner()

	var (
		gotId     string
		gotZip    int64
		scanCount int
	)

	for scanner.Next() {
		scanCount++
		err = scanner.Scan(&gotId, &gotZip)
		assert.NoError(t, err)
	}
	err = scanner.Err()
	assert.NoError(t, err)

	assert.Equal(t, 1, scanCount)
	assert.Equal(t, id, gotId)
	assert.Equal(t, int64(10011), gotZip)

	itr = session.Query("select id, zips['chelsea'] as czip from bigtabledevinstance.map_test where id=?", id).Iter()
	scanner = itr.Scanner()
	scanCount = 0
	for scanner.Next() {
		scanCount++
		err = scanner.Scan(&gotId, &gotZip)
		assert.NoError(t, err)
	}
	err = scanner.Err()
	assert.NoError(t, err)

	assert.Equal(t, 1, scanCount)
	assert.Equal(t, id, gotId)
	assert.Equal(t, int64(10011), gotZip)
}

func TestDeleteIfExist(t *testing.T) {
	// dropping a random table that definitely doesn't exist should be ok
	id := strings.ReplaceAll(uuid.New().String(), "-", "_")
	err := session.Query(fmt.Sprintf("DROP TABLE IF EXISTS bigtabledevinstance.table_%s", id)).Exec()
	assert.NoError(t, err)
}

func TestCreateIfNotExist(t *testing.T) {
	// dropping a random table that definitely doesn't exist should be ok
	table := "create_" + strings.ReplaceAll(uuid.New().String(), "-", "_")
	err := session.Query(fmt.Sprintf("CREATE TABLE bigtabledevinstance.%s (id TEXT PRIMARY KEY, name TEXT)", table)).Exec()
	assert.NoError(t, err)
	err = session.Query(fmt.Sprintf("CREATE TABLE bigtabledevinstance.%s (id TEXT PRIMARY KEY, name TEXT)", table)).Exec()
	assert.Error(t, err)
	err = session.Query(fmt.Sprintf("CREATE TABLE IF NOT EXISTS bigtabledevinstance.%s (id TEXT PRIMARY KEY, name TEXT)", table)).Exec()
	assert.NoError(t, err)
}

func TestAlterTable(t *testing.T) {
	table := "ddl_table_" + strings.ReplaceAll(uuid.New().String(), "-", "_")
	t.Logf("running test %s with random table name %s", t.Name(), table)
	err := session.Query(fmt.Sprintf("CREATE TABLE bigtabledevinstance.%s (id TEXT PRIMARY KEY, name TEXT)", table)).Exec()
	assert.NoError(t, err)

	insertQuery := session.Query(fmt.Sprintf("INSERT INTO bigtabledevinstance.%s (id, name, age) VALUES (?, ?, ?)", table), "abc", "bob", 32)

	err = insertQuery.Exec()
	assert.Error(t, err, "insert should fail because there is no age column")

	err = session.Query(fmt.Sprintf("ALTER TABLE bigtabledevinstance.%s ADD age INT", table)).Exec()
	assert.NoError(t, err)

	err = insertQuery.Exec()
	assert.NoError(t, err)

	err = session.Query(fmt.Sprintf("ALTER TABLE bigtabledevinstance.%s DROP age", table)).Exec()
	assert.NoError(t, err)

	err = session.Query(fmt.Sprintf("ALTER TABLE bigtabledevinstance.%s ADD weight FLOAT", table)).Exec()
	assert.NoError(t, err)

	err = session.Query(fmt.Sprintf("INSERT INTO bigtabledevinstance.%s (id, name, weight) VALUES (?, ?, ?)", table), "abc", "bob", float32(190.5)).Exec()
	assert.NoError(t, err)

	err = session.Query(fmt.Sprintf("DROP TABLE  bigtabledevinstance.%s", table)).Exec()
	assert.NoError(t, err)
}
