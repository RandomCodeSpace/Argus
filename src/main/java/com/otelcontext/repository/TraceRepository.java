package com.otelcontext.repository;

import com.otelcontext.model.Trace;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.time.Instant;
import java.util.List;
import java.util.Optional;

@Repository
public interface TraceRepository extends JpaRepository<Trace, Long> {

    Optional<Trace> findByTraceId(String traceId);

    @Query("SELECT t FROM Trace t WHERE " +
           "(:service IS NULL OR t.serviceName = :service) AND " +
           "(:status IS NULL OR t.status = :status) AND " +
           "t.timestamp BETWEEN :start AND :end")
    Page<Trace> findFiltered(@Param("service") String service,
                             @Param("status") String status,
                             @Param("start") Instant start,
                             @Param("end") Instant end,
                             Pageable pageable);

    @Query("SELECT DISTINCT t.serviceName FROM Trace t")
    List<String> findDistinctServiceNames();

    @Query("SELECT t FROM Trace t WHERE t.timestamp BETWEEN :start AND :end ORDER BY t.timestamp ASC")
    List<Trace> findForArchive(@Param("start") Instant start, @Param("end") Instant end, Pageable pageable);

    @Modifying
    @Query("DELETE FROM Trace t WHERE t.id IN :ids")
    void deleteByIds(@Param("ids") List<Long> ids);

    @Modifying
    @Query("DELETE FROM Trace t WHERE t.timestamp < :cutoff")
    int deleteOlderThan(@Param("cutoff") Instant cutoff);

    @Query("SELECT MIN(t.timestamp) FROM Trace t WHERE t.timestamp < :cutoff")
    Optional<Instant> findOldestTimestamp(@Param("cutoff") Instant cutoff);

    long countByTimestampBetween(Instant start, Instant end);

    @Query("SELECT COUNT(t) FROM Trace t WHERE t.status = 'STATUS_CODE_ERROR' AND t.timestamp BETWEEN :start AND :end")
    long countErrorsBetween(@Param("start") Instant start, @Param("end") Instant end);

    @Query("SELECT AVG(t.duration) FROM Trace t WHERE t.timestamp BETWEEN :start AND :end")
    Optional<Double> avgDurationBetween(@Param("start") Instant start, @Param("end") Instant end);
}
