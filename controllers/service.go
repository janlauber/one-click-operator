package controllers

import (
	"context"
	"reflect"

	oneclickiov1alpha1 "github.com/janlauber/one-click-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *RolloutReconciler) reconcileService(ctx context.Context, f *oneclickiov1alpha1.Rollout) error {
	log := log.FromContext(ctx)

	expectedServices := make(map[string]bool)
	for _, intf := range f.Spec.Interfaces {
		serviceName := f.Name + "-" + intf.Name + "-svc"
		expectedServices[serviceName] = true

		service := r.serviceForRollout(f, intf)
		foundService := &corev1.Service{}
		err := r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: f.Namespace}, foundService)
		if err != nil && errors.IsNotFound(err) {
			if err := r.Create(ctx, service); err != nil {
				r.Recorder.Eventf(f, corev1.EventTypeWarning, "CreationFailed", "Failed to create Service %s", serviceName)
				return err
			}
			r.Recorder.Eventf(f, corev1.EventTypeNormal, "Created", "Created Service %s", serviceName)
		} else if err != nil {
			r.Recorder.Eventf(f, corev1.EventTypeWarning, "GetFailed", "Failed to get Service %s", serviceName)
			return err
		} else if !reflect.DeepEqual(foundService.Spec.Ports, getServicePorts(intf)) {
			foundService.Spec.Ports = getServicePorts(intf)
			if err := r.Update(ctx, foundService); err != nil {
				r.Recorder.Eventf(f, corev1.EventTypeWarning, "UpdateFailed", "Failed to update Service %s", serviceName)
				return err
			}
			r.Recorder.Eventf(f, corev1.EventTypeNormal, "Updated", "Updated Service %s", serviceName)
		}
	}

	serviceList := &corev1.ServiceList{}
	listOpts := []client.ListOption{client.InNamespace(f.Namespace)}
	if err := r.List(ctx, serviceList, listOpts...); err != nil {
		log.Error(err, "Failed to list services", "Namespace", f.Namespace)
		return err
	}

	for _, service := range serviceList.Items {
		if _, exists := expectedServices[service.Name]; !exists && service.Labels["one-click.dev/projectId"] == f.Namespace && service.Labels["one-click.dev/deploymentId"] == f.Name {
			if err := r.Delete(ctx, &service); err != nil {
				r.Recorder.Eventf(f, corev1.EventTypeWarning, "DeletionFailed", "Failed to delete Service %s", service.Name)
				return err
			}
			r.Recorder.Eventf(f, corev1.EventTypeNormal, "Deleted", "Deleted Service %s", service.Name)
		}
	}

	return nil
}

func (r *RolloutReconciler) serviceForRollout(f *oneclickiov1alpha1.Rollout, intf oneclickiov1alpha1.InterfaceSpec) *corev1.Service {
	labels := map[string]string{
		"one-click.dev/projectId":    f.Namespace,
		"one-click.dev/deploymentId": f.Name,
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      f.Name + "-" + intf.Name + "-svc",
			Namespace: f.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports:    getServicePorts(intf),
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	ctrl.SetControllerReference(f, svc, r.Scheme)
	return svc
}

func getServicePorts(intf oneclickiov1alpha1.InterfaceSpec) []corev1.ServicePort {
	return []corev1.ServicePort{
		{
			Name:       intf.Name,
			Port:       intf.Port,
			TargetPort: intstr.FromInt(int(intf.Port)),
			Protocol:   corev1.ProtocolTCP,
		},
	}
}
