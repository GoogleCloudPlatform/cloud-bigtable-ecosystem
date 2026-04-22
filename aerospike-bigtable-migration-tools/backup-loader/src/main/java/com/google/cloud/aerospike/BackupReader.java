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
package com.google.cloud.aerospike;

import com.google.cloud.aerospike.exceptions.JNIException;
import com.google.cloud.aerospike.exceptions.MalformedBackupDataException;
import com.google.cloud.aerospike.exceptions.UnsupportedBackupEntryException;
import com.google.cloud.aerospike.values.BooleanValue;
import com.google.cloud.aerospike.values.BytesValue;
import com.google.cloud.aerospike.values.FloatValue;
import com.google.cloud.aerospike.values.IntegerValue;
import com.google.cloud.aerospike.values.ListValue;
import com.google.cloud.aerospike.values.MapValue;
import com.google.cloud.aerospike.values.StringValue;
import com.google.cloud.aerospike.values.Value;
import java.io.IOException;
import java.nio.charset.Charset;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * The BackupReader class provides native methods to interact with backup files. Important Note:
 * this class is **not** thread-safe.
 */
public class BackupReader implements AutoCloseable {

  public enum CompressionAlgorithm {
    /** No compression */
    NONE,
    /** ZSTD compression */
    ZSTD
  }

  private static final Logger LOG = LoggerFactory.getLogger(BackupReader.class);
  private static final Charset charset = StandardCharsets.UTF_8;

  // Order is important - we need to execute this block after setting up static objects the
  // library's initialization uses.
  static {
    try {
      LOG.info("LOADING backupreader LIBRARY FROM {}...", System.mapLibraryName("backupreader"));
      System.loadLibrary("backupreader");
    } catch (Exception e) {
      LOG.error("Error while loading the backupreader library", e);
      throw e;
    }
  }

  private long filePtr = 0;

  private static Value createValue(Object value) {
    if (value instanceof Boolean b) {
      return new BooleanValue(b);
    } else if (value instanceof byte[] b) {
      return new BytesValue(b);
    } else if (value instanceof Double d) {
      return new FloatValue(d);
    } else if (value instanceof Long l) {
      return new IntegerValue(l);
    } else if (value instanceof List) {
      return new ListValue((List<Object>) value);
    } else if (value instanceof Map) {
      return new MapValue((Map<Object, Object>) value);
    } else if (value instanceof String s) {
      return new StringValue(s);
    } else {
      throw new IllegalArgumentException(
          "'"
              + Optional.ofNullable(value).map(o -> o.getClass().getName()).orElse("null")
              + "' values are not supported");
    }
  }

  /**
   * Constructs a BackupReader and opens the backup file for reading.
   *
   * @param filePath the name of the file to open.
   * @throws IOException if an I/O error occurs while opening the file.
   * @throws JNIException if some JNI operation fails.
   * @throws IllegalArgumentException if the filePath is null or blank.
   * @throws MalformedBackupDataException if backup file was corrupted or not readable.
   */
  public BackupReader(String filePath, CompressionAlgorithm compAlg)
      throws IOException, JNIException, IllegalArgumentException, MalformedBackupDataException {
    if (filePath == null || filePath.isBlank()) {
      throw new IllegalArgumentException("Filename must not be null nor blank.");
    }
    openFile(filePath, compAlg == CompressionAlgorithm.ZSTD);
  }

  /**
   * Opens a backup file for reading and stores a file descriptor in filePtr.
   *
   * @param filePath the path to the file to open.
   * @param useZstdCompression must be set to true when opening a compressed backup file, false
   *     otherwise.
   * @throws IOException if an I/O error occurs while opening the file.
   * @throws JNIException if some JNI operation fails.
   * @throws MalformedBackupDataException if backup file was corrupted or not readable.
   */
  private native void openFile(String filePath, boolean useZstdCompression)
      throws IOException, JNIException, MalformedBackupDataException;

  /**
   * Closes the currently opened backup file. If the file was already closed then noop.
   *
   * @throws IOException if an I/O error occurs while closing the file.
   * @throws JNIException if some JNI operation fails.
   */
  private native void closeFile() throws IOException, JNIException;

  /**
   * Closes the BackupReader, releasing held resources (if any). This method is idempotent.
   *
   * @throws IOException if an I/O error occurs while closing the file.
   * @throws JNIException if some JNI operation fails.
   */
  @Override
  public void close() throws IOException, JNIException {
    closeFile();
  }

  /**
   * Reads a record from the backup file.
   *
   * <p>This function does not support all legal entries that can be present in a backup. In
   * particular, it only supports data records holding the following types: Boolean, Bytes, Float,
   * Integer, List, Map, String.
   *
   * <p>Example unsupported backup entries are e.g., indices, UDFs, or records holding GeoJSON.
   *
   * @param namespace the namespace to read from
   * @return a ReadRecordResult object containing the row data or null if no more records are
   *     available.
   * @throws UnsupportedBackupEntryException If an unsupported backup entry was read.
   * @throws MalformedBackupDataException if record data was corrupted or not readable.
   * @throws IOException if an I/O error occurs while reading the record.
   * @throws JNIException if a JNI operation fails for a reason not listed above.
   */
  public native ReadRecordResult readRecord(String namespace)
      throws IOException,
          JNIException,
          MalformedBackupDataException,
          UnsupportedBackupEntryException;
}
