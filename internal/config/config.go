package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Address            string
	StaticDir          string
	DebugAssetsDir     string
	CentroidDir        string
	PersistDir         string
	Debug              bool
	DMDataEnabled      bool
	DMDataRefreshToken string
	DMDataClientID     string
	DMDataClientSecret string
	DMDataAppName      string
	KMoniURL           string
	KMoniInterval      time.Duration
	HTTPTimeout        time.Duration
}

func FromEnv() Config {
	port := value("PORT", "9090")
	return Config{
		Address:            value("ADDRESS", ":"+port),
		StaticDir:          value("STATIC_DIR", "static"),
		DebugAssetsDir:     value("DEBUG_ASSETS_DIR", "test/assets"),
		CentroidDir:        value("CENTROID_DIR", "assets/centroid"),
		PersistDir:         value("PERSIST_DIR", "persist"),
		Debug:              boolean("DEBUG", os.Getenv("ENV") == "testing"),
		DMDataEnabled:      boolean("DMDATA_ENABLED", true),
		DMDataRefreshToken: value("DMDATA_REFRESH_TOKEN", ""),
		DMDataClientID:     value("DMDATA_CLIENT_ID", "CId.pB6FnDGAvAMHgSsFqOHl9qujFK_WQovheSG4j8BbR1dT"),
		DMDataClientSecret: value("DMDATA_CLIENT_SECRET", "CSt.DPOwMxjZREbrylr5GDWx1B-Xh6BHI7_OKfan82ls5zIG"),
		DMDataAppName:      value("DMDATA_APP_NAME", "JQuake-1.8.5"),
		KMoniURL:           value("KMONI_SHAKE_LEVEL_URL", "https://kwatch-24h.net/EQLevel.json"),
		KMoniInterval:      duration("KMONI_INTERVAL", 10*time.Second),
		HTTPTimeout:        duration("HTTP_TIMEOUT", 15*time.Second),
	}
}

func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func boolean(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	return err == nil && b
}

func duration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
