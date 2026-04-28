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

import com.google.cloud.aerospike.BackupReader;
import com.google.cloud.aerospike.ReadRecordResult;
import com.google.cloud.aerospike.RowBuilder;
import com.google.cloud.bigtable.admin.v2.BigtableTableAdminClient;
import com.google.cloud.bigtable.admin.v2.models.ColumnFamily;
import com.google.cloud.bigtable.admin.v2.models.Table;
import com.google.cloud.teleport.metadata.Template;
import com.google.cloud.teleport.metadata.TemplateCategory;
import com.google.cloud.teleport.v2.bigtable.options.BigtableCommonOptions;
import com.google.cloud.teleport.v2.transforms.AerospikeTransforms;
import com.google.cloud.teleport.v2.transforms.BigtableTransforms;
import com.squareup.moshi.JsonDataException;
import java.io.IOException;
import java.util.Set;
import java.util.stream.Collectors;
import org.apache.beam.sdk.Pipeline;
import org.apache.beam.sdk.PipelineResult;
import org.apache.beam.sdk.io.FileIO;
import org.apache.beam.sdk.io.TextIO;
import org.apache.beam.sdk.options.PipelineOptionsFactory;
import org.apache.beam.sdk.transforms.DoFn;
import org.apache.beam.sdk.transforms.ParDo;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * This pipeline reads Aerospike backup records from .asb file(s), converts Aerospike data
 * corresponding Bigtable data, and then writes that data into Cloud Bigtable. The records for which
 * this process failed, are written into a Cloud Storage file.
 */
@Template(
    name = "Aerospike_Backup_To_Bigtable",
    category = TemplateCategory.BATCH,
    displayName = "AerospikeBackupToBigtable",
    description =
        "This Dataflow batch pipeline reads Aerospike backup files and loads the data into Cloud"
            + " Bigtable. Failed writes to Bigtable are captured and written to a separate JSON"
            + " file for inspection or retry. ",
    flexContainerName = "aerospike-backup-to-bigtable",
    optionsClass = AerospikeBackupToBigtableOptions.class,
    optionsOrder = {
      AerospikeBackupToBigtableOptions.class,
      BigtableCommonOptions.class,
      BigtableCommonOptions.WriteOptions.class,
    })
public final class AerospikeBackupToBigtable {
  private static final Logger LOG = LoggerFactory.getLogger(AerospikeBackupToBigtable.class);

  /**
   * The main entry-point for pipeline execution.
   *
   * @param args command-line args passed by the executor.
   */
  public static void main(String[] args) {
    AerospikeBackupToBigtableOptions options =
        PipelineOptionsFactory.fromArgs(args)
            .withValidation()
            .as(AerospikeBackupToBigtableOptions.class);
    if (options.getValidateTableSchema()) {
      checkTableSchema(
          options.getProject(),
          options.getBigtableWriteInstanceId(),
          options.getBigtableWriteTableId(),
          Set.of(RowBuilder.SCALAR_FAMILY, RowBuilder.COMPLEX_OBJECT_FAMILY));
    } else {
      LOG.debug(
          "Skipping Table Schema Validation because option validateTableSchema set to false.");
    }
    run(options);
  }

  /**
   * Runs the pipeline with the supplied options.
   *
   * @param options The execution parameters to the pipeline.
   * @return The result of the pipeline execution.
   */
  public static PipelineResult run(AerospikeBackupToBigtableOptions options) {
    Pipeline pipeline = Pipeline.create(options);

    BigtableTarget bigtableTarget = new BigtableTarget(options);

    TextIO.Write writer = TextIO.write().to(options.getOutputPath()).withoutSharding();
    pipeline
        .apply("GetFileList", FileIO.match().filepattern(options.getInputFilePattern()))
        .apply(
            "ReadFile",
            ParDo.of(
                new AerospikeTransforms.ProcessBackupFn(
                    options.getSrcNamespace(),
                    parseCompressionAlgorithm(options.getBackupCompressionAlgorithm()))))
        .apply("SaveToBigtable", ParDo.of(new BigtableTransforms.BigtableWriteDoFn(bigtableTarget)))
        .apply(
            "SerializeErrors",
            ParDo.of(
                new DoFn<ReadRecordResult, String>() {
                  @ProcessElement
                  public void processElement(ProcessContext c) {
                    ReadRecordResult original = c.element();
                    try {
                      c.output(original.toJson());
                    } catch (IOException | JsonDataException e) {
                      LOG.error("Encountered error when converting a record to JSON object", e);
                    }
                  }
                }))
        .apply("WriteFile", writer);

    return pipeline.run();
  }

  public static BackupReader.CompressionAlgorithm parseCompressionAlgorithm(
      String compressionAlgorithm) {
    if (compressionAlgorithm == null || compressionAlgorithm.isEmpty()) {
      return BackupReader.CompressionAlgorithm.NONE;
    } else if (compressionAlgorithm.equalsIgnoreCase("zstd")) {
      return BackupReader.CompressionAlgorithm.ZSTD;
    } else {
      throw new IllegalArgumentException("Unrecognized compression algorithm");
    }
  }

  public static void checkTableSchema(
      String projectId, String instanceId, String tableId, Set<String> requiredColumnFamilies) {
    try (BigtableTableAdminClient adminClient =
        BigtableTableAdminClient.create(projectId, instanceId)) {

      if (!adminClient.exists(tableId)) {
        LOG.error("Table '" + tableId + "' does not exist.");
        throw new RuntimeException("Table '" + tableId + "' does not exist.");
      }

      Table table = adminClient.getTable(tableId);
      Set<String> existingFamilies =
          table.getColumnFamilies().stream().map(ColumnFamily::getId).collect(Collectors.toSet());

      for (String cf : requiredColumnFamilies) {
        if (!existingFamilies.contains(cf)) {
          LOG.error("Column Family '" + cf + "' is MISSING.");
          throw new RuntimeException("Column Family '" + cf + "' is MISSING.");
        }
      }
    } catch (Exception e) {
      LOG.error("Error checking Bigtable schema", e);
      throw new RuntimeException("Error checking Bigtable schema", e);
    }
  }
}
