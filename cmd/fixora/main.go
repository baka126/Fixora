package main

import (
	"flag"
	"log"
	"path/filepath"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
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

	k8sConfig, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %s", err.Error())
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
