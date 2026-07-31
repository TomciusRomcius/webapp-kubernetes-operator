package controller

import (
	"context"

	operatorv1 "webapp-kubernetes-operator/api/v1"

	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
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

// +kubebuilder:rbac:groups=operator.com,resources=webapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.com,resources=webapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.com,resources=webapps/finalizers,verbs=update

func (r *WebAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Reconsiling")
	var webApp operatorv1.WebApp
	if err := r.Get(ctx, req.NamespacedName, &webApp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	err := r.ensureDeployment(ctx, req, webApp)
	if err != nil {
		logger.Error(err, "failed to create or update the deployment")
		setErrorStatus(webApp, err)
		return ctrl.Result{}, err
	}

	err = r.ensureService(ctx, webApp)
	if err != nil {
		logger.Error(err, "failed to create or update the service")
		setErrorStatus(webApp, err)
		return ctrl.Result{}, err
	}

	meta.SetStatusCondition(&webApp.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "ReconsileSuccess",
		Message: "WebApp is ready",
	})

	return ctrl.Result{}, nil
}

func setErrorStatus(webApp operatorv1.WebApp, err error) {
	meta.SetStatusCondition(&webApp.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionFalse,
		Reason:  "DeploymentFailed",
		Message: err.Error(),
	})
}

func (r *WebAppReconciler) ensureService(ctx context.Context, webApp operatorv1.WebApp) error {
	service := v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      webApp.Name,
			Namespace: webApp.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, &service, func() error {
		service.Spec = *buildServiceSpec(webApp)
		return nil
	})

	return err
}

func buildServiceSpec(webApp operatorv1.WebApp) *v1.ServiceSpec {
	return &v1.ServiceSpec{
		Selector: map[string]string{
			"app": webApp.Name,
		},
		// TODO: names for ports
		Ports: lo.Map(webApp.Spec.Ports, func(port operatorv1.WebAppPortMapping, _ int) v1.ServicePort {
			return v1.ServicePort{
				Port:       int32(port.External),
				TargetPort: intstr.FromInt32(port.Internal),
			}
		}),
	}
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
