package com.otelcontext.vectordb;

import java.util.*;
import java.util.concurrent.locks.ReadWriteLock;
import java.util.concurrent.locks.ReentrantReadWriteLock;

/**
 * In-memory TF-IDF vector index for semantic log search.
 * Only indexes ERROR/WARN logs. FIFO eviction at capacity.
 */
public class TfIdfIndex {

    public record LogVector(long logId, String serviceName, String severity, String body, Map<String, Double> vec) {}
    public record SearchResult(long logId, String serviceName, String severity, String body, double score) {}

    private final ReadWriteLock lock = new ReentrantReadWriteLock();
    private final List<LogVector> docs = new ArrayList<>();
    private Map<String, Double> idf = new HashMap<>();
    private final int maxSize;
    private boolean dirty = false;

    private static final Set<String> STOP_WORDS = Set.of(
        "the", "and", "for", "are", "was", "not", "with", "this", "that", "from",
        "has", "but", "have", "its", "been", "also", "than", "into"
    );

    public TfIdfIndex(int maxSize) {
        this.maxSize = maxSize > 0 ? maxSize : 100_000;
    }

    public void add(long logId, String serviceName, String severity, String body) {
        if (!shouldIndex(severity)) return;
        List<String> tokens = tokenize(body);
        if (tokens.isEmpty()) return;
        Map<String, Double> tf = computeTF(tokens);

        lock.writeLock().lock();
        try {
            if (docs.size() >= maxSize) {
                int removeCount = maxSize / 10;
                List<LogVector> kept = new ArrayList<>(docs.subList(removeCount, docs.size()));
                docs.clear();
                docs.addAll(kept);
                dirty = true;
            }
            docs.add(new LogVector(logId, serviceName, severity, body, tf));
            dirty = true;
        } finally {
            lock.writeLock().unlock();
        }
    }

    public List<SearchResult> search(String query, int k) {
        if (k <= 0) k = 10;
        List<String> tokens = tokenize(query);
        if (tokens.isEmpty()) return List.of();
        Map<String, Double> queryTF = computeTF(tokens);

        Map<String, Double> idfSnap;
        List<LogVector> docsSnap;

        lock.writeLock().lock();
        try {
            if (dirty) {
                recomputeIDF();
                dirty = false;
            }
            idfSnap = new HashMap<>(idf);
            docsSnap = new ArrayList<>(docs);
        } finally {
            lock.writeLock().unlock();
        }

        Map<String, Double> queryVec = new HashMap<>();
        for (var e : queryTF.entrySet()) {
            queryVec.put(e.getKey(), e.getValue() * idfSnap.getOrDefault(e.getKey(), 0.0));
        }
        double queryNorm = vecNorm(queryVec);
        if (queryNorm == 0) return List.of();

        List<SearchResult> results = new ArrayList<>();
        for (var doc : docsSnap) {
            Map<String, Double> docVec = new HashMap<>();
            for (var e : doc.vec().entrySet()) {
                docVec.put(e.getKey(), e.getValue() * idfSnap.getOrDefault(e.getKey(), 0.0));
            }
            double score = cosineSimilarity(queryVec, queryNorm, docVec);
            if (score > 0) {
                results.add(new SearchResult(doc.logId(), doc.serviceName(), doc.severity(), doc.body(), score));
            }
        }

        results.sort((a, b) -> Double.compare(b.score(), a.score()));
        return results.size() > k ? results.subList(0, k) : results;
    }

    public int size() {
        lock.readLock().lock();
        try { return docs.size(); }
        finally { lock.readLock().unlock(); }
    }

    private void recomputeIDF() {
        Map<String, Integer> df = new HashMap<>();
        for (var doc : docs) {
            for (String term : doc.vec().keySet()) {
                df.merge(term, 1, Integer::sum);
            }
        }
        double n = docs.size();
        Map<String, Double> newIdf = new HashMap<>();
        for (var e : df.entrySet()) {
            newIdf.put(e.getKey(), Math.log(n / e.getValue()) + 1);
        }
        this.idf = newIdf;
    }

    static boolean shouldIndex(String severity) {
        if (severity == null) return false;
        String s = severity.toUpperCase();
        return s.equals("ERROR") || s.equals("WARN") || s.equals("WARNING") || s.equals("FATAL") || s.equals("CRITICAL");
    }

    static List<String> tokenize(String text) {
        if (text == null) return List.of();
        String[] words = text.toLowerCase().split("[^a-zA-Z0-9]+");
        List<String> out = new ArrayList<>();
        for (String w : words) {
            if (w.length() > 2 && !STOP_WORDS.contains(w)) {
                out.add(w);
            }
        }
        return out;
    }

    static Map<String, Double> computeTF(List<String> tokens) {
        Map<String, Integer> counts = new HashMap<>();
        for (String t : tokens) counts.merge(t, 1, Integer::sum);
        double total = tokens.size();
        Map<String, Double> tf = new HashMap<>();
        for (var e : counts.entrySet()) tf.put(e.getKey(), e.getValue() / total);
        return tf;
    }

    static double vecNorm(Map<String, Double> v) {
        double sum = 0;
        for (double val : v.values()) sum += val * val;
        return Math.sqrt(sum);
    }

    static double cosineSimilarity(Map<String, Double> a, double normA, Map<String, Double> b) {
        double normB = vecNorm(b);
        if (normA == 0 || normB == 0) return 0;
        double dot = 0;
        for (var e : a.entrySet()) {
            Double vb = b.get(e.getKey());
            if (vb != null) dot += e.getValue() * vb;
        }
        return dot / (normA * normB);
    }
}
