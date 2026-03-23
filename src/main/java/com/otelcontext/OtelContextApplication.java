package com.otelcontext;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

@SpringBootApplication
@EnableScheduling
public class OtelContextApplication {

    public static void main(String[] args) {
        System.setProperty("user.timezone", "UTC");
        System.out.println("""
                  ___ _____ _____ _\s
                 / _ \\_   _| ____| |   \s
                | | | || | |  _| | |   \s
                | |_| || | | |___| |___\s
                 \\___/ |_| |_____|_____|

                  OtelContext Java Edition
                """);
        SpringApplication.run(OtelContextApplication.class, args);
    }
}
