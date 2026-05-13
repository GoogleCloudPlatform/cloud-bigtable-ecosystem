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
package com.google.cloud.aerospike.util;

import com.google.cloud.aerospike.MapAerospikeConnectJsonToBigtableSinkInput;
import com.google.protobuf.ByteString;
import com.squareup.moshi.Moshi;
import java.util.ArrayList;
import java.util.Base64;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

public class AerospikeConnectXdrJson {
  private static final Moshi MOSHI = new Moshi.Builder().build();

  private static Map<String, Object> value(
      Object timestamp, Object digest, Object type, Object bins) {
    Map<String, Object> result = new HashMap<>();
    if (timestamp != null) {
      result.put(MapAerospikeConnectJsonToBigtableSinkInput.TIMESTAMP_FIELD, timestamp);
    }
    if (digest != null) {
      List<Object> keyObject = List.of("fakeNamespace", "fakeSet", digest, "fakeUserKey");
      result.put(MapAerospikeConnectJsonToBigtableSinkInput.KEY_FIELD, keyObject);
    }
    if (type != null) {
      result.put(MapAerospikeConnectJsonToBigtableSinkInput.TYPE_FIELD, type);
    }
    if (bins != null) {
      result.put(MapAerospikeConnectJsonToBigtableSinkInput.BINS_FIELD, bins);
    }
    return result;
  }

  private static List<Object> binsValue(List<Bin> binsParams) {
    List<Object> binsField = new ArrayList<>();
    for (Bin binParams : binsParams) {
      Map<Object, Object> bin = new HashMap<>();
      if (binParams.name != null) {
        bin.put(MapAerospikeConnectJsonToBigtableSinkInput.BIN_NAME_FIELD, binParams.name);
      }
      if (binParams.type != null) {
        bin.put(MapAerospikeConnectJsonToBigtableSinkInput.BIN_TYPE_FIELD, binParams.type);
      }
      if (binParams.value != null) {
        bin.put(MapAerospikeConnectJsonToBigtableSinkInput.BIN_VALUE_FIELD, binParams.value);
      }
      binsField.add(bin);
    }
    return binsField;
  }

  public record Bin(Object name, Object type, Object value) {}

  public static Map<String, Object> deleteMessageMap(Long timestamp, byte[] key) {
    return value(
        timestamp,
        Base64.getEncoder().encodeToString(key),
        MapAerospikeConnectJsonToBigtableSinkInput.MessageType.DELETE.name(),
        null);
  }

  public static Map<String, ?> writeMessageMap(Long timestamp, byte[] key, List<Bin> bins) {
    return value(
        timestamp,
        Base64.getEncoder().encodeToString(key),
        MapAerospikeConnectJsonToBigtableSinkInput.MessageType.WRITE.name(),
        binsValue(bins));
  }

  public static String deleteMessage(Long timestamp, ByteString key) {
    return MOSHI.adapter(Map.class).toJson(deleteMessageMap(timestamp, key.toByteArray()));
  }

  public static String writeMessage(Long timestamp, ByteString key, List<Bin> bins) {
    return MOSHI.adapter(Map.class).toJson(writeMessageMap(timestamp, key.toByteArray(), bins));
  }
}
