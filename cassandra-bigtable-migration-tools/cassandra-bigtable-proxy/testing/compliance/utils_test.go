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
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
)

// Helper to parse RFC3339 timestamp strings used in the tests
func parseTime(t *testing.T, ts string) time.Time {
	t.Helper()
	parsedTime, err := time.Parse(time.RFC3339, ts)
	require.NoError(t, err, "Failed to parse timestamp string")
	return parsedTime.UTC()
}

// Helper to parse "YYYY-MM-DD HH:MM:SS" timestamp strings used in the tests.
func parseSimpleTime(t *testing.T, ts string) time.Time {
	t.Helper()
	parsedTime, err := time.Parse("2006-01-02 15:04:05", ts)
	require.NoError(t, err, "Failed to parse timestamp string")
	return parsedTime.UTC()
}

func uniqueTableName(prefix string) string {
	// add an underscore separator
	if len(prefix) != 0 && prefix[len(prefix)-1] != '_' {
		prefix = prefix + "_"
	}
	return prefix + strings.ReplaceAll(uuid.New().String(), "-", "_")
}

// note: table must have a non-key row_index column
// note: input map must have the same keys
func testLexOrder(t *testing.T, input []map[string]interface{}, table string) {
	require.NotEqual(t, 0, len(input), "input cannot be empty")

	// used to weed out duplicate keys which are not allowed
	var inputMap = make(map[string]int)
	for i, v := range input {
		existing, ok := inputMap[fmt.Sprintf("%v", v)]
		require.False(t, ok, fmt.Sprintf("duplicate input '%v' at index %d and %d. duplicates are not allowed and indicate an incorrectly ordered input.", v, existing, i))

		values := []interface{}{i}
		queryPlaceholders := []string{"?"}
		columns := []string{"row_index"}
		for _, k := range maps.Keys(v) {
			queryPlaceholders = append(queryPlaceholders, "?")
			columns = append(columns, k)
			values = append(values, v[k])
		}

		queryString := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(columns, ", "), strings.Join(queryPlaceholders, ", "))
		insertQuery := session.Query(queryString, values...)
		require.NoError(t, insertQuery.Exec(), "error inserting row %d", i)
		inputMap[fmt.Sprintf("%v", v)] = i
	}

	scanner := session.Query("SELECT row_index FROM " + table).Iter().Scanner()

	var results []int32 = nil
	for scanner.Next() {
		var rowIndex int32
		err := scanner.Scan(
			&rowIndex,
		)
		require.NoError(t, err)
		results = append(results, rowIndex)
	}

	require.Equal(t, len(input), len(results), "missing or extra result row?")
	for i, result := range results {
		require.True(t, int(result) >= 0 && int(result) < len(input), fmt.Sprintf("unexpected result index: %d your environment is probably not clean", result))
		assert.Equal(t, int32(i), result, fmt.Sprintf("out of order result: expected %v but got %v", input[i], input[result]))
	}
}

var plusMinusRegex = regexp.MustCompile(`^[-+]*$`)
var rowsRegex = regexp.MustCompile(`^\(\d+ rows\)$`)

func cqlshExec(query string) (string, error) {
	return cqlshExecWithKeyspace("", query)
}
func cqlshExecWithKeyspace(keyspace string, query string) (string, error) {

	var cmd *exec.Cmd
	if keyspace != "" {
		cmd = exec.Command("cqlsh", hostAddress, "-k", keyspace, "--request-timeout=60", "-e", query)
	} else {
		cmd = exec.Command("cqlsh", hostAddress, "--request-timeout=60", "-e", query)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// If the command fails, the error message from cqlsh is in the output.
		return "", fmt.Errorf("cqlsh command failed: %w\nOutput: %s", err, stderr.String())
	}

	errStr := stderr.String()
	if errStr != "" {
		return "", errors.New(errStr)
	}

	raw := stdout.String()
	var fixed []string = nil
	for _, s := range strings.Split(raw, "\n") {
		if strings.TrimSpace(s) == "" {
			continue
		}
		if strings.HasPrefix(s, "WARNING: cqlsh was built against ") {
			continue
		}
		// skip the table header line
		if plusMinusRegex.MatchString(s) {
			continue
		}
		// skip row count line
		if rowsRegex.MatchString(s) {
			continue
		}

		fixed = append(fixed, s)
	}
	return strings.Join(fixed, "\n"), nil
}

func cqlshDescribe(query string) ([]string, error) {
	output, err := cqlshExec(query)
	if err != nil {
		return nil, err
	}
	var results []string = nil
	for _, s := range strings.Split(output, " ") {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			continue
		}
		results = append(results, trimmed)
	}
	return results, nil
}

// executeCQLQuery runs a query using cqlsh and returns the result as a slice of maps.
// Each map represents a row, with keys being the column headers.
func cqlshScanToMap(query string) ([]map[string]string, error) {
	output, err := cqlshExec(query)
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(strings.NewReader(output))
	reader.Comma = '|'

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV output: %w", err)
	}

	// Check if we have at least a header row.
	if len(records) < 1 {
		return []map[string]string{}, nil // No data, just return an empty slice.
	}

	// The first record is the header.
	header := records[0]
	dataRows := records[1:]

	var results []map[string]string
	for _, row := range dataRows {
		if len(row) == 0 {
			continue
		}
		rowData := make(map[string]string)
		for i, value := range row {
			// Check if the row has the same number of columns as the header.
			if i < len(header) {
				rowData[strings.TrimSpace(header[i])] = strings.TrimSpace(value)
			}
		}
		results = append(results, rowData)
	}

	return results, nil
}

func runCqlshAsync(batch []string, async bool) error {
	// cassandra throws weird errors when DDL is executed in parallel, so unfortunately we have to do things synchronously for cassandra
	if !async {
		for i, stmt := range batch {
			log.Println(fmt.Sprintf("Running sql statement: %d...", i))
			err := session.Query(stmt).Exec()
			if err != nil {
				return err
			}
		}
		return nil
	}

	// Use WaitGroup to track the completion of all goroutines
	var wg sync.WaitGroup

	// Use a channel to handle errors from concurrent operations
	errCh := make(chan error, len(batch))

	for i, stmt := range batch {
		// Increment the WaitGroup counter for each statement we launch
		wg.Add(1)

		// Launch a goroutine for each statement
		go func(i int, stmt string) {
			// Decrement the counter when the goroutine finishes
			defer wg.Done()

			log.Println(fmt.Sprintf("Running sql statement: %d...", i))

			// Execute the database query
			err := session.Query(stmt).Exec()

			if err != nil {
				// If an error occurs, send it to the error channel
				errCh <- err
			}
		}(i, stmt) // Pass copies of i and stmt to the goroutine
	}

	// Close the error channel once all goroutines have finished
	// This must be done in a separate goroutine to avoid a deadlock
	go func() {
		wg.Wait()
		close(errCh)
	}()

	var resultError error
	// Check the error channel for any failures
	for err := range errCh {
		resultError = err
	}

	// Wait for all tables to be created (this line executes after all errors are checked)
	wg.Wait()

	return resultError
}
