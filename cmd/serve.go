package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/pocketbase/pocketbase/apis"
	enterpriseapis "github.com/pocketbase/pocketbase/apis/enterprise"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/core/enterprise"
	"github.com/pocketbase/pocketbase/core/enterprise/control_plane"
	"github.com/pocketbase/pocketbase/core/enterprise/gateway"
	"github.com/pocketbase/pocketbase/core/enterprise/storage"
	"github.com/pocketbase/pocketbase/core/enterprise/tenant_node"
	"github.com/spf13/cobra"
)

// NewServeCommand creates and returns new command responsible for
// starting the default PocketBase web server.
func NewServeCommand(app core.App, showStartBanner bool) *cobra.Command {
	var allowedOrigins []string
	var httpAddr string
	var httpsAddr string

	// Enterprise mode flags
	var mode string
	var nodeID string
	var nodeAddress string
	var raftPeers []string
	var raftBindAddr string
	var controlPlaneAddrs []string
	var maxTenants int
	var s3Endpoint string
	var s3Region string
	var s3Bucket string
	var s3AccessKeyID string
	var s3SecretAccessKey string

	command := &cobra.Command{
		Use:          "serve [domain(s)]",
		Args:         cobra.ArbitraryArgs,
		Short:        "Starts the web server (default to 127.0.0.1:8090 if no domain is specified)",
		SilenceUsage: true,
		RunE: func(command *cobra.Command, args []string) error {
			// Check if running in enterprise mode
			if mode != "" && mode != "standard" {
				return runEnterpriseMode(mode, nodeID, nodeAddress, raftPeers, raftBindAddr, controlPlaneAddrs, maxTenants,
					s3Endpoint, s3Region, s3Bucket, s3AccessKeyID, s3SecretAccessKey, app)
			}

			// Standard PocketBase mode (existing behavior)
			// set default listener addresses if at least one domain is specified
			if len(args) > 0 {
				if httpAddr == "" {
					httpAddr = "0.0.0.0:80"
				}
				if httpsAddr == "" {
					httpsAddr = "0.0.0.0:443"
				}
			} else {
				if httpAddr == "" {
					httpAddr = "127.0.0.1:8090"
				}
			}

			err := apis.Serve(app, apis.ServeConfig{
				HttpAddr:           httpAddr,
				HttpsAddr:          httpsAddr,
				ShowStartBanner:    showStartBanner,
				AllowedOrigins:     allowedOrigins,
				CertificateDomains: args,
			})

			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}

			return err
		},
	}

	command.PersistentFlags().StringSliceVar(
		&allowedOrigins,
		"origins",
		[]string{"*"},
		"CORS allowed domain origins list",
	)

	command.PersistentFlags().StringVar(
		&httpAddr,
		"http",
		"",
		"TCP address to listen for the HTTP server\n(if domain args are specified - default to 0.0.0.0:80, otherwise - default to 127.0.0.1:8090)",
	)

	command.PersistentFlags().StringVar(
		&httpsAddr,
		"https",
		"",
		"TCP address to listen for the HTTPS server\n(if domain args are specified - default to 0.0.0.0:443, otherwise - default to empty string, aka. no TLS)\nThe incoming HTTP traffic also will be auto redirected to the HTTPS version",
	)

	// Enterprise mode flags
	command.PersistentFlags().StringVar(
		&mode,
		"mode",
		"",
		"Enterprise mode: control-plane, tenant-node, gateway, all-in-one (leave empty for standard mode)",
	)

	command.PersistentFlags().StringVar(
		&nodeID,
		"node-id",
		"",
		"Node ID for control-plane mode (e.g., cp-1)",
	)

	command.PersistentFlags().StringVar(
		&nodeAddress,
		"node-addr",
		"",
		"This node's advertised address for tenant-node mode (e.g., node1.internal:8091, defaults to localhost:8091)",
	)

	command.PersistentFlags().StringSliceVar(
		&raftPeers,
		"raft-peers",
		[]string{},
		"Raft peer addresses for control-plane mode (e.g., cp-1:7000,cp-2:7000,cp-3:7000)",
	)

	command.PersistentFlags().StringVar(
		&raftBindAddr,
		"raft-bind",
		"127.0.0.1:7000",
		"Raft bind address for control-plane mode",
	)

	command.PersistentFlags().StringSliceVar(
		&controlPlaneAddrs,
		"control-plane",
		[]string{},
		"Control plane addresses for tenant-node/gateway modes (e.g., cp-1:8090,cp-2:8090)",
	)

	command.PersistentFlags().IntVar(
		&maxTenants,
		"max-tenants",
		200,
		"Maximum number of tenants for tenant-node mode",
	)

	command.PersistentFlags().StringVar(
		&s3Endpoint,
		"s3-endpoint",
		"",
		"S3 endpoint URL (leave empty for AWS S3)",
	)

	command.PersistentFlags().StringVar(
		&s3Region,
		"s3-region",
		"us-east-1",
		"S3 region",
	)

	command.PersistentFlags().StringVar(
		&s3Bucket,
		"s3-bucket",
		"",
		"S3 bucket name for tenant data",
	)

	command.PersistentFlags().StringVar(
		&s3AccessKeyID,
		"s3-access-key",
		"",
		"S3 access key ID (or set AWS_ACCESS_KEY_ID env var)",
	)

	command.PersistentFlags().StringVar(
		&s3SecretAccessKey,
		"s3-secret-key",
		"",
		"S3 secret access key (or set AWS_SECRET_ACCESS_KEY env var)",
	)

	return command
}

// runEnterpriseMode starts PocketBase in enterprise mode
func runEnterpriseMode(mode, nodeID, nodeAddress string, raftPeers []string, raftBindAddr string,
	controlPlaneAddrs []string, maxTenants int, s3Endpoint, s3Region, s3Bucket,
	s3AccessKeyID, s3SecretAccessKey string, app core.App) error {

	log.Printf("Starting PocketBase Enterprise in %s mode", mode)

	// Parse mode
	enterpriseMode, err := enterprise.ParseMode(mode)
	if err != nil {
		return fmt.Errorf("invalid mode: %w", err)
	}

	// Get S3 credentials from env if not provided
	if s3AccessKeyID == "" {
		s3AccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
	}
	if s3SecretAccessKey == "" {
		s3SecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}

	// Get JWT secret from environment variable
	jwtSecret := os.Getenv("POCKETBASE_JWT_SECRET")

	// Build enterprise config
	config := &enterprise.ClusterConfig{
		Mode:         enterpriseMode,
		NodeID:       nodeID,
		RaftPeers:    raftPeers,
		RaftBindAddr: raftBindAddr,
		DataDir:      app.DataDir(),

		ControlPlaneAddrs:        controlPlaneAddrs,
		MaxTenants:              maxTenants,
		NodeAddress:             nodeAddress,
		GatewayControlPlaneAddrs: controlPlaneAddrs,

		S3Endpoint:        s3Endpoint,
		S3Region:          s3Region,
		S3Bucket:          s3Bucket,
		S3AccessKeyID:     s3AccessKeyID,
		S3SecretAccessKey: s3SecretAccessKey,

		LitestreamEnabled:   true,
		LitestreamRetention: "72h",

		JWTSecret: jwtSecret,
	}

	// Validate config based on mode
	if err := validateEnterpriseConfig(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Start the appropriate service based on mode
	switch enterpriseMode {
	case enterprise.ModeControlPlane:
		return runControlPlane(config)

	case enterprise.ModeTenantNode:
		return runTenantNode(config)

	case enterprise.ModeGateway:
		return runGateway(config)

	case enterprise.ModeAllInOne:
		return runAllInOne(config)

	default:
		return fmt.Errorf("unknown mode: %s", mode)
	}
}

// runControlPlane starts the control plane service
func runControlPlane(config *enterprise.ClusterConfig) error {
	log.Printf("[ControlPlane] Starting control plane node: %s", config.NodeID)

	// Create control plane
	cp, err := control_plane.NewControlPlane(config)
	if err != nil {
		return fmt.Errorf("failed to create control plane: %w", err)
	}

	// Start control plane
	if err := cp.Start(); err != nil {
		return fmt.Errorf("failed to start control plane: %w", err)
	}

	// Create and start HTTP API server
	router := enterpriseapis.NewRouter(cp, config.JWTSecret)
	if err := cp.StartHTTP(":8095", router); err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Printf("[ControlPlane] Control plane running.")
	log.Printf("[ControlPlane] - IPC server: :8090")
	log.Printf("[ControlPlane] - HTTP API: :8095")
	log.Printf("[ControlPlane] Press Ctrl+C to stop.")
	<-sigChan

	log.Printf("[ControlPlane] Shutting down...")
	return cp.Stop()
}

// runTenantNode starts the tenant node service
func runTenantNode(config *enterprise.ClusterConfig) error {
	log.Printf("[TenantNode] Starting tenant node")

	ctx := context.Background()

	// Create S3 storage backend
	s3Backend, err := storage.NewS3Backend(ctx, config.S3Endpoint, config.S3Region, config.S3Bucket, config.S3AccessKeyID, config.S3SecretAccessKey)
	if err != nil {
		return fmt.Errorf("failed to create S3 backend: %w", err)
	}

	// Create control plane client
	cpClient, err := tenant_node.NewControlPlaneClient(config.ControlPlaneAddrs)
	if err != nil {
		return fmt.Errorf("failed to create control plane client: %w", err)
	}
	defer cpClient.Close()

	// Create tenant manager
	manager, err := tenant_node.NewManager(config, s3Backend, cpClient)
	if err != nil {
		return fmt.Errorf("failed to create tenant manager: %w", err)
	}

	// Start tenant manager
	if err := manager.Start(); err != nil {
		return fmt.Errorf("failed to start tenant manager: %w", err)
	}

	// TODO: Start HTTP server to handle tenant requests

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Printf("[TenantNode] Tenant node running. Press Ctrl+C to stop.")
	<-sigChan

	log.Printf("[TenantNode] Shutting down...")
	return manager.Stop()
}

// runGateway starts the gateway service
func runGateway(config *enterprise.ClusterConfig) error {
	log.Printf("[Gateway] Starting gateway")

	// Create control plane client
	cpClient, err := tenant_node.NewControlPlaneClient(config.GatewayControlPlaneAddrs)
	if err != nil {
		return fmt.Errorf("failed to create control plane client: %w", err)
	}
	defer cpClient.Close()

	// Create gateway
	gw, err := gateway.NewGateway(config, cpClient)
	if err != nil {
		return fmt.Errorf("failed to create gateway: %w", err)
	}

	// Start gateway in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- gw.Start("0.0.0.0:8080")
	}()

	// Wait for shutdown signal or error
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Printf("[Gateway] Gateway running on :8080. Press Ctrl+C to stop.")

	select {
	case err := <-errChan:
		return fmt.Errorf("gateway error: %w", err)
	case <-sigChan:
		log.Printf("[Gateway] Shutting down...")
		return gw.Stop()
	}
}

// runAllInOne starts all services in a single process
func runAllInOne(config *enterprise.ClusterConfig) error {
	log.Printf("[AllInOne] Starting all-in-one mode")

	ctx := context.Background()

	// 1. Start control plane
	cp, err := control_plane.NewControlPlane(config)
	if err != nil {
		return fmt.Errorf("failed to create control plane: %w", err)
	}

	if err := cp.Start(); err != nil {
		return fmt.Errorf("failed to start control plane: %w", err)
	}
	defer cp.Stop()

	// 2. Start tenant node
	s3Backend, err := storage.NewS3Backend(ctx, config.S3Endpoint, config.S3Region, config.S3Bucket, config.S3AccessKeyID, config.S3SecretAccessKey)
	if err != nil {
		return fmt.Errorf("failed to create S3 backend: %w", err)
	}

	// Use localhost for control plane in all-in-one mode
	cpClient, err := tenant_node.NewControlPlaneClient([]string{"localhost:8090"})
	if err != nil {
		return fmt.Errorf("failed to create control plane client: %w", err)
	}
	defer cpClient.Close()

	manager, err := tenant_node.NewManager(config, s3Backend, cpClient)
	if err != nil {
		return fmt.Errorf("failed to create tenant manager: %w", err)
	}

	if err := manager.Start(); err != nil {
		return fmt.Errorf("failed to start tenant manager: %w", err)
	}
	defer manager.Stop()

	// 3. Start gateway
	gw, err := gateway.NewGateway(config, cpClient)
	if err != nil {
		return fmt.Errorf("failed to create gateway: %w", err)
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- gw.Start("0.0.0.0:8080")
	}()

	// Wait for shutdown signal or error
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Printf("[AllInOne] All services running. Gateway on :8080. Press Ctrl+C to stop.")

	select {
	case err := <-errChan:
		return fmt.Errorf("gateway error: %w", err)
	case <-sigChan:
		log.Printf("[AllInOne] Shutting down...")
		gw.Stop()
		return nil
	}
}

// validateEnterpriseConfig validates the enterprise configuration
func validateEnterpriseConfig(config *enterprise.ClusterConfig) error {
	switch config.Mode {
	case enterprise.ModeControlPlane:
		if config.NodeID == "" {
			return fmt.Errorf("--node-id required for control-plane mode")
		}
		if len(config.RaftPeers) == 0 {
			return fmt.Errorf("--raft-peers required for control-plane mode")
		}
		if config.S3Bucket == "" {
			return fmt.Errorf("--s3-bucket required")
		}

	case enterprise.ModeTenantNode:
		if len(config.ControlPlaneAddrs) == 0 {
			return fmt.Errorf("--control-plane required for tenant-node mode")
		}
		if config.S3Bucket == "" {
			return fmt.Errorf("--s3-bucket required")
		}

	case enterprise.ModeGateway:
		if len(config.GatewayControlPlaneAddrs) == 0 {
			return fmt.Errorf("--control-plane required for gateway mode")
		}

	case enterprise.ModeAllInOne:
		if config.S3Bucket == "" {
			return fmt.Errorf("--s3-bucket required")
		}
	}

	return nil
}
