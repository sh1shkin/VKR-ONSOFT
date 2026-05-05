package config

import "os"

type Config struct {
    Addr              string
    DatabaseURL       string
    JWTSecret         string
    LLMServiceURL     string
    ClientOrigin      string
    EnableDemoSeed    bool
    IntegrationAPIKey string
}

func Load() Config {
    return Config{
        Addr:              getEnv("APP_ADDR", ":8080"),
        DatabaseURL:       getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/tender_recommendation?sslmode=disable"),
        JWTSecret:         getEnv("APP_JWT_SECRET", "dev-secret-change-me"),
        LLMServiceURL:     getEnv("LLM_SERVICE_URL", ""),
        ClientOrigin:      getEnv("CLIENT_ORIGIN", "http://localhost:5173"),
        EnableDemoSeed:    getEnv("APP_ENABLE_DEMO_SEED", "true") == "true",
        IntegrationAPIKey: getEnv("INTEGRATION_API_KEY", ""),
    }
}

func getEnv(key, fallback string) string {
    if val := os.Getenv(key); val != "" {
        return val
    }
    return fallback
}
