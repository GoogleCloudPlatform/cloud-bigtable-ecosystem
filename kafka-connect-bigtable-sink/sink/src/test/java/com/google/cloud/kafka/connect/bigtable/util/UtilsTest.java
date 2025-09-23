package com.google.cloud.kafka.connect.bigtable.util;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertThrows;

import com.google.cloud.kafka.connect.bigtable.exception.BigtableSinkLogicError;
import com.google.common.collect.Collections2;
import java.util.AbstractMap;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public class UtilsTest {
  @Test
  public void testOrderMapSuccesses() {
    Integer key1 = 1;
    Integer key2 = 2;
    Integer key3 = 3;
    Integer key4 = 4;

    String value1 = "value1";
    String value2 = "value2";
    String value3 = "value3";
    String value4 = "value4";

    Map<Integer, String> map1 = new LinkedHashMap<>();
    map1.put(key4, value4);
    map1.put(key3, value3);
    map1.put(key2, value2);
    map1.put(key1, value1);

    assertEquals(List.of(key4, key3, key2, key1), new ArrayList<>(map1.keySet()));
    assertEquals(List.of(value4, value3, value2, value1), new ArrayList<>(map1.values()));
    assertEquals(
        List.of(key1, key2, key3, key4),
        new ArrayList<>(
            com.google.cloud.kafka.connect.bigtable.utils.Utils.orderMap(map1, List.of(key1, key2, key3, key4)).keySet()));
    assertEquals(
        List.of(value1, value2, value3, value4),
        new ArrayList<>(
            com.google.cloud.kafka.connect.bigtable.utils.Utils.orderMap(map1, List.of(key1, key2, key3, key4)).values()));
    assertEquals(
        List.of(
            new AbstractMap.SimpleImmutableEntry<>(key1, value1),
            new AbstractMap.SimpleImmutableEntry<>(key2, value2),
            new AbstractMap.SimpleImmutableEntry<>(key3, value3),
            new AbstractMap.SimpleImmutableEntry<>(key4, value4)),
        new ArrayList<>(
            com.google.cloud.kafka.connect.bigtable.utils.Utils.orderMap(map1, List.of(key1, key2, key3, key4)).entrySet()));

    assertEquals(
        List.of(key1, key2, key3, key4),
        new ArrayList<>(
            com.google.cloud.kafka.connect.bigtable.utils.Utils.orderMap(map1, List.of(-1, key1, -2, key2, -3, key3, -4, key4, -5))
                .keySet()));

    Collections2.permutations(List.of(key1, key2, key3, key4))
        .forEach(
            p -> {
              assertEquals(p, new ArrayList<>(
                  com.google.cloud.kafka.connect.bigtable.utils.Utils.orderMap(map1, p).keySet()));
            });
  }

  @Test
  public void testOrderMapError() {
    Map<Integer, String> map = Map.of(1, "1", 2, "2", -1, "-1");
    assertThrows(
        BigtableSinkLogicError.class, () -> com.google.cloud.kafka.connect.bigtable.utils.Utils.orderMap(map, Set.of(1, 2)));
  }
}
