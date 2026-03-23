package com.otelcontext.repository;

import com.otelcontext.model.GraphSnapshot;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.time.Instant;
import java.util.Optional;

@Repository
public interface GraphSnapshotRepository extends JpaRepository<GraphSnapshot, String> {

    @Query("SELECT g FROM GraphSnapshot g WHERE g.createdAt <= :at ORDER BY g.createdAt DESC LIMIT 1")
    Optional<GraphSnapshot> findClosestBefore(@Param("at") Instant at);

    @Modifying
    @Query("DELETE FROM GraphSnapshot g WHERE g.createdAt < :cutoff")
    int deleteOlderThan(@Param("cutoff") Instant cutoff);
}
