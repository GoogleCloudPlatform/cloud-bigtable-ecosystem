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

import static com.google.common.truth.Truth.assertThat;
import static org.apache.beam.it.gcp.bigtable.matchers.BigtableAsserts.assertThatBigtableRecords;
import static org.apache.beam.it.truthmatchers.PipelineAsserts.assertThatPipeline;
import static org.apache.commons.lang3.RandomStringUtils.randomAlphanumeric;
import static org.junit.Assert.assertThrows;

import com.google.cloud.aerospike.RowBuilder;
import com.google.cloud.bigtable.data.v2.models.Row;
import com.google.cloud.teleport.metadata.TemplateIntegrationTest;
import com.google.common.io.Resources;
import java.io.IOException;
import java.util.List;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.stream.Collectors;
import org.apache.beam.it.common.PipelineLauncher;
import org.apache.beam.it.common.PipelineLauncher.LaunchInfo;
import org.apache.beam.it.common.utils.ResourceManagerUtils;
import org.apache.beam.it.gcp.TemplateTestBase;
import org.apache.beam.it.gcp.artifacts.Artifact;
import org.apache.beam.it.gcp.bigtable.BigtableResourceManager;
import org.apache.beam.it.gcp.bigtable.BigtableTableSpec;
import org.junit.After;
import org.junit.Before;
import org.junit.Test;
import org.junit.experimental.categories.Category;

/** Integration test for {@link AerospikeBackupToBigtable}. */
@TemplateIntegrationTest(AerospikeBackupToBigtable.class)
@Category({TemplateIntegrationTest.class})
public class AerospikeBackupToBigtableIT extends TemplateTestBase {
  private static final String BACKUP_FILE_1 = "backup_00000.asb";
  private static final String BACKUP_FILE_2 = "backup_00001.asb";
  private static final String BACKUP_FILE_COMP = "backup_compressed.asb";
  private static final String COMPRESSED_BACKUPS_DIR = "compressed_backups";
  private static final String BACKUPS_DIR = "backups";
  private static final String NAMESPACE = "test";
  private static final String OUTPUT_FILE = "output";
  private static final String COMPRESSION_ALG = "zstd";
  private BigtableResourceManager bigtableResourceManager;

  private String appProfileId;

  @Before
  public void setupTest() throws IOException {
    gcsClient.uploadArtifact(
        BACKUPS_DIR + "/" + BACKUP_FILE_1, Resources.getResource(BACKUP_FILE_1).getPath());
    gcsClient.uploadArtifact(
        BACKUPS_DIR + "/" + BACKUP_FILE_2, Resources.getResource(BACKUP_FILE_2).getPath());
    gcsClient.uploadArtifact(
        COMPRESSED_BACKUPS_DIR + "/" + BACKUP_FILE_COMP,
        Resources.getResource(BACKUP_FILE_COMP).getPath());
    BigtableResourceManager.Builder rmBuilder =
        BigtableResourceManager.builder(
            removeUnsafeCharacters(testName), PROJECT, credentialsProvider);

    bigtableResourceManager = rmBuilder.maybeUseStaticInstance().build();

    appProfileId = "profile" + randomAlphanumeric(8).toLowerCase();
    bigtableResourceManager.createAppProfile(
        appProfileId, true, bigtableResourceManager.getClusterNames());
  }

  @After
  public void tearDownTest() {
    ResourceManagerUtils.cleanResources(bigtableResourceManager);
  }

  @Test
  public void testHappyPath() throws IOException {
    // Arrange
    String tableName = "tbl_testHappyPath" + randomAlphanumeric(8).toLowerCase();
    BigtableTableSpec tableSpec = new BigtableTableSpec();
    tableSpec.setColumnFamilies(
        List.of(RowBuilder.SCALAR_FAMILY, RowBuilder.COMPLEX_OBJECT_FAMILY));
    bigtableResourceManager.createTable(tableName, tableSpec);

    // Act
    LaunchInfo launchInfo =
        launchTemplate(
            Map.of(
                "bigtableWriteTableId",
                tableName,
                "srcNamespace",
                NAMESPACE,
                "inputFilePattern",
                getGcsPath(BACKUPS_DIR) + "/*"));
    assertThatPipeline(launchInfo).isRunning();
    pipelineOperator().waitUntilDone(createConfig(launchInfo));

    // Assert
    List<Row> rows = bigtableResourceManager.readTable(tableName);
    assertThatBigtableRecords(rows, RowBuilder.SCALAR_FAMILY).hasRows(12);

    // Make sure that error output file is empty
    List<Artifact> artifacts =
        gcsClient.listArtifacts(testName, Pattern.compile(".*" + OUTPUT_FILE + ".*"));
    assertThat(artifacts.isEmpty()).isFalse();
    String allMessages =
        artifacts.stream()
            .map(artifact -> new String(artifact.contents()))
            .collect(Collectors.joining());
    assertThat(allMessages.trim()).isEmpty();
  }

  @Test
  public void testCompressed() throws IOException {
    // Arrange
    String tableName = "tbl_testCompressed" + randomAlphanumeric(8).toLowerCase();
    BigtableTableSpec tableSpec = new BigtableTableSpec();
    tableSpec.setColumnFamilies(
        List.of(RowBuilder.SCALAR_FAMILY, RowBuilder.COMPLEX_OBJECT_FAMILY));
    bigtableResourceManager.createTable(tableName, tableSpec);

    // Act
    LaunchInfo launchInfo =
        launchTemplate(
            Map.of(
                "bigtableWriteTableId",
                tableName,
                "srcNamespace",
                NAMESPACE,
                "inputFilePattern",
                getGcsPath(COMPRESSED_BACKUPS_DIR) + "/*",
                "backupCompressionAlgorithm",
                COMPRESSION_ALG));
    assertThatPipeline(launchInfo).isRunning();
    pipelineOperator().waitUntilDone(createConfig(launchInfo));

    // Assert
    List<Row> rows = bigtableResourceManager.readTable(tableName);
    assertThatBigtableRecords(rows, RowBuilder.SCALAR_FAMILY).hasRows(1);

    // Make sure that error output file is empty
    List<Artifact> artifacts =
        gcsClient.listArtifacts(testName, Pattern.compile(".*" + OUTPUT_FILE + ".*"));
    assertThat(artifacts.isEmpty()).isFalse();
    String allMessages =
        artifacts.stream()
            .map(artifact -> new String(artifact.contents()))
            .collect(Collectors.joining());
    assertThat(allMessages.trim()).isEmpty();
  }

  @Test
  public void testBadNamespaceReadsNoRecords() throws IOException {
    // Arrange
    String tableName = "tbl_badNamespace" + randomAlphanumeric(8).toLowerCase();
    BigtableTableSpec tableSpec = new BigtableTableSpec();
    tableSpec.setColumnFamilies(
        List.of(RowBuilder.SCALAR_FAMILY, RowBuilder.COMPLEX_OBJECT_FAMILY));
    bigtableResourceManager.createTable(tableName, tableSpec);

    // Act
    LaunchInfo launchInfo =
        launchTemplate(
            Map.of(
                "bigtableWriteTableId",
                tableName,
                "srcNamespace",
                NAMESPACE + "-NOT",
                "inputFilePattern",
                getGcsPath(BACKUPS_DIR) + "/*"));
    assertThatPipeline(launchInfo).isRunning();
    pipelineOperator().waitUntilDone(createConfig(launchInfo));

    // Assert
    List<Row> rows = bigtableResourceManager.readTable(tableName);
    assertThatBigtableRecords(rows, RowBuilder.SCALAR_FAMILY).hasRows(0);

    // Make sure that error output file is also empty - no records were read
    List<Artifact> artifacts =
        gcsClient.listArtifacts(testName, Pattern.compile(".*" + OUTPUT_FILE + ".*"));
    assertThat(artifacts.isEmpty()).isFalse();
    String allMessages =
        artifacts.stream()
            .map(artifact -> new String(artifact.contents()))
            .collect(Collectors.joining());
    assertThat(allMessages.trim()).isEmpty();
  }

  @Test
  public void testInvalidRecordsGoToErrorOutput() throws IOException {
    // Arrange
    String tableName = "tbl_invalidRecords" + randomAlphanumeric(8).toLowerCase();
    // We do not create the column families required by the records, so that
    // writes to Bigtable fail and all the records go to the error output.
    BigtableTableSpec tableSpec = new BigtableTableSpec();
    tableSpec.setColumnFamilies(List.of(RowBuilder.SCALAR_FAMILY + "-NOT"));
    bigtableResourceManager.createTable(tableName, tableSpec);

    // Act

    LaunchInfo launchInfo =
        launchTemplate(
            Map.of(
                "bigtableWriteTableId",
                tableName,
                "srcNamespace",
                NAMESPACE,
                "validateTableSchema",
                "false",
                "inputFilePattern",
                getGcsPath(BACKUPS_DIR) + "/*"));
    assertThatPipeline(launchInfo).isRunning();
    pipelineOperator().waitUntilDone(createConfig(launchInfo));

    // Assert
    List<Row> rows = bigtableResourceManager.readTable(tableName);
    assertThatBigtableRecords(rows, RowBuilder.SCALAR_FAMILY).hasRows(0);

    // Make sure that error output file contains problematic records
    List<Artifact> artifacts =
        gcsClient.listArtifacts(testName, Pattern.compile(".*" + OUTPUT_FILE + ".*"));
    assertThat(artifacts.isEmpty()).isFalse();
    String allMessages =
        artifacts.stream()
            .map(artifact -> new String(artifact.contents()))
            .collect(Collectors.joining());
    Pattern pattern = Pattern.compile("\\{[^}]*\"digest\"[^}]*\\}");
    Matcher matcher = pattern.matcher(allMessages);
    int count = 0;
    while (matcher.find()) {
      count++;
    }
    assertThat(count).isEqualTo(12); // there are 12 records in the backups
  }

  @Test
  public void testNoCFFails() throws IOException {
    // Arrange
    String tableName = "tbl_noCFFails" + randomAlphanumeric(8).toLowerCase();
    BigtableTableSpec tableSpec = new BigtableTableSpec();
    tableSpec.setColumnFamilies(List.of(RowBuilder.SCALAR_FAMILY + "-NOT"));
    bigtableResourceManager.createTable(tableName, tableSpec);

    Map<String, String> launchTemplateArgs =
        Map.of(
            "bigtableWriteTableId",
            tableName,
            "srcNamespace",
            NAMESPACE,
            "inputFilePattern",
            getGcsPath(BACKUPS_DIR) + "/*");
    if (System.getProperty("directRunnerTest") != null) {
      LaunchInfo launchInfo = launchTemplate(launchTemplateArgs);
      assertThatPipeline(launchInfo).isRunning();
      assertThrows(
          RuntimeException.class, () -> pipelineOperator().waitUntilDone(createConfig(launchInfo)));
    } else {
      assertThrows(RuntimeException.class, () -> launchTemplate(launchTemplateArgs));
    }
  }

  private LaunchInfo launchTemplate(Map<String, String> additionalParameters) throws IOException {
    PipelineLauncher.LaunchConfig.Builder builder =
        PipelineLauncher.LaunchConfig.builder(removeUnsafeCharacters(testName), specPath)
            .addParameter("bigtableWriteInstanceId", bigtableResourceManager.getInstanceId())
            // we don't want the number of records in the backups (12) to be divisible by
            // bigtableBulkWriteMaxRowKeyCount, to make sure we test the batching works properly
            .addParameter("bigtableBulkWriteMaxRowKeyCount", "5")
            .addParameter("bigtableWriteAppProfile", appProfileId)
            .addParameter(
                "outputPath",
                getGcsPath(testName + "/" + OUTPUT_FILE + randomAlphanumeric(8).toLowerCase()));
    for (Map.Entry<String, String> additionalParameter : additionalParameters.entrySet()) {
      builder.addParameter(additionalParameter.getKey(), additionalParameter.getValue());
    }
    builder.addParameter("sdkContainerImage", System.getProperty("sdkContainerImage"));
    return super.launchTemplate(builder);
  }

  private String removeUnsafeCharacters(String testName) {
    return testName.replaceAll("[\\[\\]]", "-");
  }
}
