package engine

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/spectra-ai/spectra/pkg/config"
	"github.com/spectra-ai/spectra/pkg/index"
	"github.com/spectra-ai/spectra/pkg/memindex"
	"github.com/spectra-ai/spectra/pkg/meta"
	"github.com/spectra-ai/spectra/pkg/segment"
	"github.com/spectra-ai/spectra/pkg/server"
	"github.com/spectra-ai/spectra/pkg/storage"
	"github.com/spectra-ai/spectra/pkg/wal"
)

// Engine is the top-level orchestrator that wires all Spectra components together.
type Engine struct {
	cfg       *config.Config
	store     storage.ObjectStore
	metaStore meta.MetaStore
	walWriter *wal.Writer
	walReader *wal.Reader
	memIdx    *memindex.MemIndex
	compactor *segment.Compactor
	indexer   *index.Indexer
	segReader *segment.Reader
	http      *server.HTTPServer
	logger    *zap.Logger
	cancel    context.CancelFunc
}

// New creates a new Engine with the given configuration.
// For production, pass a real MetaStore. For dev/testing, pass nil and it will skip metadata features.
func New(cfg *config.Config, store storage.ObjectStore, metaStore meta.MetaStore, logger *zap.Logger) *Engine {
	nodeID := cfg.Node.NodeID
	if nodeID == "" {
		nodeID = fmt.Sprintf("node-%d", time.Now().UnixNano()%10000)
	}

	walWriter := wal.NewWriter(store, nodeID, cfg.WAL.FlushInterval, cfg.WAL.MaxBatchSize, logger)
	walReader := wal.NewReader(store, logger)
	memIdx := memindex.New(10 * time.Minute)
	segReader := segment.NewReader(store, logger)

	builder := segment.NewBuilder(cfg.Segment.TargetSize, cfg.Segment.MaxSpans, logger)
	compactor := segment.NewCompactor(store, metaStore, walReader, builder, cfg.Segment.CompactInterval, logger)
	indexer := index.NewIndexer(metaStore, segReader, store, cfg.Index.IndexInterval, logger)

	httpServer := server.NewHTTPServer(cfg.Node.HTTPAddr, walWriter, memIdx, walReader, metaStore, segReader, indexer, logger)

	return &Engine{
		cfg:       cfg,
		store:     store,
		metaStore: metaStore,
		walWriter: walWriter,
		walReader: walReader,
		memIdx:    memIdx,
		compactor: compactor,
		indexer:   indexer,
		segReader: segReader,
		http:      httpServer,
		logger:    logger,
	}
}

func (e *Engine) hasRole(role string) bool {
	for _, r := range e.cfg.Node.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// Start starts all components based on configured roles.
func (e *Engine) Start(ctx context.Context) error {
	ctx, e.cancel = context.WithCancel(ctx)

	e.logger.Info("starting Spectra engine",
		zap.Strings("roles", e.cfg.Node.Roles),
		zap.String("storage", e.cfg.Storage.Backend),
	)

	// Run migrations if meta store is available
	if e.metaStore != nil {
		if err := e.metaStore.Migrate(ctx); err != nil {
			return fmt.Errorf("engine: migrate: %w", err)
		}
	}

	// Start role-based workers
	if e.hasRole("ingest") {
		go e.walWriter.Start(ctx)
		go e.memIdx.StartEviction(ctx, 30*time.Second)
		e.logger.Info("ingest role started")
	}

	if e.hasRole("compact") && e.metaStore != nil {
		go e.compactor.Start(ctx)
		e.logger.Info("compact role started")
	}

	if e.hasRole("index") && e.metaStore != nil {
		go e.indexer.Start(ctx)
		e.logger.Info("index role started")
	}

	if e.hasRole("query") || e.hasRole("ingest") {
		go func() {
			if err := e.http.Start(); err != nil {
				e.logger.Error("HTTP server failed", zap.Error(err))
			}
		}()
		e.logger.Info("HTTP server started", zap.String("addr", e.cfg.Node.HTTPAddr))
	}

	return nil
}

// Stop gracefully shuts down all components.
func (e *Engine) Stop(ctx context.Context) error {
	e.logger.Info("stopping Spectra engine")

	if e.cancel != nil {
		e.cancel()
	}

	// Final WAL flush
	if err := e.walWriter.Stop(ctx); err != nil {
		e.logger.Warn("WAL flush on shutdown failed", zap.Error(err))
	}

	// Stop HTTP server
	if err := e.http.Stop(ctx); err != nil {
		e.logger.Warn("HTTP server shutdown failed", zap.Error(err))
	}

	// Close metadata store
	if e.metaStore != nil {
		e.metaStore.Close()
	}

	e.logger.Info("Spectra engine stopped")
	return nil
}
