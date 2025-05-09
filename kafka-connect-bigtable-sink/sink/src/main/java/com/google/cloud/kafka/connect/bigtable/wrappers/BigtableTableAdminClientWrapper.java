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
  public ApiFuture<Table> getTableAsync(String tableId) {
    return tableAdminClient.getTableAsync(tableId);
  }

  @Override
  public ApiFuture<Table> modifyFamiliesAsync(ModifyColumnFamiliesRequest request) {
    return tableAdminClient.modifyFamiliesAsync(request);
  }

  @Override
  public void close() {
    tableAdminClient.close();
  }
}
