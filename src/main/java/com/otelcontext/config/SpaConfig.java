package com.otelcontext.config;

import org.springframework.context.annotation.Configuration;
import org.springframework.core.io.ClassPathResource;
import org.springframework.core.io.Resource;
import org.springframework.web.servlet.config.annotation.ResourceHandlerRegistry;
import org.springframework.web.servlet.config.annotation.WebMvcConfigurer;
import org.springframework.web.servlet.resource.PathResourceResolver;

import java.io.IOException;

/**
 * Serves the React SPA from static resources.
 * Non-API, non-WS routes fall back to index.html for client-side routing.
 */
@Configuration
public class SpaConfig implements WebMvcConfigurer {

    @Override
    public void addResourceHandlers(ResourceHandlerRegistry registry) {
        registry.addResourceHandler("/**")
            .addResourceLocations("classpath:/static/")
            .resourceChain(true)
            .addResolver(new PathResourceResolver() {
                @Override
                protected Resource getResource(String resourcePath, Resource location) throws IOException {
                    Resource requestedResource = location.createRelative(resourcePath);
                    if (requestedResource.exists() && requestedResource.isReadable()) {
                        return requestedResource;
                    }
                    // SPA fallback: serve index.html for non-file routes
                    if (!resourcePath.startsWith("api/") && !resourcePath.startsWith("v1/")
                        && !resourcePath.startsWith("ws") && !resourcePath.startsWith("mcp")
                        && !resourcePath.startsWith("metrics/")) {
                        Resource indexHtml = new ClassPathResource("/static/index.html");
                        if (indexHtml.exists()) return indexHtml;
                    }
                    return null;
                }
            });
    }
}
