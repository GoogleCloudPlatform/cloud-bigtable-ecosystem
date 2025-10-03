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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsertUpdateAndValidateRecord(t *testing.T) {
	t.Parallel()
	// 1. Insert a new record
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code, credited) VALUES (?, ?, ?, ?)`,
		"Liam", int64(44), 678, 8888.0).Exec()
	require.NoError(t, err, "Failed to insert record")

	// 2. Update the record
	err = session.Query(`UPDATE bigtabledevinstance.user_info SET code = ? WHERE name = ? AND age = ?`,
		789, "Liam", int64(44)).Exec()
	require.NoError(t, err, "Failed to update record")

	// 3. Validate the update
	var name string
	var age int64
	var code int
	err = session.Query(`SELECT name, age, code FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
		"Liam", int64(44)).Scan(&name, &age, &code)
	require.NoError(t, err, "Failed to select record for validation")

	assert.Equal(t, "Liam", name)
	assert.Equal(t, int64(44), age)
	assert.Equal(t, 789, code, "The code field was not updated correctly")
}

func TestInsertAndUpdateWithFutureTimestampValidation(t *testing.T) {
	t.Parallel()
	// Cassandra uses microsecond timestamps
	nowMicros := time.Now().UnixNano() / 1000
	futureMicros := nowMicros + 1000000 // 1 second in the future

	// 1. Insert a record with the current timestamp
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code, credited) VALUES (?, ?, ?, ?) USING TIMESTAMP ?`,
		"Richard", int64(67), 12312, 876543.0, nowMicros).Exec()
	require.NoError(t, err, "Failed to insert record with timestamp")

	// 2. Update the record with a future timestamp
	err = session.Query(`UPDATE bigtabledevinstance.user_info USING TIMESTAMP ? SET code = ?, credited=? WHERE name = ? AND age = ?`,
		futureMicros, 7899, 1908.0, "Richard", int64(67)).Exec()
	require.NoError(t, err, "Failed to update record with future timestamp")

	// 3. Validate that the update was successful
	var name string
	var age int64
	var code int
	err = session.Query(`SELECT name, age, code FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
		"Richard", int64(67)).Scan(&name, &age, &code)
	require.NoError(t, err, "Failed to select record for validation")

	assert.Equal(t, "Richard", name)
	assert.Equal(t, int64(67), age)
	assert.Equal(t, 7899, code, "The record should reflect the update with the future timestamp")
}

func TestInsertAndUpdateWithPastTimestampValidation(t *testing.T) {
	t.Parallel()
	// Cassandra uses microsecond timestamps
	nowMicros := time.Now().UnixNano() / 1000
	pastMicros := nowMicros - 1000000 // 1 second in the past

	// 1. Insert a record with the current timestamp
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code, credited) VALUES (?, ?, ?, ?) USING TIMESTAMP ?`,
		"Charles", int64(55), 799, 456465.0, nowMicros).Exec()
	require.NoError(t, err, "Failed to insert record with timestamp")

	// 2. Attempt to update with a past timestamp (this should be ignored by Cassandra)
	err = session.Query(`UPDATE bigtabledevinstance.user_info USING TIMESTAMP ? SET code = ? WHERE name = ? AND age = ?`,
		pastMicros, 677, "Charles", int64(55)).Exec()
	require.NoError(t, err, "Update with past timestamp failed, though it should execute without error")

	// 3. Validate that the original value is retained
	var name string
	var age int64
	var code int
	err = session.Query(`SELECT name, age, code FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
		"Charles", int64(55)).Scan(&name, &age, &code)
	require.NoError(t, err, "Failed to select record for validation")

	assert.Equal(t, "Charles", name)
	assert.Equal(t, int64(55), age)
	assert.Equal(t, 799, code, "The record should not have been updated by the past timestamp")
}

func TestDataHandlingInsertUpdateAndValidateInUserInformation(t *testing.T) {
	t.Parallel()
	// 1. Insert initial record
	err := session.Query(`INSERT INTO bigtabledevinstance.user_info (name, age, code) VALUES (?, ?, ?)`,
		"Adam", int64(50), 713).Exec()
	require.NoError(t, err, "Failed to insert record")

	// 2. Update the 'credited' field
	err = session.Query(`UPDATE bigtabledevinstance.user_info SET credited = ? WHERE name = ? AND age = ? IF EXISTS`,
		8000.0, "Adam", int64(50)).Exec()
	require.NoError(t, err, "Failed to update 'credited' field")

	// 3. Validate the result
	var name string
	var age int64
	var code int
	var credited float64
	err = session.Query(`SELECT name, age, code, credited FROM bigtabledevinstance.user_info WHERE name = ? AND age = ?`,
		"Adam", int64(50)).Scan(&name, &age, &code, &credited)
	require.NoError(t, err, "Failed to select record for validation")

	assert.Equal(t, "Adam", name)
	assert.Equal(t, int64(50), age)
	assert.Equal(t, 713, code)
	assert.Equal(t, 8000.0, credited, "The 'credited' field was not updated correctly")
}

func TestNegativeTestCasesForUpdateOperations(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name          string
		query         string
		params        []interface{}
		expectedError string
	}{
		{
			name:          "Update with incorrect keyspace",
			query:         "UPDATE randomkeyspace.user_info SET code=? where name=? and age=?",
			params:        []interface{}{724, "Smith", int64(36)},
			expectedError: "keyspace 'randomkeyspace' does not exist",
		},
		{
			name:          "Update with nonexistent table",
			query:         "UPDATE bigtabledevinstance.random_table SET code=? where name=? and age=?",
			params:        []interface{}{724, "Smith", int64(36)},
			expectedError: "table random_table does not exist",
		},
		{
			name:          "Update with incorrect column name",
			query:         "UPDATE bigtabledevinstance.user_info SET random_column=? where name=? and age=?",
			params:        []interface{}{724, "Smith", int64(36)},
			expectedError: "unknown column 'random_column' in table",
		},
		{
			name:          "Update with missing primary key parts",
			query:         "UPDATE bigtabledevinstance.user_info SET code=? where name=?",
			params:        []interface{}{724, "Smith"},
			expectedError: "some primary key parts are missing: age",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := session.Query(tc.query, tc.params...).Exec()
			require.Error(t, err, "Expected an error but got none")
			// we don't care about validating the cassandra error message, just that we got an error
			if testTarget == TestTargetCassandra {
				return
			}
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}
