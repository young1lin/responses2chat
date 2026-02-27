package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/young1lin/responses2chat/internal/config"
	"github.com/young1lin/responses2chat/internal/handler"
	"github.com/young1lin/responses2chat/pkg/logger"
)

var (
	Version   = "dev"
	BuildDate = "unknown"
)

var (
	cfgFile string
	port    int
	showVer  bool
)

var rootCmd = &cobra.Command{
	Use:   "responses2chat",
	Short: "Responses API to Chat Completions API proxy",
	Long: `A proxy server that converts OpenAI Responses API requests
to Chat Completions API format, enabling Codex to work with
third-party LLM providers like DeepSeek, Zhipu, Qwen, etc.`,
	Run: func(cmd *cobra.Command, args []string) {
		if showVer {
			fmt.Printf("responses2chat %s (built %s)\n", Version, BuildDate)
			return
		}

		cfg := config.Load(cfgFile)

		// Override config with command line flags
		if port > 0 {
			cfg.Server.Port = port
		}

		// Initialize logger
		logger.Init(cfg.Logging.Level, cfg.Logging.Format)
		defer logger.Sync()

		logger.Info("starting server",
			zap.String("version", Version),
			zap.String("host", cfg.Server.Host),
			zap.Int("port", cfg.Server.Port),
		)

		startServer(cfg)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "./config.yaml", "config file path")
	rootCmd.PersistentFlags().IntVarP(&port, "port", "p", 0, "listen port (overrides config)")
	rootCmd.Flags().BoolVarP(&showVer, "version", "v", false, "show version")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func startServer(cfg *config.Config) {
	// Create handler
	proxyHandler := handler.NewProxyHandler(cfg)

	// Create server
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      proxyHandler,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", zap.Error(err))
			os.Exit(1)
		}
	}()

	// Print startup info
	fmt.Printf(`
╔═══════════════════════════════════════════════════════════╗
║                 responses2chat %s                      ║
╠═══════════════════════════════════════════════════════════╣
║  Server: http://%s:%d                            ║
║  Health: http://%s:%d/health                      ║
║  Target: %s                         ║
╚═══════════════════════════════════════════════════════════╝

`, Version, cfg.Server.Host, cfg.Server.Port, cfg.Server.Host, cfg.Server.Port, cfg.DefaultTarget.BaseURL)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", zap.Error(err))
	}

	logger.Info("server stopped")
}
