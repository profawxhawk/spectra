package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/spectra-ai/spectra/pkg/config"
	"github.com/spectra-ai/spectra/pkg/engine"
	"github.com/spectra-ai/spectra/pkg/meta"
	"github.com/spectra-ai/spectra/pkg/storage"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "spectra",
	Short: "Spectra: AI Trace Storage Engine",
	Long:  "Spectra is a production-grade database optimized for AI observability workloads.\nIt provides high write throughput, immediate read consistency, and full-text search across terabytes of trace data.",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Spectra server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Using default configuration: %v\n", err)
			cfg = config.DefaultConfig()
		}

		logger, _ := zap.NewProduction()
		defer logger.Sync()

		ctx := context.Background()

		// Storage backend
		var store storage.ObjectStore
		switch cfg.Storage.Backend {
		case "s3":
			var opts []storage.S3Option
			if cfg.Storage.S3Endpoint != "" {
				opts = append(opts, storage.WithEndpoint(cfg.Storage.S3Endpoint))
			}
			s3Store, err := storage.NewS3(ctx, cfg.Storage.S3Bucket, opts...)
			if err != nil {
				return fmt.Errorf("s3 storage: %w", err)
			}
			store = s3Store
		default:
			fsStore, err := storage.NewFS(cfg.Storage.BasePath)
			if err != nil {
				return fmt.Errorf("fs storage: %w", err)
			}
			store = fsStore
		}

		// Metadata store (Postgres) — optional, skip if unavailable
		var metaStore meta.MetaStore
		pgStore, err := meta.NewPostgresStore(ctx, cfg.Meta.DSN)
		if err != nil {
			logger.Warn("Postgres unavailable, running without metadata store", zap.Error(err))
		} else {
			metaStore = pgStore
		}

		// Create and start engine
		eng := engine.New(cfg, store, metaStore, logger)
		if err := eng.Start(ctx); err != nil {
			return fmt.Errorf("engine start: %w", err)
		}

		logger.Info("Spectra is ready",
			zap.String("http", cfg.Node.HTTPAddr),
			zap.Strings("roles", cfg.Node.Roles),
		)

		// Wait for shutdown signal
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		logger.Info("shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*1e9)
		defer cancel()
		return eng.Stop(shutdownCtx)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("spectra v0.1.0")
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "spectra.yaml", "config file path")
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
