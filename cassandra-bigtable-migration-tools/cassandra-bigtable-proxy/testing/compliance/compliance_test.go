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
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/testing/compliance/schema_setup"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/testing/compliance/utility"
	"github.com/gocql/gocql"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var (
	secretPath          = os.Getenv("INTEGRATION_TEST_CRED_PATH")
	credentialsFilePath = "/tmp/keys/service-account.json"
	credentialsPath     = "/tmp/keys/"
	containerImage      = os.Getenv("CONTAINER_IMAGE")
)

var (
	isProxy      = false
	testFileName string
)

var (
	GCP_PROJECT_ID    = os.Getenv("GCP_PROJECT_ID")
	BIGTABLE_INSTANCE = os.Getenv("BIGTABLE_INSTANCE")
	ZONE              = os.Getenv("ZONE")
)

var (
	session        *gocql.Session
	customComparer = cmp.FilterValues(func(x, y interface{}) bool {
		_, xOk := x.([]map[string]interface{})
		_, yOk := y.([]map[string]interface{})
		return xOk && yOk
	}, cmp.Comparer(func(x, y []map[string]interface{}) bool {
		if len(x) == 0 && len(y) == 0 {
			return true
		}
		return cmp.Equal(x, y)
	}))
)

// Operation struct represents an individual database query executed as part of a test case.
// It supports parameter binding and result validation.
//
// Fields:
// - Query: The CQL statement to be executed.
// - QueryType: The type of query (e.g., INSERT, SELECT, UPDATE, DELETE).
// - QueryDesc: A brief description of the query's intent.
// - Params: A list of key-value pairs representing the parameters to bind to the query.
// - ExpectedResult: The anticipated single-row result for validation.
// - ExpectedMultiRowResult: The expected multi-row result, if applicable (optional).
// - ExpectCassandraSpecificError: The expected Cassandra-specific error message, if any (optional).
// - DefaultDiff: Indicates whether default diffing logic should be applied during result validation (optional).
// - IsInOperation: Specifies whether the query involves an "IN" operation (optional).
type Operation struct {
	Query                        string                      `json:"query"`
	QueryType                    string                      `json:"query_type"`
	QueryDesc                    string                      `json:"query_desc"`
	Params                       []map[string]interface{}    `json:"params"`
	ExpectedResult               []map[string]interface{}    `json:"expected_result"`
	ExpectedMultiRowResult       *[][]map[string]interface{} `json:"expected_multi_row_result,omitempty"`
	ExpectCassandraSpecificError string                      `json:"expect_cassandra_specific_error,omitempty"`
	DefaultDiff                  *bool                       `json:"default_diff,omitempty"`
	IsInOperation                *bool                       `json:"is_in_operation,omitempty"`
}

// TestCase struct defines a single test scenario consisting of multiple database operations.
// It supports various types of queries (DML, BATCH) and provides success/failure messages for validation.
//
// Fields:
// - Title: Test case name.
// - Description: Purpose and scope of the test.
// - Kind: Type of test (e.g., "batch" or "dml").
// - Operations: List of queries to execute sequentially.
// - SuccessMessage: Message to display on success.
// - FailureMessage: Message to display on failure.
type TestCase struct {
	Title          string      `json:"title"`
	Description    string      `json:"description"`
	Kind           string      `json:"kind"`
	Operations     []Operation `json:"operations"`
	SuccessMessage string      `json:"success_message"`
	FailureMessage string      `json:"failure_message"`
}

// fetchAndStoreCredentials() retrieves GCP service account credentials from Google Secret Manager,
// stores them locally, and writes the credentials to a file for use by the application.
// This function ensures that required credentials are available for authentication in the integration test environment.
//
// Steps:
// 1. Validates if the secret path is set through the environment variable `INTEGRATION_TEST_CRED_PATH`.
// 2. Initializes a Secret Manager client.
// 3. Accesses the specified secret version to retrieve the credential payload.
// 4. Creates a local directory to store the credentials if it doesn't already exist.
// 5. Writes the credentials to a file (`service-account.json`) for subsequent use by the Bigtable proxy or Cassandra container.
//
// Returns an error if any step fails, providing detailed information about the failure.
func fetchAndStoreCredentials(ctx context.Context) error {
	if secretPath == "" {
		return fmt.Errorf("ENV INTEGRATION_TEST_CRED_PATH IS NOT SET")
	}
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create secret manager client: %v", err)
	}
	defer client.Close()

	accessRequest := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretPath,
	}

	result, err := client.AccessSecretVersion(ctx, accessRequest)
	if err != nil {
		return fmt.Errorf("failed to access secret version: %v", err)
	}

	if err := os.MkdirAll(credentialsPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	credentials := string(result.Payload.Data)
	err = os.WriteFile(credentialsFilePath, []byte(credentials), 0644)
	if err != nil {
		return fmt.Errorf("failed to write credentials to file: %v", err)
	}

	return nil
}

// TestMain() is the entry point for the test suite. It configures the environment based on command-line
// flags, sets up the appropriate testing target (either Bigtable Proxy or Cassandra), and initiates the tests.
//
// This function is triggered by the Go testing framework and is responsible for:
// 1. Parsing command-line flags to determine the target environment (`proxy` or `cassandra`) and test file name.
// 2. Setting up the environment and selecting the correct configuration for Bigtable or Cassandra based on the flag.
// 3. Routing the execution to the appropriate setup function (`setupAndRunBigtableProxy` or `setupAndRunCassandra`).
// 4. Running the tests after the environment is ready.
//
// If an invalid target is specified, the function logs an error and terminates the test run.
func TestMain(m *testing.M) {
	var exitCode int
	defer func() {
		// Ensure cleanup runs before exiting
		if r := recover(); r != nil {
			utility.LogFatal(fmt.Sprintf("Recovered from panic: %v", r))
			exitCode = 1 // Set a non-zero exit code for failure
		}
		os.Exit(exitCode)
	}()

	var target string
	var logFileName string
	var enableFileLogging bool
	flag.StringVar(&target, "target", "proxy", "Specify the test target: 'proxy' or 'cassandra'")
	flag.StringVar(&testFileName, "testFileName", "", "Specify the test file name to execute the test scenarios. Leave empty to run all *_test.json files.")
	flag.BoolVar(&enableFileLogging, "enableFileLogging", false, "Enable logging to a file (true/false). Defaults to false (console logging).")
	flag.StringVar(&logFileName, "logFileName", "", "Specify the log file path (defaults to ./test_execution.log).")
	flag.Parse()

	logFile, err := utility.SetupLogger(logFileName, enableFileLogging)
	if err != nil {
		log.Fatalf("Error setting up logger: %v", err)
	}
	// Ensure the log file is closed at the end
	if logFile != nil {
		defer logFile.Close()
	}

	switch target {
	case "proxy":
		isProxy = true
		setupAndRunBigtableProxyLocal(m)
	case "cassandra":
		isProxy = false
		exitCode = setupAndRunCassandraLocal(m)
	default:
		utility.LogFatal(fmt.Sprintf("Invalid target - %s", target))
	}
}

func setupAndRunBigtableProxyLocal(m *testing.M) {
	cluster := gocql.NewCluster("localhost")
	cluster.Port = 9042
	cluster.Keyspace = BIGTABLE_INSTANCE
	cluster.ProtoVersion = 4

	session, _ = cluster.CreateSession()

	defer session.Close()

	err := schema_setup.SetupBigtableSchema(session, "schema_setup/setup.sql")
	if err != nil {
		utility.LogFatal(fmt.Sprintf("Error while setting bigtable schema- %v", err))
		return
	}

	// Run tests
	code := m.Run()

	// Cleanup
	session.Close()
	os.Exit(code)
}

func setupAndRunCassandraLocal(m *testing.M) int {

	// Configure and create a new Cassandra cluster session
	cluster := gocql.NewCluster("localhost")
	cluster.Port = 9042                       // Use the mapped port from the container
	cluster.ProtoVersion = 4                  // Use protocol version 4
	cluster.ConnectTimeout = 30 * time.Second // Set connection timeout
	cluster.Keyspace = "system"               // Use the default 'system' keyspace initially

	// Create a session with Cassandra
	session, _ = cluster.CreateSession()
	defer session.Close()

	var BIGTABLE_CASSANDRA_INSTANCE_MAPPING map[string]string
	err := json.Unmarshal([]byte(BIGTABLE_INSTANCE), &BIGTABLE_CASSANDRA_INSTANCE_MAPPING)
	if err != nil {
		utility.LogFatal(fmt.Sprintf("error while unmarshalling bigtable_cassandra_instance_mapping - %v", err))
		return 1
	}

	for key := range BIGTABLE_CASSANDRA_INSTANCE_MAPPING {
		if err := schema_setup.SetupCassandraSchema(session, "schema_setup/setup.sql", key); err != nil {
			utility.LogFatal(fmt.Sprintf("Error setting up Cassandra schema: %v", err))
			return 1
		}
	}

	// Run the tests
	code := m.Run()

	// Cleanup and exit
	session.Close()
	return code
}

// TestIntegration_ExecuteQuery
// Reads test cases from a JSON file and executes them sequentially.
// Fails the test if the JSON file cannot be read or parsed, or if any query execution fails.
func TestIntegration_ExecuteQuery(t *testing.T) {
	// Check if the specific file exists
	if testFileName == "" {
		utility.LogInfo("Test file not provided. Running all *_test.json files.\n")
		runAllTestFiles(t)
		return
	}

	// Run single test file if it exists
	runSingleTestFile(t, testFileName)
}

// runAllTestFiles() - Executes all test files matching *_test.json
func runAllTestFiles(t *testing.T) {
	files, err := os.ReadDir("./test_files/")
	if err != nil {
		utility.LogTestFatal(t, fmt.Sprintf("Failed to read directory: %v", err))
	}

	found := false
	allPassed := true
	for _, file := range files {
		if strings.HasSuffix(file.Name(), "_test.json") {
			found = true
			utility.LogInfo(fmt.Sprintf("Executing test file: %s\n", file.Name()))
			if !runSingleTestFile(t, file.Name()) {
				allPassed = false
			}
		}
	}

	if !found {
		utility.LogTestFatal(t, "No *_test.json files found in directory.")
	}

	if !allPassed {
		utility.LogWarning(t, "Some test files failed. Check the logs for details.")
	} else {
		utility.LogInfo("All test files executed successfully.")
	}
}

// runSingleTestFile() - Executes a single test file
func runSingleTestFile(t *testing.T, fileName string) bool {
	data, err := os.ReadFile(fmt.Sprintf("./test_files/%s", fileName))
	if err != nil {
		utility.LogWarning(t, fmt.Sprintf("Failed to read JSON file %s: %v\n", fileName, err))
		return false
	}

	var testCases []TestCase
	if err := json.Unmarshal(data, &testCases); err != nil {
		utility.LogWarning(t, fmt.Sprintf("Failed to parse JSON file %s: %v\n", fileName, err))
		return false
	}
	if err := executeQuery(t, testCases, fileName); err != nil {
		utility.LogWarning(t, fmt.Sprintf("Test execution failed for %s: %v\n", fileName, err))
		return false
	}

	utility.LogInfo(fmt.Sprintf("All test cases in %s executed successfully.\n", fileName))
	return true
}

// executeQuery
// Iterates through each test case and delegates execution based on the test type (BATCH or DML).
// Logs success or failure for each test case and provides a structured test execution flow.
func executeQuery(t *testing.T, testCases []TestCase, fileName string) error {
	allPassed := true
	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			utility.LogInfo(fmt.Sprintf("Executing Test: %s\nDescription: %s\n", testCase.Title, testCase.Description))
			var testCaseResult bool
			switch strings.ToLower(testCase.Kind) {
			case "batch":
				testCaseResult = executeBatchTestCases(t, testCase, fileName)
			default:
				testCaseResult = executeDMLTestCases(t, testCase, fileName)
			}
			if !testCaseResult {
				allPassed = false
			}
			utility.LogInfo(fmt.Sprintf("Success: %s\n", testCase.SuccessMessage))
			utility.LogSectionDivider()
		})
	}
	if !allPassed {
		return fmt.Errorf("some test cases failed")
	}
	return nil
}

// executeDMLTestCases
// Handles Data Manipulation Language (DML) operations such as INSERT, SELECT, UPDATE, and DELETE.
// Routes each operation to the appropriate handler function based on the query type.
func executeDMLTestCases(t *testing.T, testCase TestCase, fileName string) bool {
	allPassed := true
	for _, operation := range testCase.Operations {
		var result bool
		switch strings.ToLower(operation.QueryType) {
		case "insert":
			result = executeInsert(t, operation, fileName)
		case "select":
			result = executeSelect(t, operation, fileName)
		case "update":
			result = executeUpdate(t, operation, fileName)
		case "delete":
			result = executeDelete(t, operation, fileName)
		default:
			utility.LogWarning(t, fmt.Sprintf("Unknown operation type: %s", operation.QueryType))
			result = false
		}
		allPassed = allPassed && result
	}
	return allPassed
}

// executeInsert
// Executes INSERT queries by substituting parameter values and logging results.
// Logs an error if the query execution fails, otherwise logs the success message.
func executeInsert(t *testing.T, operation Operation, fileName string) bool {
	params := utility.ConvertParams(t, operation.Params, fileName, operation.Query)
	err := session.Query(operation.Query, params...).Exec()

	if err != nil {
		if len(operation.ExpectedResult) > 0 {
			// Check if an error is expected
			if expectedErr, ok := operation.ExpectedResult[0]["expect_error"].(bool); ok && expectedErr {
				expectedErrMsg, _ := operation.ExpectedResult[0]["error_message"].(string)
				expectedErrCassandraMsg, _ := operation.ExpectedResult[0]["cassandra_error_message"].(string)
				avoidCompErrCassandraMsg, _ := operation.ExpectedResult[0]["avoid_compare_error_message"].(bool)
				// Compare the actual error message with the expected error message
				if strings.EqualFold(err.Error(), expectedErrMsg) || (!isProxy && strings.EqualFold(err.Error(), expectedErrCassandraMsg)) || avoidCompErrCassandraMsg {
					utility.LogInfo(fmt.Sprintf("Query failed as expected with error: %v", err))
					utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
					return true

				} else {
					utility.LogError(t, fileName, operation.QueryDesc, operation.Query, err)
					utility.LogWarning(t, fmt.Sprintf("Error message mismatch:\nExpected: '%s'\nActual:   '%s'\n", expectedErrMsg, err.Error()))
					return false
				}
			}
		}

		// If no error was expected but an error occurred, log it as an error
		utility.LogError(t, fileName, operation.QueryDesc, operation.Query, err)
		return false
	}
	utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
	return true
}

// executeSelect
// Executes SELECT queries and delegates the result validation to a separate function.
// Logs errors if query execution fails, otherwise validates the query results.
func executeSelect(t *testing.T, operation Operation, fileName string) bool {
	params := utility.ConvertParams(t, operation.Params, fileName, operation.Query)
	iter := session.Query(operation.Query, params...).Iter()

	var results []map[string]interface{}
	if iter.NumRows() == 0 {
		results = nil
	} else {
		for {
			row := make(map[string]interface{})
			if !iter.MapScan(row) || len(row) == 0 {
				break
			}
			results = append(results, row)
		}
	}
	// Handle execution errors
	if err := iter.Close(); err != nil {
		return handleSelectError(t, err, operation, fileName)

	}

	// Delegate result validation
	return validateSelectResults(t, results, operation, fileName)
}

// handleSelectError
// Handles errors for SELECT operations, including cases like unknown functions or expected errors from test cases.
func handleSelectError(t *testing.T, err error, operation Operation, fileName string) bool {
	// Handle unknown function errors gracefully
	if strings.Contains(strings.ToLower(err.Error()), "unknown function") {
		utility.LogInfo(fmt.Sprintf("Query failed as expected with error: %v", err))
		utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
		return true
	}

	if !isProxy && operation.ExpectCassandraSpecificError != "" {
		avoidCompErrMsg := false
		if len(operation.ExpectedResult) > 0 {
			avoidCompErrMsg, _ = operation.ExpectedResult[0]["avoid_compare_error_message"].(bool)
		}

		if strings.EqualFold(err.Error(), operation.ExpectCassandraSpecificError) || avoidCompErrMsg {
			utility.LogInfo(fmt.Sprintf("Query failed as expected with error: %v\n", err))
			utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
			return true
		}
		utility.LogWarning(t, fmt.Sprintf("Expected error message '%s' but got '%s'\n", operation.ExpectCassandraSpecificError, err.Error()))
		return false
	}

	// Check if the error was expected
	if len(operation.ExpectedResult) > 0 {
		if expectedErr, ok := operation.ExpectedResult[0]["expect_error"].(bool); ok && expectedErr {
			expectedErrMsg, _ := operation.ExpectedResult[0]["error_message"].(string)
			avoidCompErrMsg, _ := operation.ExpectedResult[0]["avoid_compare_error_message"].(bool)
			if err.Error() == expectedErrMsg || avoidCompErrMsg {
				utility.LogInfo(fmt.Sprintf("Query failed as expected with error: %v", err))
				utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
				return true
			} else {
				utility.LogFailure(t, fileName, operation.QueryDesc, operation.Query, fmt.Sprintf("Expected error '%s' but got '%s'", expectedErrMsg, err.Error()))
				return false
			}

		}
	}

	// Log unexpected errors
	utility.LogError(t, fileName, operation.QueryDesc, operation.Query, err)
	return false
}

// validateSelectResults
// Compares the retrieved SELECT results with the expected outcome and logs success or failure.
// Handles row count validation if WHERE clause is missing from the query.
func validateSelectResults(t *testing.T, results []map[string]interface{}, operation Operation, fileName string) bool {
	var eResult []map[string]interface{}

	if operation.ExpectedMultiRowResult != nil {
		for _, value := range *operation.ExpectedMultiRowResult {
			convertedResult := utility.ConvertExpectedResult(value)
			eResult = append(eResult, convertedResult...)
		}
	} else {
		eResult = utility.ConvertExpectedResult(operation.ExpectedResult)
	}

	// Check if expectedResult contains row_count – validate row count
	if len(eResult) > 0 && operation.ExpectedMultiRowResult == nil {
		if expectedCount, ok := eResult[0]["row_count"].(int); ok {
			if len(results) >= expectedCount {
				utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
				return true
			} else {
				utility.LogFailure(t, fileName, operation.QueryDesc, operation.Query, fmt.Sprintf("Expected at least %d rows but got %d", expectedCount, len(results)))
				return false
			}
		}
	}

	// Perform standard result comparison for queries with WHERE clause

	var diff string
	if operation.DefaultDiff != nil && *operation.DefaultDiff {
		diff = cmp.Diff(results, eResult)
	} else if results == nil && eResult == nil {
		utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
		return true
	} else {
		if operation.ExpectedMultiRowResult != nil {
			if len(results) == len(eResult) {
				for _, result := range results {
					for key, val := range result {
						switch val := val.(type) {
						case float32:
							result[key] = float32(math.Round(float64(val)*1000) / 1000)
						case float64:
							result[key] = float64(math.Round(val*1000) / 1000)
						}
					}
					matched := false
					for index, expected := range eResult {
						if reflect.DeepEqual(result, expected) {
							matched = true
							// This code removes the matched expected result from the slice
							// This is to handle the case where the expected result contains multiple rows with the same values
							eResult = append(eResult[:index], eResult[index+1:]...)
							break
						}
					}
					if !matched {
						diff = fmt.Sprintf("Expected %v but got %v", eResult, results)
						break
					}
				}
			} else {
				diff = fmt.Sprintf("Expected %v but got %v", eResult, results)
			}
		} else {
			diff = cmp.Diff(results, eResult, customComparer, cmpopts.SortSlices(func(x, y any) bool {
				return fmt.Sprintf("%v", x) < fmt.Sprintf("%v", y)
			}))
		}
	}

	if diff != "" {
		utility.LogFailure(t, fileName, operation.QueryDesc, operation.Query, diff)
		return false
	} else {
		utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
		return true
	}
}

// executeUpdate
// Executes UPDATE queries with provided parameters and logs success or failure accordingly.
// Errors during query execution are logged immediately.
func executeUpdate(t *testing.T, operation Operation, fileName string) bool {
	params := utility.ConvertParams(t, operation.Params, fileName, operation.Query)
	err := session.Query(operation.Query, params...).Exec()

	if err != nil {
		if len(operation.ExpectedResult) > 0 {
			if expectedErr, ok := operation.ExpectedResult[0]["expect_error"].(bool); ok && expectedErr {
				expectedErrMsg, _ := operation.ExpectedResult[0]["error_message"].(string)
				expectedErrCassandraMsg, _ := operation.ExpectedResult[0]["cassandra_error_message"].(string)
				avoidCompErrCassandraMsg, _ := operation.ExpectedResult[0]["avoid_compare_error_message"].(bool)
				if strings.EqualFold(err.Error(), expectedErrMsg) || (!isProxy && strings.EqualFold(err.Error(), expectedErrCassandraMsg)) || avoidCompErrCassandraMsg {
					utility.LogInfo(fmt.Sprintf("Query failed as expected with error: %v", err))
					utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
					return true
				} else {
					utility.LogError(t, fileName, operation.QueryDesc, operation.Query, err)
					utility.LogWarning(t, fmt.Sprintf("Error message mismatch:\nExpected: '%s'\nActual:   '%s'\n", expectedErrMsg, err.Error()))
					return false
				}
			}
		}
		utility.LogError(t, fileName, operation.QueryDesc, operation.Query, err)
		return false
	}
	utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
	return true
}

// executeDelete
// Executes DELETE queries and verifies errors for operations involving USING TIMESTAMP when applicable.
// If an error is expected (such as with Bigtable limitations), the function checks for the correct error message.
func executeDelete(t *testing.T, operation Operation, fileName string) bool {
	params := utility.ConvertParams(t, operation.Params, fileName, operation.Query)
	iter := session.Query(operation.Query, params...).Iter()

	if err := iter.Close(); err != nil {

		if expectedErr, ok := operation.ExpectedResult[0]["expect_error"].(bool); ok && expectedErr {
			expectedErrMsg := operation.ExpectedResult[0]["error_message"]
			expectedErrCassandraMsg := operation.ExpectedResult[0]["cassandra_error_message"]
			avoidCompErrCassandraMsg, _ := operation.ExpectedResult[0]["avoid_compare_error_message"].(bool)
			containsTimestamp := strings.Contains(strings.ToUpper(operation.Query), "USING TIMESTAMP")

			if expectedErrMsg != "" || expectedErrCassandraMsg != "" {
				// If the error contains a timestamp and it's a proxy, handle it differently
				if containsTimestamp && !isProxy {
					utility.LogWarning(t, fmt.Sprintf("expected no error for Cassandra. got %v", err))
					return false
				}
				// Check if the error message matches the expected error message (case-insensitive)
				if strings.EqualFold(strings.TrimSpace(err.Error()), strings.TrimSpace(expectedErrMsg.(string))) || (expectedErrCassandraMsg != nil && (!isProxy && strings.EqualFold(strings.TrimSpace(err.Error()), strings.TrimSpace(expectedErrCassandraMsg.(string))))) || avoidCompErrCassandraMsg {
					utility.LogInfo(fmt.Sprintf("Query failed as expected with error: %v\n", err))
					utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
					return true
				}
				utility.LogWarning(t, fmt.Sprintf("Error message mismatch:\nExpected: '%s'\nActual:   '%s'\n", expectedErrMsg, err.Error()))
				return false
			}
			utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
			return true
		}
		utility.LogError(t, fileName, operation.QueryDesc, operation.Query, err)
		return false
	}
	utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
	return true
}

// executeBatchTestCases handles batch operations by executing multiple queries within a single batch.
// It iterates over all operations in the test case and groups "batch" type queries for collective execution.
// After batch execution, the function performs validation by running a SELECT (if provided) to ensure
// the batch operation applied the expected results to the database.
// Errors are logged, and differences between expected and actual results trigger failure messages.
func executeBatchTestCases(t *testing.T, testCase TestCase, fileName string) bool {
	batch := session.NewBatch(gocql.LoggedBatch)
	var validateQuery *Operation

	for _, operation := range testCase.Operations {
		if strings.ToLower(operation.QueryType) == "batch" {
			params := utility.ConvertParams(t, operation.Params, fileName, operation.Query)
			batch.Query(operation.Query, params...)
		} else if strings.ToLower(operation.QueryType) == "validate" {
			validateQuery = &operation // Store the validation query for later execution
		}
	}

	err := session.ExecuteBatch(batch)
	if err != nil {
		utility.LogWarning(t, fmt.Sprintf("Error while executing Batch - %s", err.Error()))
		return false
	}

	if validateQuery != nil {
		return validateBatchResults(t, *validateQuery, fileName)
	}
	return true
}

// validateBatchResults performs a post-batch validation by running a SELECT query to check
// if the inserted/updated records match the expected result.
// Differences trigger failure logs, while matching records log a success message.
func validateBatchResults(t *testing.T, validateQuery Operation, fileName string) bool {
	params := utility.ConvertParams(t, validateQuery.Params, fileName, validateQuery.Query)
	iter := session.Query(validateQuery.Query, params...).Iter()

	var results []map[string]interface{}
	for {
		row := make(map[string]interface{})
		if !iter.MapScan(row) {
			break
		}
		results = append(results, row)
	}

	if err := iter.Close(); err != nil {
		utility.LogError(t, fileName, validateQuery.QueryDesc, validateQuery.Query, err)
		return false
	}
	eResult := utility.ConvertExpectedResult(validateQuery.ExpectedResult)
	if diff := cmp.Diff(results, eResult); diff != "" {
		utility.LogFailure(t, fileName, validateQuery.QueryDesc, validateQuery.Query, diff)
		return false
	}
	utility.LogSuccess(t, validateQuery.QueryDesc, validateQuery.QueryType)
	return true
}
