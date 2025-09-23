package com.google.cloud.kafka.connect.bigtable.autocreate;

import com.google.cloud.kafka.connect.bigtable.RecordErrorHandler;
import com.google.cloud.kafka.connect.bigtable.exception.InvalidBigtableSchemaModificationException;
import com.google.cloud.kafka.connect.bigtable.mapping.MutationData;
import com.google.cloud.kafka.connect.bigtable.wrappers.BigtableTableAdminClientInterface;
import com.google.common.annotations.VisibleForTesting;
import java.util.Collection;
import java.util.LinkedList;
import org.apache.kafka.connect.errors.ConnectException;

public class AutoResourceCreatorImpl implements AutoResourceCreator {

  private final BigtableSchemaManager schemaManager;
  private final RecordErrorHandler errorHandler;
  private final BigtableTableAdminClientInterface adminClient;
  private final boolean autoCreateTables;
  private final boolean autoCreateColumnFamilies;

  public AutoResourceCreatorImpl(BigtableSchemaManager schemaManager,
      RecordErrorHandler errorHandler,
      BigtableTableAdminClientInterface adminClient, boolean autoCreateTables,
      boolean autoCreateColumnFamilies) {
    this.schemaManager = schemaManager;
    this.errorHandler = errorHandler;
    this.adminClient = adminClient;
    this.autoCreateTables = autoCreateTables;
    this.autoCreateColumnFamilies = autoCreateColumnFamilies;
  }

  public Collection<MutationData> CreateResources(Collection<MutationData> mutations) {
    if (autoCreateTables) {
      mutations = autoCreateTablesAndHandleErrors(mutations);
    }
    if (autoCreateColumnFamilies) {
      mutations = autoCreateColumnFamiliesAndHandleErrors(mutations);
    }
    return mutations;
  }

  /**
   * Attempts to create Cloud Bigtable tables so that all the mutations can be applied and handles
   * errors.
   *
   * @param mutations Input records and corresponding mutations.
   * @return Subset of the input argument containing only those record for which the target Cloud
   *     Bigtable tables exist.
   */
  @VisibleForTesting
  Collection<MutationData> autoCreateTablesAndHandleErrors(
      Collection<MutationData> mutations) {
    ResourceCreationResult resourceCreationResult = schemaManager.ensureTablesExist(mutations);
    String errorMessage = "Table auto-creation failed.";
    return HandleErrors(mutations, resourceCreationResult, errorMessage);
  }

  /**
   * Attempts to create Cloud Bigtable column families so that all the mutations can be applied and
   * handles errors.
   *
   * @param mutations Input records and corresponding mutations.
   * @return Subset of the input argument containing only those record for which the target Cloud
   *     Bigtable column families exist.
   */
  @VisibleForTesting
  Collection<MutationData> autoCreateColumnFamiliesAndHandleErrors(
      Collection<MutationData> mutations) {
    ResourceCreationResult resourceCreationResult =
        schemaManager.ensureColumnFamiliesExist(mutations);
    String errorMessage = "Column family auto-creation failed.";
    return HandleErrors(mutations, resourceCreationResult, errorMessage);
  }

  private Collection<MutationData> HandleErrors(Collection<MutationData> mutations,
      ResourceCreationResult resourceCreationResult, String errorMessage) {
    var results = new LinkedList<MutationData>();
    for (MutationData mut : mutations) {
      if (resourceCreationResult.getBigtableErrors().contains(mut.getRecord())) {
        errorHandler.reportError(mut.getRecord(), new ConnectException(errorMessage));
        continue;
      } else if (resourceCreationResult.getDataErrors().contains(mut.getRecord())) {
        errorHandler.reportError(mut.getRecord(),
            new InvalidBigtableSchemaModificationException(errorMessage));
        continue;
      }
      results.add(mut);
    }
    return results;
  }

  public void Close() {
    adminClient.close();
  }
}
