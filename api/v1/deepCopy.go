package v1

import "k8s.io/apimachinery/pkg/runtime"

func (in *WebApp) DeepCopyInto(out *WebApp) {
	out.TypeMeta = in.TypeMeta
	out.ObjectMeta = in.ObjectMeta
	out.Spec = WebAppSpec{
		Image: in.Spec.Image,
		Ports: in.Spec.Ports,
	}
}

func (a *WebApp) DeepCopyObject() runtime.Object {
	var obj = WebApp{}
	a.DeepCopyInto(&obj)
	return &obj
}

func (a *WebAppList) DeepCopyObject() runtime.Object {
	var copy = WebAppList{}
	copy.TypeMeta = a.TypeMeta
	copy.ListMeta = a.ListMeta
	for i := range a.Items {
		a.Items[i].DeepCopyInto(&copy.Items[i])
	}

	return &copy
}
