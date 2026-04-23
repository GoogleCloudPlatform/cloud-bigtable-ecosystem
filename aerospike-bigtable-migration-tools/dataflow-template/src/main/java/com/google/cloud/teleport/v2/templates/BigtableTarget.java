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
package com.google.cloud.teleport.v2.templates;

import com.google.api.gax.retrying.RetrySettings;
import com.google.api.gax.rpc.StatusCode;
import com.google.cloud.bigtable.data.v2.BigtableDataClient;
import com.google.cloud.bigtable.data.v2.BigtableDataSettings;
import com.google.cloud.bigtable.data.v2.stub.BigtableBatchingCallSettings;
import com.google.common.collect.Sets;
import java.io.IOException;
import java.io.Serializable;
import java.time.Duration;
import java.util.Arrays;
import java.util.Optional;
import java.util.Set;
import java.util.stream.Collectors;
import org.apache.commons.lang3.StringUtils;

public class BigtableTarget implements Serializable {
  private final String projectId;
  public final String instanceId;
  public final String tableId;
  private final String appProfileId;

  private final Integer rpcAttemptTimeoutMs;
  private final Integer rpcTotalTimeoutMs;
  private final Set<StatusCode.Code> additionalRetryCodes;

  private final Integer latencyTargetMs;
  public final Integer bulkWriteMaxRowKeyCount;
  public final boolean verboseErrorLogging;

  public BigtableTarget(AerospikeBackupToBigtableOptions options) {
    projectId = options.getProject();
    instanceId = options.getBigtableWriteInstanceId();
    tableId = options.getBigtableWriteTableId();
    appProfileId = options.getBigtableWriteAppProfile();

    rpcAttemptTimeoutMs = options.getBigtableRpcAttemptTimeoutMs();
    rpcTotalTimeoutMs = options.getBigtableRpcTimeoutMs();
    additionalRetryCodes =
        Arrays.stream(
                Optional.ofNullable(options.getBigtableAdditionalRetryCodes())
                    .orElse("")
                    .split(","))
            .map(String::trim)
            .filter(s -> !s.isEmpty())
            .map(
                s -> {
                  try {
                    return StatusCode.Code.valueOf(s);
                  } catch (IllegalArgumentException e) {
                    throw new IllegalArgumentException("Invalid retry code: " + s, e);
                  }
                })
            .collect(Collectors.toSet());

    latencyTargetMs = options.getBigtableBulkWriteLatencyTargetMs();
    bulkWriteMaxRowKeyCount = options.getBigtableBulkWriteMaxRowKeyCount();
    verboseErrorLogging = options.getBigtableVerboseErrorLogging();
  }

  public BigtableDataClient createDataClient() throws IOException {
    RetrySettings.Builder retrySettingsBuilder = RetrySettings.newBuilder();
    if (rpcAttemptTimeoutMs != null) {
      retrySettingsBuilder.setMaxRpcTimeoutDuration(Duration.ofMillis(rpcAttemptTimeoutMs));
    }
    if (rpcTotalTimeoutMs != null) {
      retrySettingsBuilder.setTotalTimeoutDuration(Duration.ofMillis(rpcTotalTimeoutMs));
    }

    BigtableDataSettings.Builder settingsBuilder =
        BigtableDataSettings.newBuilder().setProjectId(projectId).setInstanceId(instanceId);
    if (!StringUtils.isEmpty(appProfileId)) {
      settingsBuilder.setAppProfileId(appProfileId);
    }
    if (latencyTargetMs != null) {
      settingsBuilder.enableBatchMutationLatencyBasedThrottling(latencyTargetMs);
    }

    BigtableBatchingCallSettings.Builder bulkMutateRowsSettings =
        settingsBuilder.stubSettings().bulkMutateRowsSettings();
    bulkMutateRowsSettings.setRetrySettings(retrySettingsBuilder.build());
    if (!additionalRetryCodes.isEmpty()) {
      bulkMutateRowsSettings.setRetryableCodes(
          Sets.union(additionalRetryCodes, bulkMutateRowsSettings.getRetryableCodes()));
    }

    return BigtableDataClient.create(settingsBuilder.build());
  }
}
