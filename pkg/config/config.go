package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ListenAddr       string
	DatabaseURL      string
	LogLevel         string
	FlushEvery       time.Duration
	Shards           int
	MaxCPU           int
	ReadMaxRangeDays int
	ShutdownWait     time.Duration
}

func Parse() (*Config, error) {
	var errs []error
	c := &Config{}
	c.ListenAddr = getenv("LISTEN_ADDR", ":3000")
	c.DatabaseURL = getenv("DATABASE_URL", "")
	c.LogLevel = getenv("LOG_LEVEL", "info")
	c.FlushEvery = mustDuration(getenv("FLUSH_EVERY", "1s"))
	c.Shards = mustInt(getenv("SHARDS", "64"))
	c.MaxCPU = mustInt(getenv("MAX_CPU", "0"))
	c.ReadMaxRangeDays = mustInt(getenv("READ_MAX_RANGE_DAYS", "90"))
	c.ShutdownWait = mustDuration(getenv("SHUTDOWN_WAIT", "5s"))
	if c.DatabaseURL == "" {
		errs = append(errs, fmt.Errorf("DATABASE_URL is required"))
	}
	if c.Shards <= 0 {
		errs = append(errs, fmt.Errorf("SHARDS must be > 0"))
	}
	if c.ReadMaxRangeDays < 0 {
		errs = append(errs, fmt.Errorf("READ_MAX_RANGE_DAYS must be >= 0"))
	}
	if len(errs) > 0 {
		return nil, joinErrs(errs)
	}
	return c, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
func mustInt(s string) int { n, _ := strconv.Atoi(s); return n }
func mustDuration(s string) time.Duration {
	d, _ := time.ParseDuration(s)
	if d <= 0 {
		return time.Second
	}
	return d
}

func joinErrs(errs []error) error {
	msg := ""
	for i, e := range errs {
		if i > 0 {
			msg += "; "
		}
		msg += e.Error()
	}
	return fmt.Errorf(msg)
}
