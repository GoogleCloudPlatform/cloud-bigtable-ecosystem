package com.google.cloud.kafka.connect.bigtable.util;

import com.google.bigtable.v2.MutateRowRequest;
import com.google.cloud.bigtable.data.v2.internal.RequestContext;
import com.google.cloud.bigtable.data.v2.models.RowMutation;
import com.google.cloud.bigtable.data.v2.models.TableId;
import com.google.cloud.kafka.connect.bigtable.mapping.ByteUtils;
import com.google.cloud.kafka.connect.bigtable.mapping.MutationData;
import com.google.protobuf.util.JsonFormat;

import java.nio.charset.StandardCharsets;
import java.util.Base64;

public class ProtoUtil {

  public static String toProto(MutationData mutation) {
    try {
      RequestContext context = RequestContext.create("project", "instance", "profile");
      RowMutation rowMutation = RowMutation.create(TableId.of(mutation.getTargetTable()), mutation.getRowKey(), mutation.getInsertMutation());
      MutateRowRequest request = rowMutation.toProto(context);
      return JsonFormat.printer().print(request);
    } catch (Exception e) {
      return "Conversion failed: " + e.getMessage();
    }
  }

  public static String fromBase64(String b64) {
    byte[] decodedBytes = Base64.getDecoder().decode(b64);
    return new String(decodedBytes, StandardCharsets.UTF_8);
  }

  public static String toBase64(int value) {
    byte[] bytes = ByteUtils.toBytes(value);
    return toBase64(bytes);
  }

  public static String toBase64(String value) {
    byte[] bytes = ByteUtils.toBytes(value);
    return toBase64(bytes);
  }

  public static String toBase64(byte[] bytes) {
    byte[] encodedBytes = Base64.getEncoder().encode(bytes);
    return new String(encodedBytes, StandardCharsets.UTF_8);
  }
}
