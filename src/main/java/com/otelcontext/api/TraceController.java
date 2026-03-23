package com.otelcontext.api;

import com.otelcontext.model.Trace;
import com.otelcontext.repository.SpanRepository;
import com.otelcontext.repository.TraceRepository;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Sort;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.time.Instant;
import java.util.Map;

@RestController
@RequestMapping("/api/traces")
public class TraceController {

    private final TraceRepository traceRepo;
    private final SpanRepository spanRepo;

    public TraceController(TraceRepository traceRepo, SpanRepository spanRepo) {
        this.traceRepo = traceRepo;
        this.spanRepo = spanRepo;
    }

    @GetMapping
    public Map<String, Object> getTraces(
            @RequestParam(required = false) String service,
            @RequestParam(required = false) String status,
            @RequestParam(required = false) String start,
            @RequestParam(required = false) String end,
            @RequestParam(defaultValue = "20") int limit,
            @RequestParam(defaultValue = "0") int offset) {

        Instant startTime = start != null ? Instant.parse(start) : Instant.now().minusSeconds(3600);
        Instant endTime = end != null ? Instant.parse(end) : Instant.now();

        var page = traceRepo.findFiltered(service, status, startTime, endTime,
            PageRequest.of(offset / Math.max(limit, 1), Math.max(limit, 1), Sort.by(Sort.Direction.DESC, "timestamp")));

        return Map.of("data", page.getContent(), "total", page.getTotalElements());
    }

    @GetMapping("/{id}")
    public ResponseEntity<?> getTraceById(@PathVariable String id) {
        return traceRepo.findByTraceId(id)
            .map(trace -> {
                var spans = spanRepo.findByTraceId(id);
                trace.setSpans(spans);
                return ResponseEntity.ok(trace);
            })
            .orElse(ResponseEntity.notFound().build());
    }
}
