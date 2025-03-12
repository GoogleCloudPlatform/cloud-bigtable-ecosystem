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
	"os"
	"strings"
	"testing"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/docker/docker/api/types/container"
	"github.com/gocql/gocql"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/compliance/schema_setup"
	"github.com/ollionorg/cassandra-to-bigtable-proxy/compliance/utility"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	secretPath          = os.Getenv("INTEGRATION_TEST_CRED_PATH")
	credentialsFilePath = "/tmp/keys/service-account.json"
	credentialsPath     = "/tmp/keys/"
	containerImage      = "bigtable-adaptor:compliance-test"
)

var (
	isProxy                  = false
	testFileName             string
	logFileName              string
	enableFileLogging        = false
	currentTestExecutionFile string
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
// - Query: CQL statement to execute.
// - QueryType: Type of query (INSERT, SELECT, etc.).
// - QueryDesc: Description of the query's intent.
// - Params: Parameters to bind to the query.
// - ExpectedResult: Anticipated output for validation.
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
		setupAndRunBigtableProxy(m)
	case "local":
		isProxy = true
		setupAndRunBigtableProxyLocal(m)
	case "cassandra":
		isProxy = false
		exitCode = setupAndRunCassandra(m)

	case "cassandralocal":
		isProxy = false
		exitCode = setupAndRunCassandraLocal(m)
	default:
		utility.LogFatal(fmt.Sprintf("Invalid target - %s", target))
	}
}

// setupAndRunBigtableProxy() initializes and starts the Bigtable Proxy environment within a Docker container.
// It ensures that the required GCP credentials are available, starts a test container running the Bigtable
// Proxy, and establishes a Cassandra-compatible session for testing.
//
// Steps:
//  1. Fetches and stores GCP credentials by calling `fetchAndStoreCredentials`.
//  2. Configures and starts a Docker container using the Bigtable Proxy image with the appropriate environment
//     variables and credential bindings.
//  3. Waits for the container to initialize by monitoring the log output for a specific message indicating readiness.
//  4. Retrieves the container's host and mapped ports, and creates a new Cassandra session using these details.
//  5. Runs the test suite, then terminates the container and cleans up resources.
//
// If the container fails to start or if Cassandra cannot connect to the proxy, the function logs the error and exits.
func setupAndRunBigtableProxy(m *testing.M) int {
	ctx := context.Background()
	if err := fetchAndStoreCredentials(ctx); err != nil {
		utility.LogFatal(fmt.Sprintf("error while setting up gcp credentials - %v", err))
		return 1
	}

	if GCP_PROJECT_ID == "" || BIGTABLE_INSTANCE == "" {
		utility.LogFatal("Env `GCP_PROJECT_ID` and `BIGTABLE_INSTANCE` is mandatory")
		return 1
	}

	// Setup Bigtable schema and test environment
	err := schema_setup.SetupBigtableSchema(GCP_PROJECT_ID, BIGTABLE_INSTANCE, ZONE, "schema_setup/schema.json")
	if err != nil {
		utility.LogFatal(fmt.Sprintf("Error while setting bigtable schema- %v", err))
		return 1
	}

	// Request a testcontainers.Container object
	req := testcontainers.ContainerRequest{
		Image:        containerImage,
		ExposedPorts: []string{"9042/tcp"},
		Env: map[string]string{
			"GOOGLE_APPLICATION_CREDENTIALS": "/tmp/keys/service-account.json",
		},
		HostConfigModifier: func(hostConfig *container.HostConfig) {
			hostConfig.Binds = []string{
				fmt.Sprintf("%s:/tmp/keys/service-account.json", credentialsFilePath),
			}
		},
		WaitingFor: wait.ForLog("proxy is listening").WithStartupTimeout(60 * time.Second),
	}

	proxyContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	if err != nil {
		utility.LogFatal(fmt.Sprintf("Could not start container: %v", err))
		return 1
	}
	defer func() {
		utility.LogInfo("Terminating test container...")
		err := proxyContainer.Terminate(ctx)
		if err != nil {
			utility.LogFatal(fmt.Sprintf("error while terminating - %v", err.Error()))
		}
	}()

	host, err := proxyContainer.Host(ctx)
	if err != nil {
		utility.LogFatal(fmt.Sprintf("Could not get container host: %v", err))
		// Cleanup
		session.Close()
		if err := schema_setup.TeardownBigtableInstance(GCP_PROJECT_ID, BIGTABLE_INSTANCE); err != nil {
			utility.LogFatal(fmt.Sprintf("Error during Bigtable teardown: %v", err))
			return 1
		} else {
			utility.LogInfo("Bigtable teardown completed successfully.")
		}
		return 1
	}

	mappedPort, err := proxyContainer.MappedPort(ctx, "9042")
	if err != nil {
		utility.LogFatal(fmt.Sprintf("Could not get mapped port: %v", err))
		// Cleanup
		session.Close()
		if err := schema_setup.TeardownBigtableInstance(GCP_PROJECT_ID, BIGTABLE_INSTANCE); err != nil {
			utility.LogFatal(fmt.Sprintf("Error during Bigtable teardown: %v", err))
			return 1
		} else {
			utility.LogInfo("Bigtable teardown completed successfully.")
		}
		return 1
	}

	cluster := gocql.NewCluster(host)
	cluster.Port = mappedPort.Int()
	cluster.Keyspace = BIGTABLE_INSTANCE
	cluster.ProtoVersion = 4

	session, err = cluster.CreateSession()
	if err != nil {
		utility.LogFatal(fmt.Sprintf("Could not connect to Cassandra: %v", err))
		// Cleanup
		session.Close()
		if err := schema_setup.TeardownBigtableInstance(GCP_PROJECT_ID, BIGTABLE_INSTANCE); err != nil {
			utility.LogFatal(fmt.Sprintf("Error during Bigtable teardown: %v", err))
			return 1
		} else {
			utility.LogInfo("Bigtable teardown completed successfully.")
		}
		return 1
	}
	defer session.Close()

	// Run tests
	code := m.Run()

	// Cleanup
	session.Close()
	if err := schema_setup.TeardownBigtableInstance(GCP_PROJECT_ID, BIGTABLE_INSTANCE); err != nil {
		utility.LogFatal(fmt.Sprintf("Error during Bigtable teardown: %v", err))
		return 1
	} else {
		utility.LogInfo("Bigtable teardown completed successfully.")
	}
	return code
}
func setupAndRunBigtableProxyLocal(m *testing.M) {

	cluster := gocql.NewCluster("localhost")
	cluster.Port = 9042
	cluster.Keyspace = BIGTABLE_INSTANCE
	cluster.ProtoVersion = 4

	session, _ = cluster.CreateSession()

	defer session.Close()

	// Run tests
	code := m.Run()

	// Cleanup
	session.Close()
	os.Exit(code)
}

// setupAndRunCassandra() sets up and runs a local Cassandra environment using a Docker container.
// It configures the container, initializes the Cassandra database, and ensures the appropriate schema
// (keyspaces and tables) are created before running the tests.
//
// Steps:
// 1. Creates a Docker container running the latest version of Cassandra.
// 2. Exposes the default Cassandra CQL port (9042) and configures the environment variables for memory tuning.
// 3. Waits for the container to fully initialize by monitoring the logs for a message indicating CQL client readiness.
// 4. Retrieves the host and mapped port of the running Cassandra container.
// 5. Creates a new Cassandra session and initializes the database schema (keyspaces and tables).
// 6. Executes the test suite after the environment is ready.
// 7. Cleans up by terminating the container after the tests are complete.
//
// This function ensures Cassandra is provisioned with the necessary tables and data types for integration testing.
func setupAndRunCassandra(m *testing.M) int {
	// Create a context with a 5-minute timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Fetch and store GCP credentials if needed
	if err := fetchAndStoreCredentials(ctx); err != nil {
		utility.LogFatal(fmt.Sprintf("Error while setting up GCP credentials - %v", err))
	}

	// Define the container request
	req := testcontainers.ContainerRequest{
		Image:        "cassandra:latest",
		ExposedPorts: []string{"9042/tcp"}, // Expose the default Cassandra CQL port
		Env: map[string]string{
			"MAX_HEAP_SIZE": "512M", // Optional tuning for Cassandra
			"HEAP_NEWSIZE":  "100M",
		},
		HostConfigModifier: func(hostConfig *container.HostConfig) {
			// Optional: Mount GCP credentials if needed
			hostConfig.Binds = []string{
				fmt.Sprintf("%s:/tmp/keys/service-account.json", credentialsFilePath),
			}
		},
		WaitingFor: wait.ForLog("Starting listening for CQL clients on /0.0.0.0:9042").WithStartupTimeout(120 * time.Second), // Wait until Cassandra is ready
	}

	// Start the Cassandra container
	cassandraContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		utility.LogFatal(fmt.Sprintf("Could not start container: %v", err))
		return 1
	}
	defer func() {
		utility.LogInfo("Terminating test container...")
		// Ensure the container is terminated after tests
		if err := cassandraContainer.Terminate(ctx); err != nil {
			utility.LogFatal(fmt.Sprintf("Error while terminating container - %v", err))
		}
	}()

	// Get the container host and mapped port for Cassandra
	host, err := cassandraContainer.Host(ctx)
	if err != nil {
		utility.LogFatal(fmt.Sprintf("Could not get container host: %v", err))
		return 1
	}

	mappedPort, err := cassandraContainer.MappedPort(ctx, "9042")
	if err != nil {
		utility.LogFatal(fmt.Sprintf("Could not get mapped port: %v", err))
		return 1
	}

	// Configure and create a new Cassandra cluster session
	cluster := gocql.NewCluster(host)
	cluster.Port = mappedPort.Int()           // Use the mapped port from the container
	cluster.ProtoVersion = 4                  // Use protocol version 4
	cluster.ConnectTimeout = 30 * time.Second // Set connection timeout
	cluster.Keyspace = "system"               // Use the default 'system' keyspace initially

	// Create a session with Cassandra
	session, err = cluster.CreateSession()
	if err != nil {
		utility.LogFatal(fmt.Sprintf("Could not connect to Cassandra: %v", err))
		return 1
	}
	defer session.Close()

	// Setup the Cassandra schema
	if err := schema_setup.SetupCassandraSchema(session, "schema_setup/schema.json", BIGTABLE_INSTANCE); err != nil {
		utility.LogFatal(fmt.Sprintf("Error setting up Cassandra schema: %v", err))
		return 1
	}

	// Run the tests
	code := m.Run()

	// Cleanup and exit
	session.Close()
	return code
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
	currentTestExecutionFile = fileName
	if err := executeQuery(t, testCases); err != nil {
		utility.LogWarning(t, fmt.Sprintf("Test execution failed for %s: %v\n", fileName, err))
		return false
	}

	utility.LogInfo(fmt.Sprintf("All test cases in %s executed successfully.\n", fileName))
	return true
}

// executeQuery
// Iterates through each test case and delegates execution based on the test type (BATCH or DML).
// Logs success or failure for each test case and provides a structured test execution flow.
func executeQuery(t *testing.T, testCases []TestCase) error {
	allPassed := true
	logger := log.New(os.Stdout, "", log.LstdFlags)

	for _, testCase := range testCases {
		utility.LogInfo(fmt.Sprintf("Executing Test: %s\nDescription: %s\n", testCase.Title, testCase.Description))
		var testCaseResult bool
		switch strings.ToLower(testCase.Kind) {
		case "batch":
			testCaseResult = executeBatchTestCases(t, testCase, logger)
		default:
			testCaseResult = executeDMLTestCases(t, testCase)
		}
		if !testCaseResult {
			allPassed = false
		}
		utility.LogInfo(fmt.Sprintf("Success: %s\n", testCase.SuccessMessage))
		utility.LogSectionDivider()
	}
	if !allPassed {
		return fmt.Errorf("some test cases failed")
	}
	return nil
}

// executeDMLTestCases
// Handles Data Manipulation Language (DML) operations such as INSERT, SELECT, UPDATE, and DELETE.
// Routes each operation to the appropriate handler function based on the query type.
func executeDMLTestCases(t *testing.T, testCase TestCase) bool {
	allPassed := true
	for _, operation := range testCase.Operations {
		var result bool
		switch strings.ToLower(operation.QueryType) {
		case "insert":
			result = executeInsert(t, operation)
		case "select":
			result = executeSelect(t, operation)
		case "update":
			result = executeUpdate(t, operation)
		case "delete":
			result = executeDelete(t, operation)
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
func executeInsert(t *testing.T, operation Operation) bool {
	params := utility.ConvertParams(operation.Params)
	err := session.Query(operation.Query, params...).Exec()

	if err != nil {
		if len(operation.ExpectedResult) > 0 {
			// Check if an error is expected
			if expectedErr, ok := operation.ExpectedResult[0]["expect_error"].(bool); ok && expectedErr {
				expectedErrMsg, _ := operation.ExpectedResult[0]["error_message"].(string)
				// Compare the actual error message with the expected error message
				if strings.EqualFold(err.Error(), expectedErrMsg) {
					utility.LogInfo(fmt.Sprintf("Query failed as expected with error: %v", err))
					utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
					return true

				} else {
					utility.LogError(t, currentTestExecutionFile, operation.QueryDesc, operation.Query, err)
					utility.LogWarning(t, fmt.Sprintf("Error message mismatch:\nExpected: '%s'\nActual:   '%s'\n", expectedErrMsg, err.Error()))
					return false
				}
			}
		}

		// If no error was expected but an error occurred, log it as an error
		utility.LogError(t, currentTestExecutionFile, operation.QueryDesc, operation.Query, err)
		return false
	}
	utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
	return true
}

// executeSelect
// Executes SELECT queries and delegates the result validation to a separate function.
// Logs errors if query execution fails, otherwise validates the query results.
func executeSelect(t *testing.T, operation Operation) bool {
	params := utility.ConvertParams(operation.Params)
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
		return handleSelectError(t, err, operation)

	}

	// Delegate result validation
	return validateSelectResults(t, results, operation)
}

// handleSelectError
// Handles errors for SELECT operations, including cases like unknown functions or expected errors from test cases.
func handleSelectError(t *testing.T, err error, operation Operation) bool {
	// Handle unknown function errors gracefully
	if strings.Contains(strings.ToLower(err.Error()), "unknown function") {
		utility.LogInfo(fmt.Sprintf("Query failed as expected with error: %v", err))
		utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
		return true
	}

	if !isProxy && operation.ExpectCassandraSpecificError != "" {
		if strings.EqualFold(err.Error(), operation.ExpectCassandraSpecificError) {
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
				utility.LogFailure(t, currentTestExecutionFile, operation.QueryDesc, operation.Query, fmt.Sprintf("Expected error '%s' but got '%s'", expectedErrMsg, err.Error()))
				return false
			}

		}
	}

	// Log unexpected errors
	utility.LogError(t, currentTestExecutionFile, operation.QueryDesc, operation.Query, err)
	return false
}

// validateSelectResults
// Compares the retrieved SELECT results with the expected outcome and logs success or failure.
// Handles row count validation if WHERE clause is missing from the query.
func validateSelectResults(t *testing.T, results []map[string]interface{}, operation Operation) bool {
	var eResult []map[string]interface{}

	if operation.ExpectedMultiRowResult != nil {
		for _, value := range *operation.ExpectedMultiRowResult {
			convertedResult := utility.ConvertExpectedResult(value)
			eResult = append(eResult, convertedResult...)
		}
	} else {
		eResult = utility.ConvertExpectedResult(operation.ExpectedResult)
	}

	// Check if query has WHERE clause – If not, validate row count
	if !strings.Contains(strings.ToLower(operation.Query), "where") {
		expectedCount, _ := eResult[0]["row_count"].(int)
		if len(results) >= expectedCount {
			utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
			return true
		} else {
			utility.LogFailure(t, currentTestExecutionFile, operation.QueryDesc, operation.Query, fmt.Sprintf("Expected at least %d rows but got %d", expectedCount, len(results)))
			return false
		}
	}

	// Perform standard result comparison for queries with WHERE clause

	var diff string
	if operation.DefaultDiff != nil && *operation.DefaultDiff {
		diff = cmp.Diff(results, eResult)
	} else {
		diff = cmp.Diff(results, eResult, customComparer, cmpopts.SortSlices(func(x, y any) bool {
			return fmt.Sprintf("%v", x) < fmt.Sprintf("%v", y)
		}))
	}

	if diff != "" {
		utility.LogFailure(t, currentTestExecutionFile, operation.QueryDesc, operation.Query, diff)
		return false
	} else {
		utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
		return true
	}
}

// executeUpdate
// Executes UPDATE queries with provided parameters and logs success or failure accordingly.
// Errors during query execution are logged immediately.
func executeUpdate(t *testing.T, operation Operation) bool {
	params := utility.ConvertParams(operation.Params)
	err := session.Query(operation.Query, params...).Exec()

	if err != nil {
		if expectedErr, ok := operation.ExpectedResult[0]["expect_error"].(bool); ok && expectedErr {
			expectedErrMsg, _ := operation.ExpectedResult[0]["error_message"].(string)
			if strings.EqualFold(err.Error(), expectedErrMsg) {
				utility.LogInfo(fmt.Sprintf("Query failed as expected with error: %v", err))
				utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
				return true

			} else {
				utility.LogError(t, currentTestExecutionFile, operation.QueryDesc, operation.Query, err)
				utility.LogWarning(t, fmt.Sprintf("Error message mismatch:\nExpected: '%s'\nActual:   '%s'\n", expectedErrMsg, err.Error()))
				return false
			}
		}
		utility.LogError(t, currentTestExecutionFile, operation.QueryDesc, operation.Query, err)
		return false
	}
	utility.LogSuccess(t, operation.QueryDesc, operation.QueryType)
	return true
}

// executeDelete
// Executes DELETE queries and verifies errors for operations involving USING TIMESTAMP when applicable.
// If an error is expected (such as with Bigtable limitations), the function checks for the correct error message.
func executeDelete(t *testing.T, operation Operation) bool {
	params := utility.ConvertParams(operation.Params)
	iter := session.Query(operation.Query, params...).Iter()

	if err := iter.Close(); err != nil {

		if expectedErr, ok := operation.ExpectedResult[0]["expect_error"].(bool); ok && expectedErr {
			expectedErrMsg := operation.ExpectedResult[0]["error_message"]
			containsTimestamp := strings.Contains(strings.ToUpper(operation.Query), "USING TIMESTAMP")

			if expectedErrMsg != "" {
				// If the error contains a timestamp and it's a proxy, handle it differently
				if containsTimestamp && !isProxy {
					utility.LogWarning(t, fmt.Sprintf("expected no error for Cassandra. got %v", err))
					return false
				}
				// Check if the error message matches the expected error message (case-insensitive)
				if strings.EqualFold(strings.TrimSpace(err.Error()), strings.TrimSpace(expectedErrMsg.(string))) {
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
		utility.LogError(t, currentTestExecutionFile, operation.QueryDesc, operation.Query, err)
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
func executeBatchTestCases(t *testing.T, testCase TestCase, logger *log.Logger) bool {
	batch := session.NewBatch(gocql.LoggedBatch)
	var validateQuery *Operation

	for _, operation := range testCase.Operations {
		if strings.ToLower(operation.QueryType) == "batch" {
			params := utility.ConvertParams(operation.Params)
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
		return validateBatchResults(t, *validateQuery)
	}
	return true
}

// validateBatchResults performs a post-batch validation by running a SELECT query to check
// if the inserted/updated records match the expected result.
// Differences trigger failure logs, while matching records log a success message.
func validateBatchResults(t *testing.T, validateQuery Operation) bool {
	params := utility.ConvertParams(validateQuery.Params)
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
		utility.LogError(t, currentTestExecutionFile, validateQuery.QueryDesc, validateQuery.Query, err)
		return false
	}
	eResult := utility.ConvertExpectedResult(validateQuery.ExpectedResult)
	if diff := cmp.Diff(results, eResult); diff != "" {
		utility.LogFailure(t, currentTestExecutionFile, validateQuery.QueryDesc, validateQuery.Query, diff)
		return false
	}
	utility.LogSuccess(t, validateQuery.QueryDesc, validateQuery.QueryType)
	return true
}
