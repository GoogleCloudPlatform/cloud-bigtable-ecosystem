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
import com.google.cloud.bigtable.admin.v2.BigtableTableAdminClient;
import com.google.cloud.bigtable.admin.v2.models.CreateTableRequest;
import com.google.cloud.bigtable.admin.v2.models.ModifyColumnFamiliesRequest;
import com.google.cloud.bigtable.admin.v2.models.Table;
import java.util.List;

public class BigtableTableAdminClientWrapper implements BigtableTableAdminClientInterface {

  private final BigtableTableAdminClient tableAdminClient;

  public BigtableTableAdminClientWrapper(BigtableTableAdminClient tableAdminClient) {
    this.tableAdminClient = tableAdminClient;
  }

  @Override
  public List<String> listTables() {
    return tableAdminClient.listTables();
  }

  @Override
  public ApiFuture<Table> createTableAsync(CreateTableRequest request) {
    return tableAdminClient.createTableAsync(request);
  }

  @Override
  public Table createTable(CreateTableRequest request) {
    return tableAdminClient.createTable(request);
  }

  @Override
  public Table getTable(String tableId) {
    return tableAdminClient.getTable(tableId);
  }

  @Override
  public ApiFuture<Table> getTableAsync(String tableId) {
    return tableAdminClient.getTableAsync(tableId);
  }

  @Override
  public ApiFuture<Table> modifyFamiliesAsync(ModifyColumnFamiliesRequest request) {
    return tableAdminClient.modifyFamiliesAsync(request);
  }

  @Override
  public Table modifyFamilies(ModifyColumnFamiliesRequest request) {
    return tableAdminClient.modifyFamilies(request);
  }

  @Override
  public void close() {
    tableAdminClient.close();
  }
}
