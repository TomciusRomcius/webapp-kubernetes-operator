/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	operatorv1 "webapp-kubernetes-operator/api/v1"

	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// WebAppReconciler reconciles a WebApp object
type WebAppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=operator.operator.com,resources=webapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.operator.com,resources=webapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.operator.com,resources=webapps/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the WebApp object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/reconcile
func (r *WebAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Reconsiling")
	var webApp operatorv1.WebApp
	if err := r.Get(ctx, req.NamespacedName, &webApp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	err := r.ensureDeployment(ctx, req, webApp)
	if err != nil {
		logger.Error(err, "failed to create or update a deployment")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *WebAppReconciler) ensureDeployment(ctx context.Context, req ctrl.Request, webApp operatorv1.WebApp) error {
	deployment := appsv1.Deployment{}
	deployment.Name = req.Name
	deployment.Namespace = req.Namespace

	deploymentSpec := r.buildDeploymentSpec(&webApp)
	if err := controllerutil.SetControllerReference(&webApp, &deployment, r.Scheme); err != nil {
		return err
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, &deployment, func() error {
		deployment.Spec = *deploymentSpec
		return nil
	})

	return err
}

func (r *WebAppReconciler) buildDeploymentSpec(webApp *operatorv1.WebApp) *appsv1.DeploymentSpec {
	spec := appsv1.DeploymentSpec{
		Replicas: new(int32(1)),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": webApp.Name,
			},
		},
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": webApp.Name,
				},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:            webApp.Name,
						Image:           webApp.Spec.Image,
						ImagePullPolicy: v1.PullAlways,
						Ports: lo.Map(webApp.Spec.Ports, func(port operatorv1.WebAppPortMapping, _ int) v1.ContainerPort {
							return v1.ContainerPort{
								HostPort:      int32(port.External),
								ContainerPort: int32(port.Internal),
							}
						}),
					},
				},
			},
		},
	}
	return &spec
}

// SetupWithManager sets up the controller with the Manager.
func (r *WebAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.WebApp{}).
		Named("webapp").
		Complete(r)
}
