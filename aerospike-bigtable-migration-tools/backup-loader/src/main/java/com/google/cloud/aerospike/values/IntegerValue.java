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
import java.util.Objects;
import java.util.Optional;

/** Represents an integer value read from an Aerospike backup. */
public final class IntegerValue extends Value {
  public final long value;

  /**
   * Constructs an IntegerValue with the specified integer.
   *
   * @param value the integer value
   */
  public IntegerValue(Long value) {
    Objects.requireNonNull(value, this.getClass().getName() + "'s value must not be null.");
    this.value = value;
  }

  /**
   * Returns a string representation of the integer value.
   *
   * @return a string in the format "IntegerValue: <value>"
   */
  @Override
  public String toString() {
    return "IntegerValue: " + value;
  }

  /**
   * Sets the integer value in the specified Aerospike row mutation.
   *
   * @param binName the name of the bin
   * @param rowBuilder the {@link RowBuilder} to modify
   */
  @Override
  public void setCell(String binName, RowBuilder<?> rowBuilder) {
    rowBuilder.addValue(binName, value);
  }

  /** A {@link JsonAdapter} for (de)serializing {@link IntegerValue} as JSON number. */
  static class MoshiJsonAdapter extends JsonAdapter<IntegerValue> {
    @Override
    public IntegerValue fromJson(JsonReader jsonReader) {
      throw new UnsupportedOperationException();
    }

    @Override
    public void toJson(JsonWriter jsonWriter, IntegerValue value) throws IOException {
      Value.getJsonAdapter(Long.class)
          .toJson(jsonWriter, Optional.ofNullable(value).map(v -> v.value).orElse(null));
    }
  }
}
