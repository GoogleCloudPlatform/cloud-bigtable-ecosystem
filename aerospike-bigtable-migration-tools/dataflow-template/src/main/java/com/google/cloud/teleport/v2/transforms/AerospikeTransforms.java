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

import com.google.cloud.aerospike.BackupReader;
import com.google.cloud.aerospike.ReadRecordResult;
import com.google.cloud.aerospike.exceptions.JNIException;
import com.google.cloud.aerospike.exceptions.MalformedBackupDataException;
import com.google.cloud.aerospike.exceptions.UnsupportedBackupEntryException;
import com.google.cloud.storage.BlobId;
import com.google.cloud.storage.Storage;
import com.google.cloud.storage.StorageOptions;
import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.nio.channels.Channels;
import java.util.HashMap;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;
import org.apache.beam.sdk.io.fs.MatchResult;
import org.apache.beam.sdk.transforms.DoFn;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class AerospikeTransforms {
  private static final Logger LOG = LoggerFactory.getLogger(AerospikeTransforms.class);

  public static class ProcessBackupFn extends DoFn<MatchResult.Metadata, ReadRecordResult> {
    public final String namespace;
    public final BackupReader.CompressionAlgorithm compressionAlgorithm;

    public ProcessBackupFn(
        String namespace, BackupReader.CompressionAlgorithm compressionAlgorithm) {
      this.namespace = namespace;
      this.compressionAlgorithm = compressionAlgorithm;
    }

    /**
     * Reads a backup file and outputs the records from the backup to the next pipeline step.
     *
     * <p>If processing fails, the whole file is retried. This is safe because: reading the file is
     * deterministic and duplicate records are acceptable since writes to Bigtable are idempotent.
     */
    @ProcessElement
    public void processElement(
        @Element MatchResult.Metadata fileMetadata, OutputReceiver<ReadRecordResult> receiver) {
      String tmpDirPath = System.getProperty("java.io.tmpdir", "/tmp");
      String pipePath = new File(tmpDirPath, "fifoPipe-" + UUID.randomUUID()).getAbsolutePath();
      String gcsFilePath = fileMetadata.resourceId().toString();
      Map<String, Integer> unsupportedEntryMessages = new HashMap<>();
      try (BucketStreamer bucketStreamer = new BucketStreamer(gcsFilePath, pipePath);
          BackupReader backupReader =
              new BackupReader(bucketStreamer.pipe.getAbsolutePath(), this.compressionAlgorithm)) {
        LOG.debug("Reading backup file: {}.", gcsFilePath);
        while (true) {
          try {
            ReadRecordResult result = backupReader.readRecord(this.namespace);
            if (result == null) {
              // EOF reached, end the loop.
              break;
            } else {
              receiver.output(result);
            }
          } catch (UnsupportedBackupEntryException e) {
            unsupportedEntryMessages.merge(e.getMessage(), 1, Integer::sum);
          } catch (MalformedBackupDataException e) {
            LOG.error(
                "Malformed data encountered while reading the Aerospike backup, skipping it.", e);
          }
        }
      } catch (IOException | InterruptedException e) {
        LOG.error(
            "Error occurred while reading from file {} via a pipe {}.", gcsFilePath, pipePath, e);
        throw new RuntimeException(e);
      } catch (JNIException | MalformedBackupDataException e) {
        LOG.error(
            "Pipe {} (streaming data from {}) cannot be successfully read by BackupReader.",
            pipePath,
            gcsFilePath,
            e);
        throw new RuntimeException(e);
      } finally {
        if (!unsupportedEntryMessages.isEmpty()) {
          StringBuilder report =
              new StringBuilder("Unsupported entries encountered while reading the backup:\n");
          unsupportedEntryMessages.forEach(
              (message, count) -> report.append(String.format("  - %s: %d%n", message, count)));
          LOG.error(report.toString());
        }
      }
    }
  }

  public static class BucketStreamer implements AutoCloseable {
    public final File pipe;
    public final String gcsPath;
    private final CompletableFuture<Void> future;

    public BucketStreamer(String gcsPath, String pipePath)
        throws InterruptedException, IOException {
      this.gcsPath = gcsPath;
      this.pipe = new File(pipePath);

      String[] createPipeCommand = {"mkfifo", pipePath};
      ProcessBuilder createPipePb = new ProcessBuilder(createPipeCommand);
      Process createPipeProcess = createPipePb.start();
      if (createPipeProcess.waitFor() != 0) {
        throw new RuntimeException(
            String.format(
                "`mkfifo \"%s\"` failed with exit code: %s.",
                pipePath, createPipeProcess.exitValue()));
      }

      this.future =
          CompletableFuture.runAsync(
              () -> {
                Storage storage = StorageOptions.getDefaultInstance().getService();
                BlobId blobId = BlobId.fromGsUtilUri(gcsPath);
                LOG.info("Streaming GCS data from {}", blobId);
                try (InputStream gcs = Channels.newInputStream(storage.reader(blobId));
                    FileOutputStream pipe = new FileOutputStream(this.pipe)) {
                  gcs.transferTo(pipe);
                } catch (IOException e) {
                  // Note that this IOException could theoretically be a broken pipe error caused
                  // by our BackupReader deciding to close the reading end of the pipe early.
                  // Fortunately, we're reading the backup file right to the end, so in this case
                  // an IOException means an error.
                  throw new RuntimeException(e);
                }
              });
    }

    @Override
    public void close() {
      LOG.info(
          "Deleting pipe {} used to stream GCS data from {}.", pipe.getAbsolutePath(), gcsPath);
      if (!pipe.delete()) {
        LOG.warn(
            "Failed to delete pipe {} used to stream GCS data from {}.",
            pipe.getAbsolutePath(),
            gcsPath);
      }
      if (future.isDone()) {
        try {
          future.get();
          LOG.info("Streaming GCS data from {} succeeded.", gcsPath);
        } catch (Exception e) {
          throw new RuntimeException(
              String.format("Failed to stream GCS data from \"%s\"", gcsPath), e);
        }
      } else {
        LOG.info("Cancelling a future streaming GCS data from {}.", gcsPath);
        future.cancel(true);
      }
    }
  }
}
