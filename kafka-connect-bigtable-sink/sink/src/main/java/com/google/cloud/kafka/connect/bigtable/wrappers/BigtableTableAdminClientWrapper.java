package com.google.cloud.kafka.connect.bigtable.wrappers;

import com.google.api.core.ApiFuture;
import com.google.cloud.bigtable.admin.v2.BigtableTableAdminClient;
import com.google.cloud.bigtable.admin.v2.models.CreateTableRequest;
import com.google.cloud.bigtable.admin.v2.models.ModifyColumnFamiliesRequest;
import com.google.cloud.bigtable.admin.v2.models.Table;
import com.google.cloud.bigtable.data.v2.models.TableId;
import com.google.cloud.kafka.connect.bigtable.utils.Utils;
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
  public ApiFuture<Table> getTableAsync(TableId tableId) {
    return tableAdminClient.getTableAsync(Utils.getTableIdString(tableId));
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
