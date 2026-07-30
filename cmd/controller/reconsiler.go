package controller

import (
	"context" // Provides functionality for managing and passing context, especially useful in request-scoped operations.
	"errors"
	"os"
	"path/filepath"
	v1 "webapp-kubernetes-operator/api/v1"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime" // Handles runtime types and schemes for Kubernetes objects.
	"k8s.io/client-go/kubernetes"                      // Kubernetes client-go library for interacting with the Kubernetes API server.
	"k8s.io/client-go/tools/clientcmd"                 // Handles loading and parsing kubeconfig files for out-of-cluster Kubernetes access.
	"k8s.io/client-go/util/homedir"
	ctr "sigs.k8s.io/controller-runtime"        // Main package for building controllers using the Kubernetes Controller Runtime.
	"sigs.k8s.io/controller-runtime/pkg/client" // Provides a dynamic client for interacting with Kubernetes objects.
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var scheme = runtime.NewScheme()
var setupLog = ctr.Log.WithName("setup")

func init() {
	utilruntime.Must(v1.SchemeBuilder.AddToScheme(scheme))
}

type Reconsiler struct {
	client.Client
	scheme     *runtime.Scheme
	kubeClient *kubernetes.Clientset
}

func (r *Reconsiler) Reconcile(ctx context.Context, req ctr.Request) (ctr.Result, error) {
	var log = log.FromContext(ctx).WithValues("application", req.NamespacedName)
	log.Info("Reconsiling application")

	var application v1.WebApp
	var err = r.Client.Get(ctx, req.NamespacedName, &application)
	if err != nil {
		log.Error(err, "No resource")
	}

	log.Info("Yes")
	return ctr.Result{}, nil
}

func RunController() {
	ctr.SetLogger(zap.New())
	setupLog.Info("Initializing")

	kubeconfigFilePath := filepath.Join(homedir.HomeDir(), ".kube", "config")
	if _, err := os.Stat(kubeconfigFilePath); errors.Is(err, os.ErrNotExist) {
		setupLog.Error(err, "Failed to start up the manager")
		os.Exit(1)
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigFilePath)
	if err != nil {
		setupLog.Error(err, "Failed to start up the manager")
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(config)

	mngr, err := ctr.NewManager(config, manager.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "Failed to start up the manager")
		os.Exit(1)
	}

	err = ctr.NewControllerManagedBy(mngr).
		For(&v1.WebApp{}).
		Complete(&Reconsiler{
			Client:     mngr.GetClient(),
			scheme:     mngr.GetScheme(),
			kubeClient: clientset,
		})

	if err != nil {
		setupLog.Error(err, "Failed to start up the manager")
		os.Exit(1)
	}

	setupLog.Info("Starting the manager")
	if err := mngr.Start(ctr.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Failed to start up the manager")
		os.Exit(1)
	}
}
