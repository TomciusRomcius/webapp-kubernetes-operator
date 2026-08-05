package controller

import (
	"context"
	"fmt"
	"strings"

	operatorv1 "webapp-kubernetes-operator/api/v1"
	"webapp-kubernetes-operator/internal/helm"

	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	traefikReleaseName = "traefik"
	traefikNamespace   = "traefik"
	traefikChartName   = "traefik"
	traefikRepoURL     = "https://traefik.github.io/charts"
)

// WebAppReconciler reconciles a WebApp object
type WebAppReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	HelmClient helm.HelmClient
}

func (r *WebAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Reconsiling")
	var webApp operatorv1.WebApp
	r.tryTraefikCleanup(ctx, r.Client)
	if err := r.Get(ctx, req.NamespacedName, &webApp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	err := r.ensureTraefik(ctx, r.Client)
	if err != nil {
		logger.Error(err, "failed to install or upgrade Traefik")
		setErrorStatus(webApp, err)
		return ctrl.Result{}, err
	}

	if err = r.ensureIngressRoute(ctx, webApp); err != nil {
		logger.Error(err, "failed to create or update the IngressRoute")
		setErrorStatus(webApp, err)
		return ctrl.Result{}, err
	}

	err = r.ensureDeployment(ctx, req, webApp)
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

func (r *WebAppReconciler) tryTraefikCleanup(ctx context.Context, client client.Client) error {
	logger := logf.FromContext(ctx)
	webApps := operatorv1.WebAppList{}
	err := client.List(ctx, &webApps)
	if err != nil {
		return err
	}
	numOfWebApps := len(webApps.Items)
	if numOfWebApps == 0 {
		if err := r.HelmClient.UninstallChart(ctx, traefikReleaseName, traefikNamespace); err != nil {
			return err
		}
		traefikCrds := strings.Split(Crds, "\n")
		for _, crd := range traefikCrds {
			crd = strings.TrimSpace(crd)
			if crd == "" {
				continue
			}
			u := unstructured.Unstructured{}
			u.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "apiextensions.k8s.io",
				Version: "v1",
				Kind:    "CustomResourceDefinition",
			})
			u.SetName(crd)
			if err := client.Delete(ctx, &u); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				logger.Error(err, "failed to delete CRD", "crd", crd)
				return err
			}
			logger.Info("Deleted CRD", "crd", crd)
		}
		return nil
	}
	logger.Info("Retrieved number of web apps", "Webapps", numOfWebApps)
	return nil
}

func (r *WebAppReconciler) ensureTraefik(ctx context.Context, client client.Client) error {
	logger := logf.FromContext(ctx)
	_, err := r.HelmClient.InstallOrUpgradeChart(&helm.ChartActionConfig{
		ReleaseName: traefikReleaseName,
		Namespace:   traefikNamespace,
		ChartName:   traefikChartName,
		RepoURL:     traefikRepoURL,
	}, map[string]any{})
	if err != nil {
		return err
	}
	logger.Info("Ensured Traefik chart", "release", traefikReleaseName, "namespace", traefikNamespace)
	return nil
}

func (r *WebAppReconciler) ensureIngressRoute(ctx context.Context, webApp operatorv1.WebApp) error {
	logger := logf.FromContext(ctx)
	logger.Info("Ensuring IngressRoute")
	ir := unstructured.Unstructured{}
	ir.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "traefik.io",
		Version: "v1alpha1",
		Kind:    "IngressRoute",
	})
	ir.SetName(fmt.Sprintf("%s-ingressroute", webApp.Name))
	ir.SetNamespace(webApp.Namespace)
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, &ir, func() error {
		ir.Object["spec"] = map[string]any{
			// TODO: make this configurable
			"ingressClassName": "traefik",
			"routes": []map[string]any{
				{
					"kind": "Rule",
					// TODO: make this configurable
					"match": fmt.Sprintf("Host(`%s`)", strings.ToLower(webApp.Name)+".com"),
					"services": []map[string]any{
						{
							"kind":      "Service",
							"namespace": webApp.Namespace,
							"name":      webApp.Name,
							"port":      webApp.Spec.Ports[0].External,
						},
					},
				},
			},
		}
		return nil
	})
	if err != nil {
		logger.Error(err, "failed to create or update IngressRoute")
		return err
	}

	controllerutil.SetControllerReference(&webApp, &ir, r.Scheme)
	logger.Info("IngressRoute ensured")
	return nil
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

	controllerutil.SetControllerReference(&webApp, &service, r.Scheme)
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
