package com.otelcontext.repository;

import com.otelcontext.model.LogEntry;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.time.Instant;
import java.util.List;

@Repository
public interface LogRepository extends JpaRepository<LogEntry, Long> {

    @Query("SELECT l FROM LogEntry l WHERE " +
           "(:service IS NULL OR l.serviceName = :service) AND " +
           "(:severity IS NULL OR l.severity = :severity) AND " +
           "(:traceId IS NULL OR l.traceId = :traceId) AND " +
           "(:search IS NULL OR l.body LIKE CONCAT('%', :search, '%') OR l.traceId LIKE CONCAT('%', :search, '%')) AND " +
           "(:start IS NULL OR l.timestamp >= :start) AND " +
           "(:end IS NULL OR l.timestamp <= :end)")
    Page<LogEntry> findFiltered(@Param("service") String service,
                                @Param("severity") String severity,
                                @Param("traceId") String traceId,
                                @Param("search") String search,
                                @Param("start") Instant start,
                                @Param("end") Instant end,
                                Pageable pageable);

    @Query("SELECT l FROM LogEntry l WHERE l.timestamp BETWEEN :start AND :end ORDER BY l.timestamp ASC")
    List<LogEntry> findByTimestampBetween(@Param("start") Instant start, @Param("end") Instant end);

    @Modifying
    @Query("DELETE FROM LogEntry l WHERE l.timestamp < :cutoff")
    int deleteOlderThan(@Param("cutoff") Instant cutoff);

    @Modifying
    @Query("DELETE FROM LogEntry l WHERE l.id IN :ids")
    void deleteByIds(@Param("ids") List<Long> ids);

    @Query("SELECT DISTINCT l.serviceName FROM LogEntry l")
    List<String> findDistinctServiceNames();

    long countByTimestampBetween(Instant start, Instant end);
}
