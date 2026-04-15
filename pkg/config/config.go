package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for a Spectra node.
type Config struct {
	Node    NodeConfig    `yaml:"node"`
	Storage StorageConfig `yaml:"storage"`
	Meta    MetaConfig    `yaml:"meta"`
	Redis   RedisConfig   `yaml:"redis"`
	WAL     WALConfig     `yaml:"wal"`
	Segment SegmentConfig `yaml:"segment"`
	Index   IndexConfig   `yaml:"index"`
}

// NodeConfig defines per-node settings including roles and listen addresses.
type NodeConfig struct {
	NodeID   string   `yaml:"node_id"`
	Roles    []string `yaml:"roles"`
	GRPCAddr string   `yaml:"grpc_addr"`
	HTTPAddr string   `yaml:"http_addr"`
}

// StorageConfig defines the object storage backend.
type StorageConfig struct {
	Backend    string `yaml:"backend"`     // "fs" or "s3"
	BasePath   string `yaml:"base_path"`   // for fs backend
	S3Bucket   string `yaml:"s3_bucket"`   // for s3 backend
	S3Region   string `yaml:"s3_region"`   // for s3 backend
	S3Endpoint string `yaml:"s3_endpoint"` // for MinIO/custom endpoint
}

// MetaConfig defines the Postgres metadata store connection.
type MetaConfig struct {
	DSN string `yaml:"dsn"`
}

// RedisConfig defines the Redis connection for coordination.
type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// WALConfig defines write-ahead log settings.
type WALConfig struct {
	FlushInterval time.Duration `yaml:"flush_interval"`
	MaxBatchSize  int           `yaml:"max_batch_size"`
}

// SegmentConfig defines segment compaction settings.
type SegmentConfig struct {
	TargetSize      int           `yaml:"target_size"`       // target segment size in bytes
	MaxSpans        int           `yaml:"max_spans"`         // max spans per segment
	CompactInterval time.Duration `yaml:"compact_interval"`  // how often to run compaction
}

// IndexConfig defines full-text indexing settings.
type IndexConfig struct {
	IndexInterval time.Duration `yaml:"index_interval"` // how often to run indexing
	BasePath      string        `yaml:"base_path"`      // base path for index files
}

// Load reads configuration from a YAML file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("spectra config: read %s: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("spectra config: parse %s: %w", path, err)
	}

	return cfg, nil
}

// DefaultConfig returns a Config with sensible defaults for local development.
func DefaultConfig() *Config {
	return &Config{
		Node: NodeConfig{
			NodeID:   "",
			Roles:    []string{"ingest", "compact", "index", "query"},
			GRPCAddr: ":6666",
			HTTPAddr: ":8080",
		},
		Storage: StorageConfig{
			Backend:  "fs",
			BasePath: "/tmp/spectra/data",
		},
		Meta: MetaConfig{
			DSN: "postgres://spectra:spectra@localhost:5432/spectra?sslmode=disable",
		},
		Redis: RedisConfig{
			Addr: "localhost:6379",
			DB:   0,
		},
		WAL: WALConfig{
			FlushInterval: 1 * time.Second,
			MaxBatchSize:  1000,
		},
		Segment: SegmentConfig{
			TargetSize:      50 * 1024 * 1024, // 50MB
			MaxSpans:        100000,
			CompactInterval: 5 * time.Second,
		},
		Index: IndexConfig{
			IndexInterval: 5 * time.Second,
			BasePath:      "/tmp/spectra/index",
		},
	}
}
