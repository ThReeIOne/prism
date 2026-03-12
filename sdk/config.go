package sdk

import "time"

// Config holds the Tracer configuration.
type Config struct {
	CollectorAddr string
	BatchSize     int
	FlushInterval time.Duration
	Sampler       Sampler
	IngestToken   string
}

func defaultConfig() *Config {
	return &Config{
		CollectorAddr: "localhost:24317",
		BatchSize:     1024,
		FlushInterval: 5 * time.Second,
		Sampler:       AlwaysSampler{},
	}
}

// Option configures the Tracer.
type Option func(*Config)

// WithCollector sets the collector gRPC address.
func WithCollector(addr string) Option {
	return func(c *Config) {
		c.CollectorAddr = addr
	}
}

// WithBatchSize sets the batch export size.
func WithBatchSize(n int) Option {
	return func(c *Config) {
		c.BatchSize = n
	}
}

// WithFlushInterval sets the batch flush interval.
func WithFlushInterval(d time.Duration) Option {
	return func(c *Config) {
		c.FlushInterval = d
	}
}

// WithSampler sets the sampling strategy.
func WithSampler(s Sampler) Option {
	return func(c *Config) {
		c.Sampler = s
	}
}

// WithIngestToken sets the bearer token sent in the Authorization metadata
// header on every gRPC Report call. If empty, no Authorization header is sent.
func WithIngestToken(token string) Option {
	return func(c *Config) {
		c.IngestToken = token
	}
}
