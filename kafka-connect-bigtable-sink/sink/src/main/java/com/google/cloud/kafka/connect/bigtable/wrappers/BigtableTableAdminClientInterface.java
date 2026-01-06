/*
 * Copyright 2025 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package com.google.cloud.kafka.connect.bigtable.wrappers;

import com.google.api.core.ApiFuture;
import com.google.cloud.bigtable.admin.v2.models.CreateTableRequest;
import com.google.cloud.bigtable.admin.v2.models.ModifyColumnFamiliesRequest;
import com.google.cloud.bigtable.admin.v2.models.Table;
import java.util.List;

/**
 * This is an interface for com.google.cloud.bigtable.admin.v2.BigtableTableAdminClient which makes
 * mocking easier.
 */
public interface BigtableTableAdminClientInterface {

  List<String> listTables();

  ApiFuture<Table> createTableAsync(CreateTableRequest request);

  Table createTable(CreateTableRequest request);

  Table getTable(String tableId);

  ApiFuture<Table> getTableAsync(String tableId);

  ApiFuture<Table> modifyFamiliesAsync(ModifyColumnFamiliesRequest request);

  Table modifyFamilies(ModifyColumnFamiliesRequest request);

  void close();
}
