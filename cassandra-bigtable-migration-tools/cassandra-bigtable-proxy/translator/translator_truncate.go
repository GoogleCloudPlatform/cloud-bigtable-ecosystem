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

package translator

import (
	"errors"

	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/antlr4-go/antlr/v4"
)

func (t *Translator) TranslateTruncateTableToBigtable(query, sessionKeyspace string) (*TruncateTableStatementMap, error) {
	lexer := cql.NewCqlLexer(antlr.NewInputStream(query))
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	p := cql.NewCqlParser(stream)

	truncateTableObj := p.Truncate()

	if truncateTableObj == nil || truncateTableObj.Table() == nil {
		return nil, errors.New("error while parsing truncate query")
	}

	var tableName, keyspaceName string

	if truncateTableObj != nil && truncateTableObj.Table() != nil && truncateTableObj.Table().GetText() != "" {
		tableName = truncateTableObj.Table().GetText()
		if !validTableName.MatchString(tableName) {
			return nil, errors.New("invalid table name parsed from query")
		}
	} else {
		return nil, errors.New("invalid truncate table query: table missing")
	}

	if truncateTableObj != nil && truncateTableObj.Keyspace() != nil && truncateTableObj.Keyspace().GetText() != "" {
		keyspaceName = truncateTableObj.Keyspace().GetText()
	} else if sessionKeyspace != "" {
		keyspaceName = sessionKeyspace
	} else {
		return nil, errors.New("missing keyspace. keyspace is required")
	}

	var stmt = TruncateTableStatementMap{
		QueryType: "truncate",
		Keyspace:  keyspaceName,
		Table:     tableName,
	}

	return &stmt, nil
}
