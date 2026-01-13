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
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
)

type TestTarget int

const (
	TestTargetProxy TestTarget = iota
	TestTargetCassandra
)

func (target TestTarget) String() string {
	switch target {
	case TestTargetProxy:
		return "bigtable-proxy"
	case TestTargetCassandra:
		return "cassandra"
	}
	return "unknown"
}

// session is a global variable to hold the database session.
var session *gocql.Session

var testTarget TestTarget
var hostAddress string

func createSession(keyspace string) (*gocql.Session, error) {
	hostAddress = os.Getenv("CASSANDRA_HOST")
	if hostAddress == "" {
		hostAddress = "127.0.0.1"
	}
	log.Printf("connecting to cluster at %s...\n", hostAddress)
	cluster := gocql.NewCluster(hostAddress) // Assumes Cassandra is running locally
	cluster.Timeout = 20 * time.Second
	if keyspace != "" {
		cluster.Keyspace = keyspace
	}

	return cluster.CreateSession()
}

// TestMain sets up the database connection and schema before running tests,
// and tears it down afterward.
func TestMain(m *testing.M) {
	setUpTests()

	fmt.Printf("running against test target '%s'\n", testTarget.String())

	// --- Run Tests ---
	m.Run()

	cleanUpTests()
}

// note: we don't destroy any tables here because recreating every table, for every test would cause Bigtable to rate limit us.
func cleanUpTests() {
	session.Close()
	err := client.Close()
	if err != nil {
		fmt.Printf("failed to close bigtable connection: %s", err.Error())
	}
}

var gcpProjectId = ""
var instanceId = ""
var client *bigtable.Client

func setUpTests() {
	gcpProjectId = os.Getenv("PROJECT_ID")
	if gcpProjectId == "" {
		log.Fatalf("missing env var 'PROJECT_ID'")
	}
	instanceId = os.Getenv("INSTANCE_ID")
	if instanceId == "" {
		log.Fatalf("missing env var 'INSTANCE_ID'")
	}

	var err error
	session, err = createSession("bigtabledevinstance")
	if err != nil {
		log.Fatalf("could not connect to the session: %v", err)
	}

	var clusterName string
	err = session.Query("SELECT cluster_name FROM system.local LIMIT 1").Scan(&clusterName)
	if err != nil {
		log.Fatalf("failed to determine test target type with error: %v", err)
		return
	}
	if strings.Contains(clusterName, "cassandra-bigtable-proxy-") {
		testTarget = TestTargetProxy
	} else {
		testTarget = TestTargetCassandra
	}
	log.Println(fmt.Sprintf("determined test target to be %s from the cluster name '%s'", testTarget.String(), clusterName))

	log.Println("Creating test tables...")
	err = runCqlshAsync(getSchemas(), testTarget != TestTargetCassandra)
	if err != nil {
		log.Fatalf("could not create table: %v", err)
	}

	tables, err := cqlshScanToMap("select * from system_schema.tables")
	if err != nil {
		log.Fatalf("could not read system tables: %v", err)
	}
	var tableNames []string
	for _, tableRow := range tables {
		keyspace := tableRow["keyspace_name"]
		table := tableRow["table_name"]
		// don't truncate system tables
		if keyspace != "bigtabledevinstance" {
			continue
		}
		tableNames = append(tableNames, table)
	}

	var truncateStatements []string
	for _, table := range tableNames {
		truncateStatements = append(truncateStatements, fmt.Sprintf("TRUNCATE TABLE %s", table))
	}

	err = runCqlshAsync(truncateStatements, testTarget != TestTargetCassandra)
	if err != nil {
		log.Fatalf("could not truncate table: %v", err)
	}

	log.Println("All test tables successfully created!")

	client, err = bigtable.NewClient(context.Background(), gcpProjectId, instanceId)
	if err != nil {
		log.Fatalf("could not open bigtable connection: %v", err)
	}
}

func cleanupTable(t *testing.T, table string) {
	err := session.Query(fmt.Sprintf("DROP TABLE IF EXISTS %s;", table)).Exec()
	assert.NoError(t, err, "testing cleanup of table %s failed", table)
}
