package com.otelcontext.api;

import com.otelcontext.model.LogEntry;
import com.otelcontext.repository.LogRepository;
import com.otelcontext.vectordb.TfIdfIndex;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Sort;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.time.Instant;
import java.util.Map;

@RestController
@RequestMapping("/api/logs")
public class LogController {

    private final LogRepository logRepo;
    private final TfIdfIndex vectorIndex;

    public LogController(LogRepository logRepo, TfIdfIndex vectorIndex) {
        this.logRepo = logRepo;
        this.vectorIndex = vectorIndex;
    }

    @GetMapping
    public Map<String, Object> getLogs(
            @RequestParam(required = false) String service_name,
            @RequestParam(required = false) String severity,
            @RequestParam(required = false) String search,
            @RequestParam(required = false) String start,
            @RequestParam(required = false) String end,
            @RequestParam(defaultValue = "50") int limit,
            @RequestParam(defaultValue = "0") int offset) {

        Instant startTime = start != null ? Instant.parse(start) : null;
        Instant endTime = end != null ? Instant.parse(end) : null;

        var page = logRepo.findFiltered(
            service_name, severity, null, search, startTime, endTime,
            PageRequest.of(offset / Math.max(limit, 1), Math.max(limit, 1), Sort.by(Sort.Direction.DESC, "timestamp"))
        );

        return Map.of("data", page.getContent(), "total", page.getTotalElements());
    }

    @GetMapping("/context")
    public ResponseEntity<?> getLogContext(@RequestParam String timestamp) {
        Instant ts = Instant.parse(timestamp);
        var logs = logRepo.findByTimestampBetween(ts.minusSeconds(60), ts.plusSeconds(60));
        return ResponseEntity.ok(logs);
    }

    @GetMapping("/similar")
    public ResponseEntity<?> getSimilarLogs(@RequestParam String query, @RequestParam(defaultValue = "10") int limit) {
        return ResponseEntity.ok(vectorIndex.search(query, limit));
    }

    @GetMapping("/{id}/insight")
    public ResponseEntity<?> getLogInsight(@PathVariable Long id) {
        return logRepo.findById(id)
            .map(l -> ResponseEntity.ok(Map.of("insight", l.getAiInsight() != null ? l.getAiInsight() : "")))
            .orElse(ResponseEntity.notFound().build());
    }
}
