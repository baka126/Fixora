package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

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

func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	cfg := config.Load()

	var k8sConfig *rest.Config
	var err error

	// Try in-cluster config first if no kubeconfig is explicitly provided OR if the file doesn't exist
	if *kubeconfig == "" {
		k8sConfig, err = rest.InClusterConfig()
		if err != nil {
			log.Fatalf("Error building in-cluster config: %s", err.Error())
		}
	} else {
		// If kubeconfig is provided, check if it exists
		if _, statErr := os.Stat(*kubeconfig); os.IsNotExist(statErr) {
			// If provided but doesn't exist, fallback to in-cluster
			k8sConfig, err = rest.InClusterConfig()
			if err != nil {
				log.Fatalf("Error building in-cluster config (fallback): %s", err.Error())
			}
		} else {
			k8sConfig, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
			if err != nil {
				log.Fatalf("Error building kubeconfig from flags: %s", err.Error())
			}
		}
	}

	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		log.Fatalf("Error creating clientset: %s", err.Error())
	}

	dynamicClient, err := dynamic.NewForConfig(k8sConfig)
	if err != nil {
		log.Fatalf("Error creating dynamic client: %s", err.Error())
	}

	metricsClient, err := metricsclientset.NewForConfig(k8sConfig)
	if err != nil {
		log.Fatalf("Error creating metrics client: %s", err.Error())
	}

	stopCh := make(chan struct{})
	defer close(stopCh)

	ctrl := controller.NewController(clientset, dynamicClient, metricsClient, cfg)
	srv := server.New(ctrl, cfg)
	go srv.Start()

	go ctrl.Run(stopCh)

	<-stopCh
}
