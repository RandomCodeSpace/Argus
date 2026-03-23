package com.otelcontext.repository;

import com.otelcontext.model.MetricBucket;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.time.Instant;
import java.util.List;

@Repository
public interface MetricBucketRepository extends JpaRepository<MetricBucket, Long> {

    @Query("SELECT m FROM MetricBucket m WHERE " +
           "(:name IS NULL OR m.name = :name) AND " +
           "(:service IS NULL OR m.serviceName = :service) AND " +
           "m.timeBucket BETWEEN :start AND :end ORDER BY m.timeBucket ASC")
    List<MetricBucket> findFiltered(@Param("name") String name,
                                    @Param("service") String service,
                                    @Param("start") Instant start,
                                    @Param("end") Instant end);

    @Query("SELECT DISTINCT m.name FROM MetricBucket m")
    List<String> findDistinctNames();

    @Modifying
    @Query("DELETE FROM MetricBucket m WHERE m.timeBucket < :cutoff")
    int deleteOlderThan(@Param("cutoff") Instant cutoff);

    @Modifying
    @Query("DELETE FROM MetricBucket m WHERE m.id IN :ids")
    void deleteByIds(@Param("ids") List<Long> ids);
}
