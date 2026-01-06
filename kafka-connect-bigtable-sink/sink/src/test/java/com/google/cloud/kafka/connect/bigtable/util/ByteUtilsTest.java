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
package com.google.cloud.kafka.connect.bigtable.util;

import static org.junit.Assert.assertArrayEquals;

import com.google.cloud.kafka.connect.bigtable.utils.ByteUtils;
import java.math.BigDecimal;
import org.apache.hadoop.hbase.util.Bytes;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

/** Ensures compatability with hbase-common Bytes */
@RunWith(JUnit4.class)
public class ByteUtilsTest {

  @Test
  public void testBoolean() {
    assertArrayEquals(Bytes.toBytes(true), ByteUtils.toBytes(true));
    assertArrayEquals(Bytes.toBytes(false), ByteUtils.toBytes(false));
  }

  @Test
  public void testFloat() {
    assertArrayEquals(Bytes.toBytes(0f), ByteUtils.toBytes(0f));
    assertArrayEquals(Bytes.toBytes(1f), ByteUtils.toBytes(1f));
    assertArrayEquals(Bytes.toBytes(-1f), ByteUtils.toBytes(-1f));
    assertArrayEquals(Bytes.toBytes(3.14f), ByteUtils.toBytes(3.14f));
    assertArrayEquals(Bytes.toBytes(314.159f), ByteUtils.toBytes(314.159f));
    assertArrayEquals(Bytes.toBytes(-314.159f), ByteUtils.toBytes(-314.159f));
    assertArrayEquals(Bytes.toBytes(-3.14f), ByteUtils.toBytes(-3.14f));
    assertArrayEquals(Bytes.toBytes(Float.MAX_VALUE), ByteUtils.toBytes(Float.MAX_VALUE));
    assertArrayEquals(Bytes.toBytes(Float.MIN_VALUE), ByteUtils.toBytes(Float.MIN_VALUE));
  }

  @Test
  public void testDouble() {
    assertArrayEquals(Bytes.toBytes(0d), ByteUtils.toBytes(0d));
    assertArrayEquals(Bytes.toBytes(-1d), ByteUtils.toBytes(-1d));
    assertArrayEquals(Bytes.toBytes(1d), ByteUtils.toBytes(1d));
    assertArrayEquals(Bytes.toBytes(3.14d), ByteUtils.toBytes(3.14d));
    assertArrayEquals(Bytes.toBytes(314.159d), ByteUtils.toBytes(314.159d));
    assertArrayEquals(Bytes.toBytes(-314.159d), ByteUtils.toBytes(-314.159d));
    assertArrayEquals(Bytes.toBytes(-3.14d), ByteUtils.toBytes(-3.14d));
    assertArrayEquals(Bytes.toBytes(Double.MAX_VALUE), ByteUtils.toBytes(Double.MAX_VALUE));
    assertArrayEquals(Bytes.toBytes(Double.MIN_VALUE), ByteUtils.toBytes(Double.MIN_VALUE));
  }

  @Test
  public void testInt() {
    assertArrayEquals(Bytes.toBytes(0), ByteUtils.toBytes(0));
    assertArrayEquals(Bytes.toBytes(1), ByteUtils.toBytes(1));
    assertArrayEquals(Bytes.toBytes(-1), ByteUtils.toBytes(-1));
    assertArrayEquals(Bytes.toBytes(400), ByteUtils.toBytes(400));
    assertArrayEquals(Bytes.toBytes(Integer.MAX_VALUE), ByteUtils.toBytes(Integer.MAX_VALUE));
    assertArrayEquals(Bytes.toBytes(Integer.MIN_VALUE), ByteUtils.toBytes(Integer.MIN_VALUE));
  }

  @Test
  public void testLong() {
    assertArrayEquals(Bytes.toBytes(0L), ByteUtils.toBytes(0L));
    assertArrayEquals(Bytes.toBytes(2L), ByteUtils.toBytes(2L));
    assertArrayEquals(Bytes.toBytes(-1L), ByteUtils.toBytes(-1L));
    assertArrayEquals(Bytes.toBytes(-1000L), ByteUtils.toBytes(-1000L));
    assertArrayEquals(Bytes.toBytes(1000L), ByteUtils.toBytes(1000L));
    assertArrayEquals(Bytes.toBytes(Long.MAX_VALUE), ByteUtils.toBytes(Long.MAX_VALUE));
    assertArrayEquals(Bytes.toBytes(Long.MIN_VALUE), ByteUtils.toBytes(Long.MIN_VALUE));
  }

  @Test
  public void testString() {
    assertArrayEquals(Bytes.toBytes(""), ByteUtils.toBytes(""));
    assertArrayEquals(
        Bytes.toBytes("https://www.Google.com"), ByteUtils.toBytes("https://www.Google.com"));
    assertArrayEquals(
        Bytes.toBytes("https://www.google.com"), ByteUtils.toBytes("https://www.google.com"));
    assertArrayEquals(Bytes.toBytes("this is a string"), ByteUtils.toBytes("this is a string"));
  }

  @Test
  public void testShort() {
    assertArrayEquals(Bytes.toBytes((short) 0), ByteUtils.toBytes((short) 0));
    assertArrayEquals(Bytes.toBytes((short) -1), ByteUtils.toBytes((short) -1));
    assertArrayEquals(Bytes.toBytes((short) -100), ByteUtils.toBytes((short) -100));
    assertArrayEquals(Bytes.toBytes((short) 1), ByteUtils.toBytes((short) 1));
    assertArrayEquals(Bytes.toBytes((short) 300), ByteUtils.toBytes((short) 300));
    assertArrayEquals(Bytes.toBytes(Short.MAX_VALUE), ByteUtils.toBytes(Short.MAX_VALUE));
    assertArrayEquals(Bytes.toBytes(Short.MIN_VALUE), ByteUtils.toBytes(Short.MIN_VALUE));
  }

  @Test
  public void testBigDecimal() {
    assertArrayEquals(Bytes.toBytes(BigDecimal.ZERO), ByteUtils.toBytes(BigDecimal.ZERO));
    assertArrayEquals(Bytes.toBytes(BigDecimal.ONE), ByteUtils.toBytes(BigDecimal.ONE));
    assertArrayEquals(
        Bytes.toBytes(BigDecimal.valueOf(-1)), ByteUtils.toBytes(BigDecimal.valueOf(-1)));
    assertArrayEquals(
        Bytes.toBytes(BigDecimal.valueOf(1)), ByteUtils.toBytes(BigDecimal.valueOf(1)));
    assertArrayEquals(
        Bytes.toBytes(BigDecimal.valueOf(-100)), ByteUtils.toBytes(BigDecimal.valueOf(-100)));
    assertArrayEquals(
        Bytes.toBytes(BigDecimal.valueOf(100)), ByteUtils.toBytes(BigDecimal.valueOf(100)));
    assertArrayEquals(
        Bytes.toBytes(BigDecimal.valueOf(100)), ByteUtils.toBytes(BigDecimal.valueOf(100)));
    assertArrayEquals(
        Bytes.toBytes(new BigDecimal("0.300000000000000000000000000000001")),
        ByteUtils.toBytes(new BigDecimal("0.300000000000000000000000000000001")));
  }
}
