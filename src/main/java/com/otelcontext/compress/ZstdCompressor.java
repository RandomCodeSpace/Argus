package com.otelcontext.compress;

import java.nio.charset.StandardCharsets;
import java.util.Arrays;

/**
 * Zstd compression utilities using aircompressor (pure Java).
 */
public final class ZstdCompressor {

    private static final byte[] ZSTD_MAGIC = {0x28, (byte) 0xB5, 0x2F, (byte) 0xFD};
    private static final io.airlift.compress.zstd.ZstdCompressor COMPRESSOR = new io.airlift.compress.zstd.ZstdCompressor();
    private static final io.airlift.compress.zstd.ZstdDecompressor DECOMPRESSOR = new io.airlift.compress.zstd.ZstdDecompressor();

    private ZstdCompressor() {}

    public static byte[] compress(byte[] input) {
        if (input == null || input.length == 0) return input;
        int maxLen = COMPRESSOR.maxCompressedLength(input.length);
        byte[] output = new byte[maxLen];
        int compressedSize = COMPRESSOR.compress(input, 0, input.length, output, 0, output.length);
        byte[] result = new byte[ZSTD_MAGIC.length + compressedSize];
        System.arraycopy(ZSTD_MAGIC, 0, result, 0, ZSTD_MAGIC.length);
        System.arraycopy(output, 0, result, ZSTD_MAGIC.length, compressedSize);
        return result;
    }

    public static byte[] decompress(byte[] input) {
        if (input == null || input.length == 0) return input;
        if (!isCompressed(input)) return input;
        byte[] compressed = Arrays.copyOfRange(input, ZSTD_MAGIC.length, input.length);
        long decompressedSize = io.airlift.compress.zstd.ZstdDecompressor.getDecompressedSize(compressed, 0, compressed.length);
        byte[] output = new byte[(int) decompressedSize];
        DECOMPRESSOR.decompress(compressed, 0, compressed.length, output, 0, output.length);
        return output;
    }

    public static boolean isCompressed(byte[] data) {
        if (data == null || data.length < ZSTD_MAGIC.length) return false;
        for (int i = 0; i < ZSTD_MAGIC.length; i++) {
            if (data[i] != ZSTD_MAGIC[i]) return false;
        }
        return true;
    }

    public static String compressString(String input) {
        if (input == null || input.isEmpty()) return input;
        byte[] compressed = compress(input.getBytes(StandardCharsets.UTF_8));
        return new String(compressed, StandardCharsets.ISO_8859_1);
    }

    public static String decompressString(String input) {
        if (input == null || input.isEmpty()) return input;
        byte[] bytes = input.getBytes(StandardCharsets.ISO_8859_1);
        if (!isCompressed(bytes)) return input;
        byte[] decompressed = decompress(bytes);
        return new String(decompressed, StandardCharsets.UTF_8);
    }
}
