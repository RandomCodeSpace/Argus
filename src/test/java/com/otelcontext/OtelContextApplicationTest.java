package com.otelcontext;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.web.servlet.MockMvc;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

@SpringBootTest
@AutoConfigureMockMvc
class OtelContextApplicationTest {

    @Autowired
    private MockMvc mockMvc;

    @Test
    void contextLoads() {
        // Application context loads successfully
    }

    @Test
    void healthEndpoint() throws Exception {
        mockMvc.perform(get("/api/health"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.status").value("ok"));
    }

    @Test
    void statsEndpoint() throws Exception {
        mockMvc.perform(get("/api/stats"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.traces").exists())
            .andExpect(jsonPath("$.logs").exists());
    }

    @Test
    void servicesEndpoint() throws Exception {
        mockMvc.perform(get("/api/metadata/services"))
            .andExpect(status().isOk());
    }

    @Test
    void logsEndpoint() throws Exception {
        mockMvc.perform(get("/api/logs"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.data").isArray())
            .andExpect(jsonPath("$.total").isNumber());
    }

    @Test
    void tracesEndpoint() throws Exception {
        mockMvc.perform(get("/api/traces"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.data").isArray());
    }
}
