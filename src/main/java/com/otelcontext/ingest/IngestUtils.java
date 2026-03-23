package com.otelcontext.ingest;

import com.google.protobuf.ByteString;
import io.opentelemetry.proto.common.v1.KeyValue;

import java.util.HexFormat;
import java.util.List;

public final class IngestUtils {

    private IngestUtils() {}

    public static String getServiceName(List<KeyValue> attributes) {
        for (var kv : attributes) {
            if ("service.name".equals(kv.getKey())) {
                return kv.getValue().getStringValue();
            }
        }
        return "unknown-service";
    }

    public static String hexBytes(ByteString bytes) {
        if (bytes == null || bytes.isEmpty()) return "";
        return HexFormat.of().formatHex(bytes.toByteArray());
    }
}
