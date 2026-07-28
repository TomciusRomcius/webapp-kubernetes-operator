package controller

import (
	"context"
	v1 "webapp-kubernetes-operator/api/v1"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	ctr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

func (r *Reconsiler) Reconsile(ctx context.Context, req ctr.Request) (ctr.Result, error) {
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
