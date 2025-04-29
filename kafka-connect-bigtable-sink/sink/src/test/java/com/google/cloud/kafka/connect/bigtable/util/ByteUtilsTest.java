package com.google.cloud.kafka.connect.bigtable.util;

import static org.junit.Assert.assertArrayEquals;

import com.google.cloud.kafka.connect.bigtable.mapping.ByteUtils;
import java.math.BigDecimal;
import org.apache.hadoop.hbase.util.Bytes;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

/**
 * Ensures compatability with hbase-common Bytes
 */
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
    assertArrayEquals(Bytes.toBytes(3.14f), ByteUtils.toBytes(3.14f));
    assertArrayEquals(Bytes.toBytes(-3.14f), ByteUtils.toBytes(-3.14f));
    assertArrayEquals(Bytes.toBytes(Float.MAX_VALUE), ByteUtils.toBytes(Float.MAX_VALUE));
    assertArrayEquals(Bytes.toBytes(Float.MIN_VALUE), ByteUtils.toBytes(Float.MIN_VALUE));
  }

  @Test
  public void testDouble() {
    assertArrayEquals(Bytes.toBytes(0d), ByteUtils.toBytes(0d));
    assertArrayEquals(Bytes.toBytes(Double.MAX_VALUE), ByteUtils.toBytes(Double.MAX_VALUE));
    assertArrayEquals(Bytes.toBytes(Double.MIN_VALUE), ByteUtils.toBytes(Double.MIN_VALUE));
  }

  @Test
  public void testInt() {
    assertArrayEquals(Bytes.toBytes(0), ByteUtils.toBytes(0));
    assertArrayEquals(Bytes.toBytes(Integer.MAX_VALUE), ByteUtils.toBytes(Integer.MAX_VALUE));
    assertArrayEquals(Bytes.toBytes(Integer.MIN_VALUE), ByteUtils.toBytes(Integer.MIN_VALUE));
  }

  @Test
  public void testLong() {
    assertArrayEquals(Bytes.toBytes(0L), ByteUtils.toBytes(0L));
    assertArrayEquals(Bytes.toBytes(Long.MAX_VALUE), ByteUtils.toBytes(Long.MAX_VALUE));
    assertArrayEquals(Bytes.toBytes(Long.MIN_VALUE), ByteUtils.toBytes(Long.MIN_VALUE));
  }

  @Test
  public void testString() {
    assertArrayEquals(Bytes.toBytes(""), ByteUtils.toBytes(""));
    assertArrayEquals(Bytes.toBytes("this is a string"), ByteUtils.toBytes("this is a string"));
  }

  @Test
  public void testShort() {
    assertArrayEquals(Bytes.toBytes((short) 0), ByteUtils.toBytes((short) 0));
    assertArrayEquals(Bytes.toBytes(Short.MAX_VALUE), ByteUtils.toBytes(Short.MAX_VALUE));
    assertArrayEquals(Bytes.toBytes(Short.MIN_VALUE), ByteUtils.toBytes(Short.MIN_VALUE));
  }

  @Test
  public void testBigDecimal() {
    assertArrayEquals(Bytes.toBytes(BigDecimal.ZERO), ByteUtils.toBytes(BigDecimal.ZERO));
    assertArrayEquals(
        Bytes.toBytes(new BigDecimal("0.300000000000000000000000000000001")),
        ByteUtils.toBytes(new BigDecimal("0.300000000000000000000000000000001"))
    );
  }

}
