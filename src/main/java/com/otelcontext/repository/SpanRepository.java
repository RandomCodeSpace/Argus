package com.otelcontext.repository;

import com.otelcontext.model.Span;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.time.Instant;
import java.util.List;

@Repository
public interface SpanRepository extends JpaRepository<Span, Long> {

    List<Span> findByTraceId(String traceId);

    @Query("SELECT s FROM Span s WHERE s.startTime > :since ORDER BY s.startTime ASC")
    List<Span> findRecentSpans(@Param("since") Instant since);

    @Query("SELECT DISTINCT s.serviceName FROM Span s")
    List<String> findDistinctServiceNames();

    long countByTraceId(String traceId);
}
