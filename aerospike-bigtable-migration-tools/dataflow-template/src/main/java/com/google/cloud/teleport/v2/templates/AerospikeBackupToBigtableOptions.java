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

import com.google.cloud.teleport.metadata.TemplateParameter;
import com.google.cloud.teleport.v2.bigtable.options.BigtableCommonOptions;
import org.apache.beam.sdk.options.Default;
import org.apache.beam.sdk.options.PipelineOptions;
import org.apache.beam.sdk.options.Validation;

public interface AerospikeBackupToBigtableOptions extends PipelineOptions, BigtableCommonOptions {
  // Note that we are not extending BigtableCommonOptions.WriteOptions because
  // we don't want to include a parameter bigtableWriteColumnFamily which is required in
  // WriteOptions.
  @TemplateParameter.GcsReadFolder(
      order = 1,
      description = "Input files in Cloud Storage",
      helpText =
          "The Cloud Storage file pattern to search for backup files. Aerospike backups may be"
              + " split across multiple files; users can safely retry entire files containing"
              + " failed writes, as Bigtable writes are idempotent and can be repeated without side"
              + " effects.",
      example = "gs://mybucket/backup-*.asb")
  @Validation.Required
  String getInputFilePattern();

  void setInputFilePattern(String value);

  @TemplateParameter.GcsWriteFile(
      order = 2,
      description = "Error Output Cloud Storage file",
      helpText = "Path for writing unprocessed records. All records will be put in this one file.",
      example = "gs://your-bucket/unprocessed_records")
  @Validation.Required
  String getOutputPath();

  void setOutputPath(String value);

  @TemplateParameter.Text(
      order = 3,
      description = "Aerospike Source Namespace",
      helpText =
          "Aerospike Namespace "
              + "which will be read from the backup. Records from other"
              + "namespaces will be omitted.")
  @Validation.Required
  String getSrcNamespace();

  void setSrcNamespace(String value);

  @TemplateParameter.Boolean(
      order = 4,
      description = "Validate Table Schema Before Uploading",
      helpText =
          "By default, the pipeline ensures that all required column families exist in the target"
              + " Cloud Bigtable before starting the upload. This behavior can be disabled by"
              + " setting this option to false.",
      optional = true,
      hiddenUi = true)
  @Default.Boolean(true)
  Boolean getValidateTableSchema();

  void setValidateTableSchema(Boolean value);

  @TemplateParameter.Boolean(
      order = 5,
      description = "Enable Bigtable Verbose Logging",
      helpText =
          "When enabled (set to true), all individual write errors to Bigtable are logged. By"
              + " default, only one error per batch is reported. Use with caution, as this may"
              + " significantly increase log volume.",
      optional = true,
      hiddenUi = true)
  @Default.Boolean(false)
  Boolean getBigtableVerboseErrorLogging();

  void setBigtableVerboseErrorLogging(Boolean value);

  @TemplateParameter.Text(
      order = 6,
      regexes = {"[a-z][a-z0-9\\-]+[a-z0-9]"},
      description = "Bigtable Instance ID",
      helpText = "The ID of the Bigtable instance that contains the table.")
  @Validation.Required
  String getBigtableWriteInstanceId();

  void setBigtableWriteInstanceId(String value);

  @TemplateParameter.Text(
      order = 7,
      regexes = {"[_a-zA-Z0-9][-_.a-zA-Z0-9]*"},
      description = "Bigtable Table ID",
      helpText = "The ID of the Bigtable table to write to.")
  @Validation.Required
  String getBigtableWriteTableId();

  void setBigtableWriteTableId(String value);

  @TemplateParameter.Text(
      order = 8,
      optional = true,
      regexes = {"[a-z][a-z0-9\\-]+[a-z0-9]"},
      description = "Bigtable App Profile",
      helpText =
          "The ID of the Bigtable application profile to use for the export. If you do not specify"
              + " an app profile, Bigtable uses the default app profile"
              + " (https://cloud.google.com/bigtable/docs/app-profiles#default-app-profile) of the"
              + " instance.")
  @Default.String("default")
  String getBigtableWriteAppProfile();

  void setBigtableWriteAppProfile(String value);

  @TemplateParameter.Integer(
      order = 9,
      optional = true,
      description = "Bigtable's latency target in milliseconds for latency-based throttling",
      helpText = "The latency target of Bigtable in milliseconds for latency-based throttling.")
  Integer getBigtableBulkWriteLatencyTargetMs();

  void setBigtableBulkWriteLatencyTargetMs(Integer value);

  @TemplateParameter.Integer(
      order = 10,
      optional = true,
      description = "The max number of row keys in a Bigtable batch write operation",
      helpText = "The maximum number of row keys in a Bigtable batch write operation.")
  Integer getBigtableBulkWriteMaxRowKeyCount();

  void setBigtableBulkWriteMaxRowKeyCount(Integer value);

  @TemplateParameter.Text(
      order = 11,
      description = "Compression Algorithm",
      helpText =
          "Compression algorithm used for making the backup. "
              + "If unspecified, backup is assumed uncompressed.",
      optional = true)
  @Default.String("")
  String getBackupCompressionAlgorithm();

  void setBackupCompressionAlgorithm(String value);
}
