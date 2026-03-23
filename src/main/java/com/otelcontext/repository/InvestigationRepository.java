package com.otelcontext.repository;

import com.otelcontext.model.Investigation;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public interface InvestigationRepository extends JpaRepository<Investigation, String> {

    @Query("SELECT i FROM Investigation i WHERE " +
           "(:service IS NULL OR i.triggerService = :service OR i.rootService = :service) AND " +
           "(:severity IS NULL OR i.severity = :severity) AND " +
           "(:status IS NULL OR i.status = :status) " +
           "ORDER BY i.createdAt DESC")
    List<Investigation> findFiltered(@Param("service") String service,
                                     @Param("severity") String severity,
                                     @Param("status") String status,
                                     Pageable pageable);
}
