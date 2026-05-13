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

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

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
import com.google.protobuf.ByteString;
import java.io.IOException;
import java.util.AbstractMap;
import java.util.Base64;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;
import org.junit.jupiter.api.Test;

public class BackupReaderTest {
  private static String getResourcePath(String resourceName) {
    return "target/test-classes/" + resourceName;
  }

  private static BackupReader.CompressionAlgorithm NO_COMP = BackupReader.CompressionAlgorithm.NONE;
  private static BackupReader.CompressionAlgorithm ZSTD = BackupReader.CompressionAlgorithm.ZSTD;

  public record UnwrappedReadRecordResult(ByteString digest, Map<String, Object> value) {
    public UnwrappedReadRecordResult(ReadRecordResult readRecordResult) {
      this(
          ByteString.copyFrom(readRecordResult.digest),
          readRecordResult.bins.entrySet().stream()
              .map(
                  e ->
                      new AbstractMap.SimpleImmutableEntry<>(e.getKey(), unwrapValue(e.getValue())))
              .collect(Collectors.toMap(Map.Entry::getKey, Map.Entry::getValue)));
    }

    private static Object unwrapValue(Value v) {
      if (v instanceof BooleanValue booleanValue) {
        return booleanValue.value;
      } else if (v instanceof BytesValue bytesValue) {
        return bytesValue.value;
      } else if (v instanceof FloatValue floatValue) {
        return floatValue.value;
      } else if (v instanceof IntegerValue integerValue) {
        return integerValue.value;
      } else if (v instanceof ListValue listValue) {
        return listValue.getList();
      } else if (v instanceof MapValue mapValue) {
        return mapValue.getMap();
      } else if (v instanceof StringValue stringValue) {
        return stringValue.value;
      } else {
        throw new IllegalArgumentException();
      }
    }
  }

  @Test
  public void testInvalidPath() {
    assertThrows(IllegalArgumentException.class, () -> new BackupReader(null, NO_COMP));
    assertThrows(IllegalArgumentException.class, () -> new BackupReader(" ", NO_COMP));
    assertThrows(IOException.class, () -> new BackupReader("/nonexistent", NO_COMP));
  }

  @Test
  public void testEmptyBackup()
      throws MalformedBackupDataException, IOException, UnsupportedBackupEntryException {
    for (String testCase : List.of("empty.asb", "empty-no-metadata.asb")) {
      try (BackupReader backupReader = new BackupReader(getResourcePath(testCase), NO_COMP)) {
        assertNull(backupReader.readRecord("whatever"));
      }
    }
  }

  @Test
  public void testHappyPath()
      throws MalformedBackupDataException, IOException, UnsupportedBackupEntryException {
    try (BackupReader backupReader = new BackupReader(getResourcePath("basic.asb"), NO_COMP)) {
      UnwrappedReadRecordResult result =
          new UnwrappedReadRecordResult(backupReader.readRecord("test"));
      assertEquals(
          new UnwrappedReadRecordResult(
              ByteString.copyFrom(Base64.getDecoder().decode("7JEZLUt/jONdXXjTS8ply6qqyWA=")),
              Map.of("foo", 123L, "bar", "abc", "baz", true)),
          result);
      assertNull(backupReader.readRecord("test"));
    }
  }

  @Test
  public void testScalars()
      throws MalformedBackupDataException, IOException, UnsupportedBackupEntryException {
    try (BackupReader backupReader = new BackupReader(getResourcePath("scalars.asb"), NO_COMP)) {
      UnwrappedReadRecordResult result =
          new UnwrappedReadRecordResult(backupReader.readRecord("test"));
      assertArrayEquals(Base64.getDecoder().decode("FSU="), (byte[]) result.value.get("corge"));
      result.value.remove("corge");
      assertEquals(
          new UnwrappedReadRecordResult(
              ByteString.copyFrom(Base64.getDecoder().decode("7JEZLUt/jONdXXjTS8ply6qqyWA=")),
              Map.of(
                  "foo",
                  123L,
                  "bar",
                  "abc",
                  "baz",
                  true,
                  "qux",
                  Double.POSITIVE_INFINITY,
                  "quux",
                  "xD")),
          result);
      assertNull(backupReader.readRecord("test"));
    }
  }

  @Test
  public void testObjects()
      throws MalformedBackupDataException, UnsupportedBackupEntryException, IOException {
    try (BackupReader backupReader = new BackupReader(getResourcePath("objects.asb"), NO_COMP)) {
      UnwrappedReadRecordResult result =
          new UnwrappedReadRecordResult(backupReader.readRecord("test"));

      assertEquals(
          new UnwrappedReadRecordResult(
              ByteString.copyFrom(Base64.getDecoder().decode("0T4dkNzFvNj6lQeYYlS3WmhnlNs=")),
              Map.of(
                  "list_empty", List.of(),
                  "list_scalars", List.of(1L, 2L, 3L),
                  "list_nested",
                      List.of(List.of(), List.of(1L), List.of(1L, List.of(), List.of(1L))),
                  "map_empty", Map.of(),
                  "map_scalars", Map.of(1L, "a", 2L, "b", 3L, "c"),
                  "map_nested",
                      Map.of(
                          1L,
                          Map.of(),
                          2L,
                          Map.of(1L, "a"),
                          3L,
                          Map.of(1L, Map.of(1L, "b"), 2L, Map.of())),
                  "mixed_nested",
                      Map.of(
                          1L,
                          List.of(),
                          2L,
                          Map.of(1L, List.of(), 2L, Map.of(3L, "a")),
                          3L,
                          List.of(Map.of(1L, "a"), Map.of(2L, "b"))))),
          result);
      assertNull(backupReader.readRecord("test"));
    }
  }

  @Test
  public void testStringContainingNewlines()
      throws MalformedBackupDataException, IOException, UnsupportedBackupEntryException {
    try (BackupReader backupReader =
        new BackupReader(getResourcePath("string-newlines.asb"), NO_COMP)) {
      UnwrappedReadRecordResult result =
          new UnwrappedReadRecordResult(backupReader.readRecord("test"));
      assertEquals(Map.of("str", "\n".repeat(7) + "EOF"), result.value);
      assertNull(backupReader.readRecord("test"));
    }
  }

  @Test
  public void testMultipleNamespaces()
      throws MalformedBackupDataException, IOException, UnsupportedBackupEntryException {
    // TODO: check if it is a valid backup file once we receive an Aerospike Enterprise license key.
    //  It might not be a valid, because it lacks `# namespace {namespace}`.
    try (BackupReader backupReader =
        new BackupReader(getResourcePath("multiple-namespaces.asb"), NO_COMP)) {
      UnwrappedReadRecordResult result1 =
          new UnwrappedReadRecordResult(backupReader.readRecord("test"));
      UnwrappedReadRecordResult result2 =
          new UnwrappedReadRecordResult(backupReader.readRecord("test2"));
      assertNull(backupReader.readRecord("whatever"));

      for (UnwrappedReadRecordResult result : List.of(result1, result2)) {
        assertEquals(
            new UnwrappedReadRecordResult(
                ByteString.copyFrom(Base64.getDecoder().decode("7JEZLUt/jONdXXjTS8ply6qqyWA=")),
                Map.of("foo", 123L, "bar", "abc", "baz", true)),
            result);
      }
    }
  }

  @Test
  public void testWrongNamespace()
      throws MalformedBackupDataException, IOException, UnsupportedBackupEntryException {
    try (BackupReader backupReader = new BackupReader(getResourcePath("basic.asb"), NO_COMP)) {
      assertNull(backupReader.readRecord("wrong"));
      // We already skipped over the only record from the backup file.
      assertNull(backupReader.readRecord("test"));
    }
  }

  @Test
  public void testInvalidNamespace()
      throws MalformedBackupDataException, IOException, UnsupportedBackupEntryException {
    for (String invalidNamespace : new String[] {null, "", "a,b"}) {
      try (BackupReader backupReader = new BackupReader(getResourcePath("basic.asb"), NO_COMP)) {
        assertThrows(JNIException.class, () -> backupReader.readRecord(invalidNamespace));
        assertNotNull(backupReader.readRecord("test"));
        assertNull(backupReader.readRecord("test"));
      }
    }
  }

  @Test
  public void testBasicInvalidRecord()
      throws MalformedBackupDataException, IOException, UnsupportedBackupEntryException {
    try (BackupReader backupReader = new BackupReader(getResourcePath("malformed.asb"), NO_COMP)) {
      assertThrows(MalformedBackupDataException.class, () -> backupReader.readRecord("test"));
      UnwrappedReadRecordResult result =
          new UnwrappedReadRecordResult(backupReader.readRecord("test"));
      // Note that a non-terminal bin is invalid, but the subsequent read succeeds.
      assertEquals(
          new UnwrappedReadRecordResult(
              ByteString.copyFrom(Base64.getDecoder().decode("7JEZLUt/jONdXXjTS8ply6qqyWA=")),
              Map.of("foo", 123L, "bar", "abc", "baz", true)),
          result);
      assertThrows(MalformedBackupDataException.class, () -> backupReader.readRecord("test"));
      // Note that a non-terminal bin is invalid, but the subsequent read cleanly returns a `null`
      // signifying EOF.
      assertNull(backupReader.readRecord("test"));
    }
  }

  @Test
  public void testUdfEntry()
      throws MalformedBackupDataException, IOException, UnsupportedBackupEntryException {
    try (BackupReader backupReader = new BackupReader(getResourcePath("udf.asb"), NO_COMP)) {
      assertThrows(UnsupportedBackupEntryException.class, () -> backupReader.readRecord("test"));
      assertNull(backupReader.readRecord("test"));
    }
  }

  @Test
  public void testIndexEntry()
      throws MalformedBackupDataException, IOException, UnsupportedBackupEntryException {
    try (BackupReader backupReader = new BackupReader(getResourcePath("index.asb"), NO_COMP)) {
      assertThrows(UnsupportedBackupEntryException.class, () -> backupReader.readRecord("test"));
      assertNull(backupReader.readRecord("test"));
    }
  }

  @Test
  public void testUnsupportedTypeEntry()
      throws MalformedBackupDataException, IOException, UnsupportedBackupEntryException {
    try (BackupReader backupReader = new BackupReader(getResourcePath("geojson.asb"), NO_COMP)) {
      assertThrows(UnsupportedBackupEntryException.class, () -> backupReader.readRecord("test"));
      assertNull(backupReader.readRecord("test"));
    }
  }

  @Test
  public void testCloseIsIdempotent() throws MalformedBackupDataException, IOException {
    BackupReader backupReader = new BackupReader(getResourcePath("basic.asb"), NO_COMP);
    backupReader.close();
    backupReader.close();
  }

  @Test
  public void testEndCanBeReadMultipleTimes()
      throws MalformedBackupDataException, IOException, UnsupportedBackupEntryException {
    try (BackupReader backupReader = new BackupReader(getResourcePath("empty.asb"), NO_COMP)) {
      assertNull(backupReader.readRecord("whatever"));
      assertNull(backupReader.readRecord("whatever"));
      assertNull(backupReader.readRecord("whatever"));
    }
  }

  @Test
  public void testMalformedVersion() {
    assertThrows(
        MalformedBackupDataException.class,
        () -> new BackupReader(getResourcePath("basic-malformed-version.asb"), NO_COMP));
  }

  @Test
  public void testInvalidByteDeserialization()
      throws MalformedBackupDataException, IOException, UnsupportedBackupEntryException {
    try (BackupReader backupReader =
        new BackupReader(getResourcePath("invalid-objects.asb"), NO_COMP)) {
      JNIException thrown = assertThrows(JNIException.class, () -> backupReader.readRecord("test"));
      boolean messageMatches = false;
      for (Throwable e = thrown; e != null; e = e.getCause()) {
        if (e.getMessage().contains("Failed to deserialize")) {
          messageMatches = true;
        }
      }
      assertTrue(messageMatches);
      assertNull(backupReader.readRecord("test"));
    }
  }

  @Test
  public void testCompression()
      throws MalformedBackupDataException, IOException, UnsupportedBackupEntryException {
    try (BackupReader backupReader = new BackupReader(getResourcePath("compressed.asb"), ZSTD)) {
      UnwrappedReadRecordResult result =
          new UnwrappedReadRecordResult(backupReader.readRecord("test"));
      assertEquals(
          new UnwrappedReadRecordResult(
              ByteString.copyFrom(Base64.getDecoder().decode("4BEodXo5XOj4luxDr9ypWZ5FiVE=")),
              Map.of("foo", 123L, "bar", "abc", "baz", true)),
          result);
      assertNull(backupReader.readRecord("test"));
    }
  }

  @Test
  public void testWrongCompressionMode() {
    assertThrows(IOException.class, () -> new BackupReader(getResourcePath("basic.asb"), ZSTD));
  }
}
