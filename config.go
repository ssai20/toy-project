package main

import "os"

type Config struct {
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	DBSSLMode   string
	AppPort     string
	Env         string
	KafkaBroker string
	KafkaCert   string
	KafkaKey    string
	KafkaCA     string
}

func loadConfig() *Config {
	return &Config{
		DBHost:      getEnv("DB_HOST", "localhost"),
		DBPort:      getEnv("DB_PORT", "5432"),
		DBUser:      getEnv("DB_USER", "myuser"),
		DBPassword:  getEnv("DB_PASSWORD", "mypassword"),
		DBName:      getEnv("DB_NAME", "mydatabase"),
		DBSSLMode:   getEnv("DB_SSLMODE", "disable"),
		AppPort:     getEnv("APP_PORT", "8086"),
		Env:         getEnv("ENV", "development"),
		KafkaBroker: getEnv("KAFKA_BROKER", "kafka-23ff71ac-ssairan20-5f0c.i.aivencloud.com:16742"),
		KafkaCert:   getEnv("KAFKA_SERT", "/tmp/access-cert.pem"),
		KafkaKey:    getEnv("KAFKA_KEY", "/tmp/access-key.pem"),
		KafkaCA:     getEnv("KAFKA_CA", "/tmp/ca_cert.pem"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
