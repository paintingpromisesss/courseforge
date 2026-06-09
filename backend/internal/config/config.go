package config

import "os"

type Config struct {
	CoursesDir string
	Addr       string
}

func Load() *Config {
	return &Config{
		CoursesDir: getenv("COURSEFORGE_COURSES_DIR", "./courses"),
		Addr:       getenv("COURSEFORGE_ADDR", ":8080"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
