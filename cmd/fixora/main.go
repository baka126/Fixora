package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"

	"fixora/pkg/config"
	"fixora/pkg/controller"
	"fixora/pkg/server"
)

func initLogger() {
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") != "" {
		switch strings.ToUpper(os.Getenv("LOG_LEVEL")) {
		case "DEBUG":
			level = slog.LevelDebug
		case "WARN":
			level = slog.LevelWarn
		case "ERROR":
			level = slog.LevelError
		}
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func main() {
	initLogger()

	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	cfg := config.Load()
	slog.Info("Starting Fixora", "mode", cfg.Mode, "log_level", os.Getenv("LOG_LEVEL"))

	var k8sConfig *rest.Config
	var err error

	// Try in-cluster config first if no kubeconfig is explicitly provided OR if the file doesn't exist
	if *kubeconfig == "" {
		k8sConfig, err = rest.InClusterConfig()
		if err != nil {
			slog.Error("Error building in-cluster config", "error", err)
			os.Exit(1)
		}
	} else {
		// If kubeconfig is provided, check if it exists
		if _, statErr := os.Stat(*kubeconfig); os.IsNotExist(statErr) {
			// If provided but doesn't exist, fallback to in-cluster
			slog.Info("Kubeconfig not found at path, falling back to in-cluster config", "path", *kubeconfig)
			k8sConfig, err = rest.InClusterConfig()
			if err != nil {
				slog.Error("Error building in-cluster config (fallback)", "error", err)
				os.Exit(1)
			}
		} else {
			k8sConfig, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
			if err != nil {
				slog.Error("Error building kubeconfig from flags", "path", *kubeconfig, "error", err)
				os.Exit(1)
			}
		}
	}

	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		slog.Error("Error creating clientset", "error", err)
		os.Exit(1)
	}

	dynamicClient, err := dynamic.NewForConfig(k8sConfig)
	if err != nil {
		slog.Error("Error creating dynamic client", "error", err)
		os.Exit(1)
	}

	metricsClient, err := metricsclientset.NewForConfig(k8sConfig)
	if err != nil {
		slog.Error("Error creating metrics client", "error", err)
		os.Exit(1)
	}

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(signalCtx)
	defer cancel()

	stopCh := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(stopCh)
	}()

	ctrl := controller.NewController(clientset, dynamicClient, metricsClient, cfg)
	srv := server.New(ctrl, cfg)

	slog.Info("Initialization complete, starting services")
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.Start(ctx)
	}()

	go ctrl.Run(stopCh)

	select {
	case err := <-serverErr:
		if err != nil {
			slog.Error("Server failed", "error", err)
			cancel()
			os.Exit(1)
		}
		cancel()
	case <-ctx.Done():
		cancel()
		if err := <-serverErr; err != nil {
			slog.Error("Server shutdown failed", "error", err)
		}
	}

	slog.Info("Shutting down Fixora")
}
