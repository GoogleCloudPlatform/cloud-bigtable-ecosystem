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

import com.google.cloud.aerospike.values.Value;
import com.squareup.moshi.JsonWriter;
import java.io.IOException;
import java.io.Serializable;
import java.nio.charset.StandardCharsets;
import java.util.Base64;
import java.util.HashMap;
import java.util.Map;
import okio.Buffer;

/** Represents the result of reading a record from an Aerospike backup. */
public class ReadRecordResult implements Serializable {
  /** The digest of the record. */
  public final byte[] digest;

  /** A map of bin names to their values. */
  public final Map<String, Value> bins;

  /**
   * Constructs a ReadRecordResult with the given digest.
   *
   * @param digest The digest of the record.
   */
  public ReadRecordResult(byte[] digest) {
    this.digest = digest;
    this.bins = new HashMap<>();
  }

  /**
   * Adds a bin value to the result.
   *
   * @param binName The name of the bin.
   * @param value The value of the bin.
   */
  public void addBinValue(String binName, Value value) {
    bins.put(binName, value);
  }

  public String base64Digest() {
    return Base64.getEncoder().encodeToString(digest);
  }

  public String toJson() throws IOException {
    try (Buffer out = new Buffer()) {
      JsonWriter writer = JsonWriter.of(out);
      writer
          .beginObject()
          .name("digest")
          .value(base64Digest())
          .name("bins")
          .value(Value.getJsonAdapter(Map.class).toJson(bins))
          .endObject();
      return out.readString(StandardCharsets.UTF_8);
    }
  }
}
