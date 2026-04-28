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

/** Represents a string value read from an Aerospike backup. */
public final class StringValue extends Value {
  /** The string value. */
  public final String value;

  /**
   * Constructs a StringValue with the given string.
   *
   * @param value the string value
   */
  public StringValue(String value) {
    Objects.requireNonNull(value, this.getClass().getName() + "'s value must not be null.");
    this.value = value;
  }

  /**
   * Returns a string representation of this value.
   *
   * @return the string representation
   */
  @Override
  public String toString() {
    return "StringValue: " + value;
  }

  /**
   * Sets the value in the specified Aerospike row mutation.
   *
   * @param binName the name of the bin
   * @param rowBuilder the {@link RowBuilder} to modify
   */
  @Override
  public void setCell(String binName, RowBuilder<?> rowBuilder) {
    rowBuilder.addValue(binName, value);
  }

  /** A {@link JsonAdapter} for (de)serializing {@link ListValue} as JSON string. */
  static class MoshiJsonAdapter extends JsonAdapter<StringValue> {
    @Override
    public StringValue fromJson(JsonReader jsonReader) {
      throw new UnsupportedOperationException();
    }

    @Override
    public void toJson(JsonWriter jsonWriter, StringValue value) throws IOException {
      Value.getJsonAdapter(String.class)
          .toJson(jsonWriter, Optional.ofNullable(value).map(v -> v.value).orElse(null));
    }
  }
}
