package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/spectra-ai/spectra/pkg/config"
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
		fmt.Printf("Starting Spectra server (node=%s) on gRPC=%s HTTP=%s\n",
			cfg.Node.NodeID, cfg.Node.GRPCAddr, cfg.Node.HTTPAddr)
		fmt.Printf("Roles: %v\n", cfg.Node.Roles)
		fmt.Printf("Storage backend: %s\n", cfg.Storage.Backend)

		// TODO: wire engine and start serving
		select {}
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
