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
import java.util.List;
import java.util.Objects;
import java.util.Optional;
import java.util.StringJoiner;

/** Represents a list value read from an Aerospike backup. */
public final class ListValue extends Value {
  private final List<Object> list;

  /** Constructs an ListValue from a list of objects. */
  public ListValue(List<Object> list) {
    Objects.requireNonNull(list, this.getClass().getName() + "'s value must not be null.");
    this.list = list;
  }

  /** Returns the list of values. */
  public List<Object> getList() {
    return list;
  }

  /**
   * Returns a string representation of the list value.
   *
   * @return a string representation of the list.
   */
  @Override
  public String toString() {
    StringJoiner joiner = new StringJoiner(", ", "ListValue[", "]");
    for (Object value : list) {
      joiner.add(Optional.ofNullable(value).map(Object::toString).orElse("null"));
    }
    return joiner.toString();
  }

  /**
   * Sets a cell in Aerospike. Not implemented for ListValue yet.
   *
   * @param binName the name of the bin.
   * @param rowBuilder the {@link RowBuilder} to modify
   */
  @Override
  public void setCell(String binName, RowBuilder<?> rowBuilder) {
    rowBuilder.addValue(binName, this.list);
  }

  /** A {@link JsonAdapter} for (de)serializing {@link ListValue} as JSON array. */
  static class MoshiJsonAdapter extends JsonAdapter<ListValue> {
    @Override
    public ListValue fromJson(JsonReader jsonReader) {
      throw new UnsupportedOperationException();
    }

    @Override
    public void toJson(JsonWriter jsonWriter, ListValue value) throws IOException {
      Value.getJsonAdapter(List.class)
          .toJson(jsonWriter, Optional.ofNullable(value).map(v -> v.list).orElse(null));
    }
  }
}
