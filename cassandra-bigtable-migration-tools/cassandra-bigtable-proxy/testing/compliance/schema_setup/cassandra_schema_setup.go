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

package schema_setup

import (
	"fmt"
	"os"
	"strings"

	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/testing/compliance/utility"
	"github.com/gocql/gocql"
)

// SetupCassandraSchema sets up the Cassandra keyspace and tables
func SetupCassandraSchema(session *gocql.Session, schemaFilePath, keyspace string) error {

	data, err := os.ReadFile(schemaFilePath)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %v", err)
	}

	// expects each statement to be terminated by a semicolon
	stmts := strings.Split(string(data), ";")
	for i, stmt := range stmts {
		stmt = strings.ReplaceAll(strings.TrimSpace(stmt), "\n", "")
		if stmt == "" {
			continue
		}
		utility.LogInfo(fmt.Sprintf("Running setup query %d: '%s'", i, stmt))
		err = session.Query(stmt).Exec()
		if err != nil {
			return fmt.Errorf("failed to run setup query %d because %w", i, err)
		}
	}
	utility.LogInfo("Cassandra schema setup completed successfully!")
	return nil
}

// createKeyspace creates a keyspace in Cassandra
func CreateKeyspace(session *gocql.Session, keyspace string) error {
	ddlStatementKeyspace := fmt.Sprintf(
		"CREATE KEYSPACE IF NOT EXISTS %s WITH REPLICATION = { 'class' : 'SimpleStrategy', 'replication_factor' : '1' };",
		keyspace,
	)
	utility.LogInfo(fmt.Sprintf("Creating keyspace: %s", ddlStatementKeyspace))
	if err := session.Query(ddlStatementKeyspace).Exec(); err != nil {
		return fmt.Errorf("could not create keyspace %s: %v", keyspace, err)
	}
	return nil
}
