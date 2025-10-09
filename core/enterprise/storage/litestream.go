package storage

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/benbjohnson/litestream"
	"github.com/benbjohnson/litestream/s3"
	"github.com/pocketbase/pocketbase/core/enterprise"
)

// LitestreamManager manages Litestream replication for tenant databases
type LitestreamManager struct {
	config   *enterprise.ClusterConfig
	replicas map[string]*replicaState // key: tenantID:dbName
	mu       sync.RWMutex
	logger   *log.Logger
}

type replicaState struct {
	tenantID string
	dbName   string
	db       *litestream.DB
	replica  *litestream.Replica
	cancel   context.CancelFunc
}

// NewLitestreamManager creates a new Litestream manager
func NewLitestreamManager(config *enterprise.ClusterConfig) *LitestreamManager {
	return &LitestreamManager{
		config:   config,
		replicas: make(map[string]*replicaState),
		logger:   log.Default(),
	}
}

// StartReplication starts Litestream replication for a tenant database
func (m *LitestreamManager) StartReplication(tenantID string, dbPath string, dbName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := tenantID + ":" + dbName

	// Check if already replicating
	if _, exists := m.replicas[key]; exists {
		m.logger.Printf("[Litestream] Already replicating %s/%s", tenantID, dbName)
		return nil
	}

	m.logger.Printf("[Litestream] Starting replication for tenant %s, database %s at %s", tenantID, dbName, dbPath)

	// Create Litestream DB
	db := litestream.NewDB(dbPath)

	// Configure checkpoint settings
	db.MinCheckpointPageN = 1000   // Checkpoint after 1000 pages
	db.MaxCheckpointPageN = 10000  // Force checkpoint after 10000 pages
	db.CheckpointInterval = 1 * time.Minute
	db.MonitorInterval = 1 * time.Second

	// Set up logging (suppress verbose litestream logs)
	db.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Only show warnings and errors
	}))

	// Create S3 replica client
	s3Client := s3.NewReplicaClient()
	s3Client.AccessKeyID = m.config.S3AccessKeyID
	s3Client.SecretAccessKey = m.config.S3SecretAccessKey
	s3Client.Region = m.config.S3Region
	s3Client.Bucket = m.config.S3Bucket
	s3Client.Path = fmt.Sprintf("tenants/%s/litestream/%s", tenantID, dbName)

	if m.config.S3Endpoint != "" {
		s3Client.Endpoint = m.config.S3Endpoint
		s3Client.ForcePathStyle = true // Required for MinIO/LocalStack
	}

	// Initialize S3 client
	ctx := context.Background()
	if err := s3Client.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	// Create replica
	replica := litestream.NewReplicaWithClient(db, s3Client)

	// Configure sync interval
	if m.config.LitestreamReplicateSync {
		replica.SyncInterval = 1 * time.Second // Sync every second (safer but slower)
	} else {
		replica.SyncInterval = 10 * time.Second // Default interval
	}

	replica.MonitorEnabled = true

	// Attach replica to database
	db.Replica = replica

	// Open the database (starts background replication)
	if err := db.Open(); err != nil {
		return fmt.Errorf("failed to open litestream db: %w", err)
	}

	// Start the replica monitoring
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := replica.Start(ctx); err != nil && ctx.Err() == nil {
			m.logger.Printf("[Litestream] Replica error for %s/%s: %v", tenantID, dbName, err)
		}
	}()

	// Store state
	m.replicas[key] = &replicaState{
		tenantID: tenantID,
		dbName:   dbName,
		db:       db,
		replica:  replica,
		cancel:   cancel,
	}

	m.logger.Printf("[Litestream] Successfully started replication for %s/%s to s3://%s/%s",
		tenantID, dbName, m.config.S3Bucket, s3Client.Path)

	return nil
}

// StopReplication stops Litestream replication for a tenant database
func (m *LitestreamManager) StopReplication(tenantID string, dbName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := tenantID + ":" + dbName

	state, exists := m.replicas[key]
	if !exists {
		return nil // Not replicating
	}

	m.logger.Printf("[Litestream] Stopping replication for tenant %s, database %s", tenantID, dbName)

	// Stop the replica monitoring (soft stop to complete pending syncs)
	if state.replica != nil {
		if err := state.replica.Stop(false); err != nil {
			m.logger.Printf("[Litestream] Error stopping replica: %v", err)
		}
	}

	// Cancel context
	if state.cancel != nil {
		state.cancel()
	}

	// Final sync to ensure all data is replicated
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if state.replica != nil {
		if err := state.replica.Sync(ctx); err != nil {
			m.logger.Printf("[Litestream] Error during final sync: %v", err)
		}
	}

	// Close database
	if state.db != nil {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer closeCancel()

		if err := state.db.Close(closeCtx); err != nil {
			m.logger.Printf("[Litestream] Error closing db: %v", err)
		}
	}

	// Remove from map
	delete(m.replicas, key)

	m.logger.Printf("[Litestream] Stopped replication for tenant %s, database %s", tenantID, dbName)
	return nil
}

// StopAllReplications stops all active replications
func (m *LitestreamManager) StopAllReplications() error {
	m.mu.Lock()
	keys := make([]string, 0, len(m.replicas))
	for key := range m.replicas {
		keys = append(keys, key)
	}
	m.mu.Unlock()

	// Stop each replication (unlock while stopping to avoid deadlock)
	for _, key := range keys {
		m.mu.RLock()
		state := m.replicas[key]
		m.mu.RUnlock()

		if state != nil {
			// Extract tenant ID and dbName from key
			// Stop without holding lock
			if err := m.StopReplication(state.tenantID, state.dbName); err != nil {
				m.logger.Printf("[Litestream] Error stopping %s: %v", key, err)
			}
		}
	}

	m.logger.Printf("[Litestream] Stopped all replications")
	return nil
}

// RestoreDatabase restores a database from S3 using Litestream
func (m *LitestreamManager) RestoreDatabase(ctx context.Context, tenantID, dbName, destPath string) error {
	m.logger.Printf("[Litestream] Restoring database %s/%s to %s", tenantID, dbName, destPath)

	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create S3 replica client (same config as replication)
	s3Client := s3.NewReplicaClient()
	s3Client.AccessKeyID = m.config.S3AccessKeyID
	s3Client.SecretAccessKey = m.config.S3SecretAccessKey
	s3Client.Region = m.config.S3Region
	s3Client.Bucket = m.config.S3Bucket
	s3Client.Path = fmt.Sprintf("tenants/%s/litestream/%s", tenantID, dbName)

	if m.config.S3Endpoint != "" {
		s3Client.Endpoint = m.config.S3Endpoint
		s3Client.ForcePathStyle = true
	}

	// Initialize S3 client
	if err := s3Client.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	// Create a temporary DB for restore
	db := litestream.NewDB(destPath)
	replica := litestream.NewReplicaWithClient(db, s3Client)

	// Configure restore options
	opt := litestream.NewRestoreOptions()
	// Restore to latest point in time
	opt.Timestamp = time.Now()
	opt.Parallelism = 4 // Parallel restore for speed

	// Perform restore
	if err := replica.Restore(ctx, opt); err != nil {
		// Check if error is "no snapshots" which means this is a new database
		if err == litestream.ErrNoSnapshots {
			m.logger.Printf("[Litestream] No snapshots found for %s/%s (new database)", tenantID, dbName)
			// Create empty database file
			if _, err := os.Create(destPath); err != nil {
				return fmt.Errorf("failed to create empty database: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to restore from S3: %w", err)
	}

	m.logger.Printf("[Litestream] Successfully restored %s/%s from S3", tenantID, dbName)
	return nil
}

// GetReplicationStats returns replication statistics for a tenant database
func (m *LitestreamManager) GetReplicationStats(tenantID string, dbName string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := tenantID + ":" + dbName

	state, exists := m.replicas[key]
	if !exists {
		return nil, fmt.Errorf("no replication for %s/%s", tenantID, dbName)
	}

	stats := make(map[string]interface{})

	if state.replica != nil {
		// Get current replica position
		pos := state.replica.Pos()
		stats["txID"] = pos.TXID
		stats["postApplyChecksum"] = pos.PostApplyChecksum

		// Get time bounds
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if createdAt, updatedAt, err := state.replica.TimeBounds(ctx); err == nil {
			stats["createdAt"] = createdAt
			stats["updatedAt"] = updatedAt
			stats["lagSeconds"] = time.Since(updatedAt).Seconds()
		}
	}

	if state.db != nil {
		stats["dbPath"] = state.db.Path()
		stats["pageSize"] = state.db.PageSize()
	}

	stats["tenantId"] = tenantID
	stats["dbName"] = dbName
	stats["status"] = "active"

	return stats, nil
}

// SyncNow forces an immediate sync for a tenant database
func (m *LitestreamManager) SyncNow(ctx context.Context, tenantID string, dbName string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := tenantID + ":" + dbName

	state, exists := m.replicas[key]
	if !exists {
		return fmt.Errorf("no replication for %s/%s", tenantID, dbName)
	}

	if state.replica == nil {
		return fmt.Errorf("replica not initialized for %s/%s", tenantID, dbName)
	}

	m.logger.Printf("[Litestream] Forcing sync for %s/%s", tenantID, dbName)

	if err := state.replica.Sync(ctx); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	m.logger.Printf("[Litestream] Sync completed for %s/%s", tenantID, dbName)
	return nil
}

// IsReplicating checks if a database is currently being replicated
func (m *LitestreamManager) IsReplicating(tenantID string, dbName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := tenantID + ":" + dbName
	_, exists := m.replicas[key]
	return exists
}
