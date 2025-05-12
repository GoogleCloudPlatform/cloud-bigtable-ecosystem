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

  Table getTable(String tableId);
  ApiFuture<Table> getTableAsync(String tableId);

  ApiFuture<Table> modifyFamiliesAsync(ModifyColumnFamiliesRequest request);

  void close();

}
