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

/** Represents a float value read from Aerospike backup. */
public final class FloatValue extends Value {
  /** The float value. */
  public final double value;

  /**
   * Constructs a FloatValue with the given value.
   *
   * @param value the float value
   */
  public FloatValue(Double value) {
    Objects.requireNonNull(value, this.getClass().getName() + "'s value must not be null.");
    this.value = value;
  }

  /**
   * Returns a string representation of the float value.
   *
   * @return the string representation
   */
  @Override
  public String toString() {
    return "FloatValue: " + value;
  }

  /**
   * Sets the float value in the specified bin.
   *
   * @param binName the name of the bin
   * @param rowBuilder the {@link RowBuilder} to modify
   */
  @Override
  public void setCell(String binName, RowBuilder<?> rowBuilder) {
    rowBuilder.addValue(binName, value);
  }

  /** A {@link JsonAdapter} for (de)serializing {@link FloatValue} as JSON number. */
  static class MoshiJsonAdapter extends JsonAdapter<FloatValue> {
    @Override
    public FloatValue fromJson(JsonReader jsonReader) {
      throw new UnsupportedOperationException();
    }

    @Override
    public void toJson(JsonWriter jsonWriter, FloatValue value) throws IOException {
      Value.getJsonAdapter(String.class)
          .toJson(
              jsonWriter,
              Optional.ofNullable(value).map(v -> Double.toString(v.value)).orElse(null));
    }
  }
}
