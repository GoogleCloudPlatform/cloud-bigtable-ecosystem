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
package com.google.cloud.teleport.v2.transforms;

import com.google.api.core.ApiFuture;
import com.google.api.gax.batching.Batcher;
import com.google.cloud.aerospike.BigtableMutationBuilder;
import com.google.cloud.aerospike.ReadRecordResult;
import com.google.cloud.aerospike.values.Value;
import com.google.cloud.bigtable.data.v2.BigtableDataClient;
import com.google.cloud.bigtable.data.v2.models.RowMutationEntry;
import com.google.cloud.bigtable.data.v2.models.TableId;
import com.google.cloud.teleport.v2.templates.BigtableTarget;
import com.google.protobuf.ByteString;
import com.squareup.moshi.JsonDataException;
import java.io.IOException;
import java.time.Duration;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.ExecutionException;
import java.util.function.Consumer;
import org.apache.beam.sdk.transforms.DoFn;
import org.apache.beam.sdk.transforms.windowing.GlobalWindow;
import org.joda.time.Instant;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class BigtableTransforms {

  private static final Logger LOG = LoggerFactory.getLogger(BigtableTransforms.class);

  /**
   * Writes Aerospike records to Cloud Bigtable in batches. Returns records for which the write
   * failed.
   */
  public static class BigtableWriteDoFn extends DoFn<ReadRecordResult, ReadRecordResult> {
    private static final Duration BATCHER_CLOSE_TIMEOUT = Duration.ofSeconds(1);
    // We don't know the Aerospike records' timestamps, so we put them into Cloud Bigtable with the
    // smallest possible one.
    private static final Long BIGTABLE_TIMESTAMP = 0L;

    private final BigtableTarget bigtableTarget;

    private transient BigtableDataClient client;
    private transient Batcher<RowMutationEntry, Void> batcher;
    private transient Map<ReadRecordResult, ApiFuture<Void>> currentBatch;

    public BigtableWriteDoFn(BigtableTarget bigtableTarget) {
      this.bigtableTarget = bigtableTarget;
    }

    @Setup
    public void setup() throws IOException {
      client = bigtableTarget.createDataClient();
      // Note that we can use a single batcher for the whole `DoFn` because `ParDo` guarantees that
      // "A DoFn that has terminated abnormally (by throwing an Exception) will never be reused".
      batcher = client.newBulkMutationBatcher(TableId.of(bigtableTarget.tableId));
      currentBatch = new HashMap<>();
      LOG.debug("Succesfully created bigtable client connected to: " + bigtableTarget.instanceId);
    }

    @Teardown
    public void teardown() {
      if (batcher != null) {
        try {
          batcher.close(BATCHER_CLOSE_TIMEOUT);
        } catch (Exception e) {
          LOG.error("Closing Bigtable batcher failed", e);
        }
      }
      if (client != null) {
        try {
          client.close();
        } catch (Exception e) {
          LOG.error("Closing Bigtable client failed", e);
        }
      }
    }

    @FinishBundle
    public void finishBundle(FinishBundleContext c) throws InterruptedException {
      flushBatcher(failedMessage -> c.output(failedMessage, Instant.now(), GlobalWindow.INSTANCE));
    }

    public void flushBatcher(Consumer<ReadRecordResult> invalidRecordHandler)
        throws InterruptedException {
      Exception exception = null;
      int exceptionCounter = 0;
      batcher.flush();
      for (Map.Entry<ReadRecordResult, ApiFuture<Void>> recordResult : currentBatch.entrySet()) {
        ReadRecordResult record = recordResult.getKey();
        try {
          recordResult.getValue().get();
        } catch (ExecutionException e) {
          String recordAsJson = "<Failed to parse record to JSON>";
          try {
            recordAsJson = record.toJson();
          } catch (IOException | JsonDataException ex) {
            LOG.error("Failed to serialize record to JSON", ex);
          }

          if (bigtableTarget.verboseErrorLogging) {
            LOG.error("Exception occured when writing record " + recordAsJson, e);
          } else {
            exception = e;
            exceptionCounter += 1;
          }

          invalidRecordHandler.accept(record);
        }
      }
      currentBatch = new HashMap<>();
      if (exceptionCounter > 0) {
        LOG.error(
            "During writing a batch of writes "
                + exceptionCounter
                + " exceptions occured. One of them was: ",
            exception);
      }
    }

    @ProcessElement
    public void processElement(ProcessContext c) throws InterruptedException {
      ReadRecordResult record = c.element();

      ByteString rowKey = ByteString.copyFromUtf8(record.base64Digest());
      BigtableMutationBuilder bigtableMutationBuilder =
          new BigtableMutationBuilder(bigtableTarget.tableId, rowKey, BIGTABLE_TIMESTAMP);
      for (Map.Entry<String, Value> entry : record.bins.entrySet()) {
        entry.getValue().setCell(entry.getKey(), bigtableMutationBuilder);
      }
      RowMutationEntry mutation =
          RowMutationEntry.createFromMutationUnsafe(rowKey, bigtableMutationBuilder.getMutation());
      ApiFuture<Void> result = batcher.add(mutation);
      currentBatch.put(record, result);
      if (bigtableTarget.bulkWriteMaxRowKeyCount != null
          && currentBatch.size() >= bigtableTarget.bulkWriteMaxRowKeyCount) {
        flushBatcher(c::output);
      }
    }
  }
}
