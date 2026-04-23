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
package com.google.cloud.aerospike.values;

import com.google.cloud.aerospike.RowBuilder;
import com.squareup.moshi.JsonAdapter;
import com.squareup.moshi.JsonReader;
import com.squareup.moshi.JsonWriter;
import java.io.IOException;
import java.util.Base64;
import java.util.Objects;
import java.util.Optional;

/** Represents a bytes value read from Aerospike backup. */
public final class BytesValue extends Value {
  /** The bytes value represented by this class. */
  public final byte[] value;

  /**
   * Constructs a BytesValue with provided value.
   *
   * @param value the bytes value
   */
  public BytesValue(byte[] value) {
    Objects.requireNonNull(value, this.getClass().getName() + "'s value must not be null.");
    this.value = value;
  }

  /** Returns the boolean value. */
  @Override
  public String toString() {
    return "BytesValue: " + Base64.getEncoder().encodeToString(value);
  }

  /**
   * Sets the value of a bin in an Aerospike row mutation.
   *
   * @param binName the name of the bin
   * @param rowBuilder the {@link RowBuilder} to modify
   */
  @Override
  public void setCell(String binName, RowBuilder<?> rowBuilder) {
    rowBuilder.addValue(binName, value);
  }

  /** A {@link JsonAdapter} for (de)serializing {@link BytesValue} as JSON base64-encoded string. */
  static class MoshiJsonAdapter extends JsonAdapter<BytesValue> {
    @Override
    public BytesValue fromJson(JsonReader jsonReader) {
      throw new UnsupportedOperationException();
    }

    @Override
    public void toJson(JsonWriter jsonWriter, BytesValue value) throws IOException {
      Value.getJsonAdapter(String.class)
          .toJson(
              jsonWriter,
              Optional.ofNullable(value)
                  .map(v -> Base64.getEncoder().encodeToString(v.value))
                  .orElse(null));
    }
  }
}
