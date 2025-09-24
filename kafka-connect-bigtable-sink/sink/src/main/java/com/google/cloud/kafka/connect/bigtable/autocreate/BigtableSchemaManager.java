/*
 * Copyright 2024 Google LLC
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
package com.google.cloud.kafka.connect.bigtable.autocreate;

import com.google.api.core.ApiFuture;
import com.google.api.gax.rpc.ApiException;
import com.google.api.gax.rpc.StatusCode;
import com.google.cloud.bigtable.admin.v2.models.ColumnFamily;
import com.google.cloud.bigtable.admin.v2.models.CreateTableRequest;
import com.google.cloud.bigtable.admin.v2.models.ModifyColumnFamiliesRequest;
import com.google.cloud.bigtable.admin.v2.models.Table;
import com.google.cloud.bigtable.data.v2.models.TableId;
import com.google.cloud.kafka.connect.bigtable.exception.InvalidBigtableSchemaModificationException;
import com.google.cloud.kafka.connect.bigtable.mapping.MutationData;
import com.google.cloud.kafka.connect.bigtable.utils.SinkResult;
import com.google.cloud.kafka.connect.bigtable.utils.Utils;
import com.google.cloud.kafka.connect.bigtable.wrappers.BigtableTableAdminClientInterface;
import com.google.common.annotations.VisibleForTesting;
import java.util.AbstractMap;
import java.util.ArrayList;
import java.util.Collection;
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.ExecutionException;
import java.util.stream.Collectors;
import org.apache.kafka.connect.errors.ConnectException;
import org.apache.kafka.connect.sink.SinkRecord;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * A class responsible for the creation of Cloud Bigtable {@link Table Table(s)} and
 * {@link ColumnFamily ColumnFamily(s)} needed by the transformed Kafka Connect records.
 *
 * <p>This class contains nontrivial logic since we try to avoid API calls if possible.
 *
 * <p>This class does not automatically rediscover deleted resources. If another user of the Cloud
 * Bigtable instance deletes a table or a column, the sink using an instance of this class to
 * auto-create the resources, might end up sending requests targeting nonexistent
 * {@link Table Table(s)} and/or {@link ColumnFamily ColumnFamily(s)}.
 */
public class BigtableSchemaManager implements AutoCloseable {

  @VisibleForTesting
  protected Logger logger = LoggerFactory.getLogger(BigtableSchemaManager.class);

  private final BigtableTableAdminClientInterface bigtable;

  /**
   * A {@link Map} storing the names of existing Cloud Bigtable tables as keys and existing column
   * families within these tables as the values.
   *
   * <p>We have a single data structure for table and column family caches to ensure that they are
   * consistent.<br> An {@link Optional#empty()} value means that a table exists, but we don't know
   * what column families it contains.
   */
  @VisibleForTesting
  protected Map<TableId, Optional<Set<String>>> tableNameToColumnFamilies;

  /**
   * The default constructor.
   *
   * @param bigtable The Cloud Bigtable admin client used to auto-create {@link Table Table(s)}
   *     and {@link ColumnFamily ColumnFamily(s)}.
   */
  public BigtableSchemaManager(BigtableTableAdminClientInterface bigtable) {
    this.bigtable = bigtable;
    tableNameToColumnFamilies = new HashMap<>();
  }

  /**
   * Ensures that all the {@link Table Table(s)} needed by the input records exist by attempting to
   * create the missing ones.
   *
   * @param recordsAndOutputs A {@link Map} containing {@link SinkRecord SinkRecord(s)} and
   *     their matching {@link MutationData} specifying which {@link Table Table(s)} need to exist.
   * @return A {@link ResourceCreationResult} containing {@link SinkRecord SinkRecord(s)} for whose
   *     {@link MutationData} auto-creation of {@link Table Table(s)} failed.
   */
  public Collection<SinkResult<MutationData>> ensureTablesExist(Collection<MutationData> recordsAndOutputs) {
    Map<TableId, List<SinkRecord>> recordsByTableNames = getTableNamesToRecords(recordsAndOutputs);

    Map<TableId, List<SinkRecord>> recordsByMissingTableNames =
        missingTablesToRecords(recordsByTableNames);
    if (recordsByMissingTableNames.isEmpty()) {
      return Collections.emptyList();
    }
    logger.debug("Missing {} tables", recordsByMissingTableNames.size());
    Map<ApiFuture<Table>, ResourceAndRecords<TableId>> recordsByCreateTableFutures =
        sendCreateTableRequests(recordsByMissingTableNames);
    // No cache update here since we create tables with no column families, so every (non-delete)
    // write to the table will need to create needed column families first, so saving the data from
    // the response gives us no benefit.
    // We ignore errors to handle races between multiple tasks of a single connector and refresh
    // the cache in a further step.
    Set<SinkRecord> dataErrors =
        awaitResourceCreationAndHandleInvalidInputErrors(
            recordsByCreateTableFutures, "Error creating a Cloud Bigtable table: %s");
    refreshTableNamesCache();
    Set<SinkRecord> bigtableErrors =
        missingTablesToRecords(recordsByMissingTableNames).values().stream()
            .flatMap(Collection::stream)
            .collect(Collectors.toSet());
    bigtableErrors.removeAll(dataErrors);

    var results = new HashMap<SinkRecord, SinkResult<MutationData>>();
    for (SinkRecord record : bigtableErrors) {
      results.putIfAbsent(record, SinkResult.failure(record,
          new ConnectException("Table auto-creation failed.")));
    }
    for (SinkRecord record : dataErrors) {
      results.putIfAbsent(record, SinkResult.failure(record,
          new InvalidBigtableSchemaModificationException("Table auto-creation failed.")));
    }
    return results.values();
  }

  /**
   * Ensures that all the {@link ColumnFamily ColumnFamily(s)} needed by the input records exist by
   * attempting to create the missing ones.
   *
   * <p>This method will not try to create missing {@link Table Table(s)} tables if some of the
   * needed ones do not exist, but it will handle that case gracefully.
   *
   * @param recordsAndOutputs A {@link Map} containing {@link SinkRecord SinkRecord(s)} and
   *     their matching {@link MutationData} specifying which {@link ColumnFamily ColumnFamily(s)}
   *     need to exist.
   * @return A {@link ResourceCreationResult} containing {@link SinkRecord SinkRecord(s)} for whose
   *     {@link MutationData} needed {@link Table Table(s)} are missing or auto-creation of
   *     {@link ColumnFamily ColumnFamily(s)} failed.
   */
  public Collection<SinkResult<MutationData>> ensureColumnFamiliesExist(
      Collection<MutationData> recordsAndOutputs) {
    Map<Map.Entry<TableId, String>, List<SinkRecord>> recordsByColumnFamilies =
        getTableColumnFamiliesToRecords(recordsAndOutputs);

    Map<Map.Entry<TableId, String>, List<SinkRecord>> recordsByMissingColumnFamilies =
        missingTableColumnFamiliesToRecords(recordsByColumnFamilies);
    if (recordsByMissingColumnFamilies.isEmpty()) {
      return Collections.emptyList();
    }
    logger.debug("Missing {} column families", recordsByMissingColumnFamilies.size());
    Map<ApiFuture<Table>, ResourceAndRecords<Map.Entry<TableId, String>>>
        recordsByCreateColumnFamilyFutures =
        sendCreateColumnFamilyRequests(recordsByMissingColumnFamilies);

    // No cache update here since the requests are handled by Cloud Bigtable in a random order.
    // We ignore errors to handle races between multiple tasks of a single connector
    // and refresh the cache in a further step.
    Set<SinkRecord> dataErrors =
        awaitResourceCreationAndHandleInvalidInputErrors(
            recordsByCreateColumnFamilyFutures, "Error creating a Cloud Bigtable column family %s");

    Set<TableId> tablesRequiringRefresh =
        recordsByMissingColumnFamilies.keySet().stream()
            .map(Map.Entry::getKey)
            .collect(Collectors.toSet());
    refreshTableColumnFamiliesCache(tablesRequiringRefresh);

    Map<Map.Entry<TableId, String>, List<SinkRecord>> missing =
        missingTableColumnFamiliesToRecords(recordsByMissingColumnFamilies);
    Set<SinkRecord> bigtableErrors =
        missing.values().stream().flatMap(Collection::stream).collect(Collectors.toSet());
    bigtableErrors.removeAll(dataErrors);

    var results = new HashMap<SinkRecord, SinkResult<MutationData>>();
    for (SinkRecord record : bigtableErrors) {
      results.putIfAbsent(record, SinkResult.failure(record,
          new ConnectException("Column family auto-creation failed.")));
    }
    for (SinkRecord record : dataErrors) {
      results.putIfAbsent(record, SinkResult.failure(record,
          new InvalidBigtableSchemaModificationException("Column family auto-creation failed.")));
    }
    return results.values();
  }

  /**
   * @param recordsAndOutputs A {@link Map} containing {@link SinkRecord SinkRecords} and
   *     corresponding Cloud Bigtable mutations.
   * @return A {@link Map} containing Cloud Bigtable table names and {@link SinkRecord SinkRecords}
   *     that need these tables to exist.
   */
  private static Map<TableId, List<SinkRecord>> getTableNamesToRecords(
      Collection<MutationData> recordsAndOutputs) {
    Map<TableId, List<SinkRecord>> tableNamesToRecords = new HashMap<>();
    for (MutationData mut : recordsAndOutputs) {
      SinkRecord record = mut.getRecord();
      List<SinkRecord> records =
          tableNamesToRecords.computeIfAbsent(mut.getTargetTable(), k -> new ArrayList<>());
      records.add(record);
    }
    return tableNamesToRecords;
  }

  /**
   * @param recordsAndOutputs A {@link Map} containing {@link SinkRecord SinkRecords} and
   *     corresponding Cloud Bigtable mutations.
   * @return A {@link Map} containing {@link Map.Entry Map.Entry(s)} consisting of Bigtable table
   *     names and column families and {@link SinkRecord SinkRecords} that need to use these tables
   *     and column families to exist.
   */
  private static Map<Map.Entry<TableId, String>, List<SinkRecord>> getTableColumnFamiliesToRecords(
      Collection<MutationData> recordsAndOutputs) {
    Map<Map.Entry<TableId, String>, List<SinkRecord>> tableColumnFamiliesToRecords = new HashMap<>();
    for (MutationData mut : recordsAndOutputs) {
      SinkRecord record = mut.getRecord();
      TableId tableName = mut.getTargetTable();
      for (String columnFamily : mut.getRequiredColumnFamilies()) {
        Map.Entry<TableId, String> key =
            new AbstractMap.SimpleImmutableEntry<>(tableName, columnFamily);
        List<SinkRecord> records =
            tableColumnFamiliesToRecords.computeIfAbsent(key, k -> new ArrayList<>());
        records.add(record);
      }
    }
    return tableColumnFamiliesToRecords;
  }

  /**
   * Refreshes the existing table names in the cache.
   *
   * <p>Note that it deletes the entries from the cache if the tables disappear.
   */
  @VisibleForTesting
  void refreshTableNamesCache() {
    Set<TableId> tables;
    try {
      tables = bigtable.listTables().stream().map(TableId::of).collect(Collectors.toSet());
    } catch (ApiException e) {
      logger.error(
          "listTables() exception. If a table got deleted in the meantime, the sink might attempt"
              + " to write some records to a nonexistent table. If a table got created in the"
              + " meantime, records targeting it might be failed prematurely.",
          e);
      // We don't know exactly which tables exist, but we expect the set not to shrink.
      // So we carry on, hoping that our cache is up-to-date.
      // The alternative is to throw an exception and fail the whole batch that way.
      return;
    }
    for (TableId key : new HashSet<>(tableNameToColumnFamilies.keySet())) {
      if (!tables.contains(key)) {
        tableNameToColumnFamilies.remove(key);
      }
    }
    for (TableId table : tables) {
      tableNameToColumnFamilies.putIfAbsent(table, Optional.empty());
    }
  }

  /**
   * Refreshes existing table names and a subset of existing column families in the cache.
   *
   * <p>Note that it deletes the entries from the cache if the tables disappeared and that it
   * doesn't modify column family caches of tables that aren't provided as an argument.
   *
   * @param tablesRequiringRefresh A {@link Set} of table names whose column family caches will
   *     be refreshed.
   */
  @VisibleForTesting
  void refreshTableColumnFamiliesCache(Set<TableId> tablesRequiringRefresh) {
    refreshTableNamesCache();
    List<Map.Entry<TableId, ApiFuture<Table>>> tableFutures =
        tableNameToColumnFamilies.keySet().stream()
            .filter(tablesRequiringRefresh::contains)
            .map(t -> new AbstractMap.SimpleImmutableEntry<>(t, bigtable.getTableAsync(t)))
            .collect(Collectors.toList());
    Map<TableId, Optional<Set<String>>> newCache = new HashMap<>(tableNameToColumnFamilies);
    for (Map.Entry<TableId, ApiFuture<Table>> entry : tableFutures) {
      TableId tableId = entry.getKey();
      try {
        Table tableDetails = entry.getValue().get();
        Set<String> tableColumnFamilies =
            tableDetails.getColumnFamilies().stream()
                .map(ColumnFamily::getId)
                .collect(Collectors.toSet());
        newCache.put(tableId, Optional.of(tableColumnFamilies));
      } catch (ExecutionException | InterruptedException e) {
        if (SchemaApiExceptions.maybeExtractBigtableStatusCode(e)
            .map(sc -> StatusCode.Code.NOT_FOUND.equals(sc.getCode()))
            .orElse(false)) {
          newCache.remove(tableId);
        } else {
          // We don't know exactly which column families exist, but we expect the set not to shrink.
          // So we carry on, hoping that our cache is up-to-date.
          // The alternative is to throw an exception and fail the whole batch that way.
          logger.error(
              "getTable({}) exception. If a column family got deleted in the meantime, the sink"
                  + " might attempt to write some records to a nonexistent column family. If"
                  + " a column family got created in the meantime, records targeting it might be"
                  + " failed prematurely.",
              tableId,
              e);
        }
      }
    }
    // Note that we update the cache atomically to avoid partial errors. If an unexpected exception
    // is thrown, the whole batch is failed. It's not ideal, but in line with the behavior of other
    // connectors.
    tableNameToColumnFamilies = newCache;
  }

  /**
   * @param tableNamesToRecords A {@link Map} containing Cloud Bigtable table names and
   *     {@link SinkRecord SinkRecords} that need these tables to exist.
   * @return A subset of the input argument with the entries corresponding to existing tables
   *     removed.
   */
  private Map<TableId, List<SinkRecord>> missingTablesToRecords(
      Map<TableId, List<SinkRecord>> tableNamesToRecords) {
    Map<TableId, List<SinkRecord>> recordsByMissingTableNames = new HashMap<>(tableNamesToRecords);
    recordsByMissingTableNames.keySet().removeAll(tableNameToColumnFamilies.keySet());
    return recordsByMissingTableNames;
  }

  /**
   * @param tableColumnFamiliesToRecords A {@link Map} containing {@link Map.Entry} consisting
   *     of Bigtable table names and column families and {@link SinkRecord SinkRecords} that need to
   *     use these tables and column families to exist.
   * @return A subset of the input argument with the entries corresponding to existing column
   *     families removed.
   */
  private Map<Map.Entry<TableId, String>, List<SinkRecord>> missingTableColumnFamiliesToRecords(
      Map<Map.Entry<TableId, String>, List<SinkRecord>> tableColumnFamiliesToRecords) {
    Map<Map.Entry<TableId, String>, List<SinkRecord>> recordsByMissingColumnFamilies =
        new HashMap<>(tableColumnFamiliesToRecords);
    for (Map.Entry<TableId, Optional<Set<String>>> existingEntry :
        tableNameToColumnFamilies.entrySet()) {
      TableId tableName = existingEntry.getKey();
      for (String columnFamily : existingEntry.getValue().orElse(new HashSet<>())) {
        recordsByMissingColumnFamilies.remove(
            new AbstractMap.SimpleImmutableEntry<>(tableName, columnFamily));
      }
    }
    return recordsByMissingColumnFamilies;
  }

  private Map<ApiFuture<Table>, ResourceAndRecords<TableId>> sendCreateTableRequests(
      Map<TableId, List<SinkRecord>> recordsByMissingTables) {
    Map<ApiFuture<Table>, ResourceAndRecords<TableId>> result = new HashMap<>();
    for (Map.Entry<TableId, List<SinkRecord>> e : recordsByMissingTables.entrySet()) {
      ResourceAndRecords<TableId> resourceAndRecords =
          new ResourceAndRecords<>(e.getKey(), e.getValue());
      result.put(createTable(e.getKey()), resourceAndRecords);
    }
    return result;
  }

  private Map<ApiFuture<Table>, ResourceAndRecords<Map.Entry<TableId, String>>>
  sendCreateColumnFamilyRequests(
      Map<Map.Entry<TableId, String>, List<SinkRecord>> recordsByMissingColumnFamilies) {
    Map<ApiFuture<Table>, ResourceAndRecords<Map.Entry<TableId, String>>> result = new HashMap<>();
    for (Map.Entry<Map.Entry<TableId, String>, List<SinkRecord>> e :
        recordsByMissingColumnFamilies.entrySet()) {
      ResourceAndRecords<Map.Entry<TableId, String>> resourceAndRecords =
          new ResourceAndRecords<>(e.getKey(), e.getValue());
      result.put(createColumnFamily(e.getKey()), resourceAndRecords);
    }
    return result;
  }

  private ApiFuture<Table> createTable(TableId tableId) {
    logger.info("Creating table `{}`", tableId);
    CreateTableRequest createTableRequest = CreateTableRequest.of(Utils.getTableIdString(tableId));
    return bigtable.createTableAsync(createTableRequest);
  }

  // We only issue one request at a time because each multi-column-family operation on a single
  // Table is atomic and fails if any of the Column Families to be created already exists.
  // Thus by sending multiple requests, we simplify error handling when races between multiple
  // tasks of a single connector happen.
  private ApiFuture<Table> createColumnFamily(Map.Entry<TableId, String> tableNameAndColumnFamily) {
    TableId tableName = tableNameAndColumnFamily.getKey();
    String columnFamily = tableNameAndColumnFamily.getValue();
    logger.info("Creating column family `{}` in table `{}`", columnFamily, tableName);
    ModifyColumnFamiliesRequest request =
        ModifyColumnFamiliesRequest.of(Utils.getTableIdString(tableName)).addFamily(columnFamily);
    return bigtable.modifyFamiliesAsync(request);
  }

  /**
   * Awaits resource auto-creation result futures and handles the errors.
   *
   * <p>The errors might be handled in two ways:
   *
   * <ul>
   *   <li>If a resource's creation failed with an exception signifying that the request was
   *       invalid, it is assumed that input {@link SinkRecord SinkRecord(s)} map to invalid values,
   *       so all the {@link SinkRecord SinkRecord(s)} needing the resource whose creation failed
   *       are returned.
   *   <li>Other resource creation errors are only logged. A different section of code is
   *       responsible for checking whether the resources exist despite these futures' errors. This
   *       way all errors not caused by invalid input can be handled generally.
   * </ul>
   *
   * @param resourceCreationFuturesAndRecords {@link Map} of {@link ApiFuture ApiFuture(s)} and
   *     information what resource is created and for which {@link SinkRecord SinkRecord(s)}.
   * @param errorMessageTemplate The Java format string template of error message with which
   *     Cloud Bigtable exceptions for valid input data are logged.
   * @param <Fut> {@link ApiFuture} containing result of the resource creation operation.
   * @param <Id> The resources' type identifier.
   * @return A {@link Set} of {@link SinkRecord SinkRecord(s)} for which auto resource creation
   *     failed due to their invalid data.
   */
  @VisibleForTesting
  <Fut extends ApiFuture<?>, Id> Set<SinkRecord> awaitResourceCreationAndHandleInvalidInputErrors(
      Map<Fut, ResourceAndRecords<Id>> resourceCreationFuturesAndRecords,
      String errorMessageTemplate) {
    Set<SinkRecord> dataErrors = new HashSet<>();
    resourceCreationFuturesAndRecords.forEach(
        (fut, resourceAndRecords) -> {
          Object resource = resourceAndRecords.getResource();
          List<SinkRecord> sinkRecords = resourceAndRecords.getRecords();
          try {
            fut.get();
            logger.trace("Resource {} created successfully.", resource);
          } catch (ExecutionException | InterruptedException e) {
            logger.trace("Resource {} NOT created successfully.", resource);
            String errorMessage = String.format(errorMessageTemplate, resource.toString());
            if (SchemaApiExceptions.isCausedByInputError(e)) {
              dataErrors.addAll(sinkRecords);
            } else {
              logger.info(errorMessage, e);
            }
          }
        });
    return dataErrors;
  }

  @Override
  public void close() throws Exception {
    bigtable.close();
  }

  /**
   * A record class connecting an auto-created resource and {@link SinkRecord SinkRecord(s)}
   * requiring it to exist.
   *
   * @param <Id> The resources' type identifier.
   */
  @VisibleForTesting
  static class ResourceAndRecords<Id> {

    private final Id resource;
    private final List<SinkRecord> records;

    public ResourceAndRecords(Id resource, List<SinkRecord> records) {
      this.resource = resource;
      this.records = records;
    }

    public Id getResource() {
      return resource;
    }

    public List<SinkRecord> getRecords() {
      return records;
    }

    @Override
    public String toString() {
      return String.format("Resource(id=%s,#records=%d)", resource, records.size());
    }
  }

  /**
   * A helper class containing logic for grouping {@link ApiException ApiException(s)} encountered
   * when modifying Cloud Bigtable schema.
   */
  @VisibleForTesting
  static class SchemaApiExceptions {

    /**
     * @param t Exception thrown by some function using Cloud Bigtable API.
     * @return true if input exception was caused by invalid Cloud Bigtable request, false
     *     otherwise.
     */
    @VisibleForTesting
    static boolean isCausedByInputError(Throwable t) {
      return maybeExtractBigtableStatusCode(t)
          .map(sc -> isStatusCodeCausedByInputError(sc.getCode()))
          .orElse(false);
    }

    @VisibleForTesting
    static Optional<StatusCode> maybeExtractBigtableStatusCode(Throwable t) {
      while (t != null) {
        if (t instanceof ApiException) {
          ApiException apiException = (ApiException) t;
          return Optional.of(apiException.getStatusCode());
        }
        t = t.getCause();
      }
      return Optional.empty();
    }

    @VisibleForTesting
    static boolean isStatusCodeCausedByInputError(StatusCode.Code code) {
      switch (code) {
        case INVALID_ARGUMENT:
        case OUT_OF_RANGE:
          return true;
        default:
          return false;
      }
    }
  }
}
