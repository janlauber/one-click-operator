package controllers

import (
	"context"

	oneclickiov1alpha1 "github.com/janlauber/one-click-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *RolloutReconciler) reconcilePVCs(ctx context.Context, f *oneclickiov1alpha1.Rollout) error {
	log := log.FromContext(ctx)

	if len(f.Spec.Volumes) == 0 {
		return r.deleteAllPVCsForRollout(ctx, f)
	}

	expectedPVCs := make(map[string]struct{})
	for _, volSpec := range f.Spec.Volumes {
		pvcName := volSpec.Name + "-" + f.Name
		expectedPVCs[pvcName] = struct{}{}

		desiredPVC := r.constructPVCForRollout(f, volSpec)
		foundPVC := &corev1.PersistentVolumeClaim{}
		err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: f.Namespace}, foundPVC)
		if err != nil && errors.IsNotFound(err) {
			if err := r.Create(ctx, desiredPVC); err != nil {
				r.Recorder.Eventf(f, corev1.EventTypeWarning, "CreationFailed", "Failed to create PVC %s", pvcName)
				return err
			}
			r.Recorder.Eventf(f, corev1.EventTypeNormal, "Created", "Created PVC %s", pvcName)
		} else if err != nil {
			r.Recorder.Eventf(f, corev1.EventTypeWarning, "GetFailed", "Failed to get PVC %s", pvcName)
			return err
		} else if currentSize, desiredSize := foundPVC.Spec.Resources.Requests[corev1.ResourceStorage], resource.MustParse(volSpec.Size); desiredSize.Cmp(currentSize) > 0 {
			if foundPVC.Spec.VolumeMode == nil || *foundPVC.Spec.VolumeMode != corev1.PersistentVolumeFilesystem {
				log.Info("PVC resizing is only supported for filesystem volume mode")
				return nil
			}

			storageClass := &storagev1.StorageClass{}
			if err := r.Get(ctx, types.NamespacedName{Name: *foundPVC.Spec.StorageClassName}, storageClass); err != nil {
				log.Error(err, "Failed to get the storage class of the PVC", "StorageClass", *foundPVC.Spec.StorageClassName)
				return err
			}

			if !allowsVolumeExpansion(storageClass) {
				log.Info("StorageClass does not allow volume expansion", "StorageClass", storageClass.Name)
				return nil
			}

			foundPVC.Spec.Resources.Requests[corev1.ResourceStorage] = desiredSize
			if err := r.Update(ctx, foundPVC); err != nil {
				log.Error(err, "Failed to update PVC size", "PVC.Namespace", foundPVC.Namespace, "PVC.Name", pvcName)
				return err
			}
			r.Recorder.Eventf(f, corev1.EventTypeNormal, "Updated", "Updated PVC %s", pvcName)
		}
	}

	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := r.List(ctx, pvcList, client.InNamespace(f.Namespace)); err != nil {
		log.Error(err, "Failed to list PVCs", "Rollout.Namespace", f.Namespace)
		return err
	}

	for _, pvc := range pvcList.Items {
		if _, exists := expectedPVCs[pvc.Name]; !exists && isOwnedByRollout(&pvc, f) {
			if err := r.Delete(ctx, &pvc); err != nil {
				r.Recorder.Eventf(f, corev1.EventTypeWarning, "DeletionFailed", "Failed to delete PVC %s", pvc.Name)
				return err
			}
			r.Recorder.Eventf(f, corev1.EventTypeNormal, "Deleted", "Deleted PVC %s", pvc.Name)
		}
	}

	return nil
}

func allowsVolumeExpansion(sc *storagev1.StorageClass) bool {
	return sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion
}

func (r *RolloutReconciler) constructPVCForRollout(f *oneclickiov1alpha1.Rollout, volSpec oneclickiov1alpha1.VolumeSpec) *corev1.PersistentVolumeClaim {
	labels := map[string]string{
		"one-click.dev/projectId":    f.Namespace,
		"one-click.dev/deploymentId": f.Name,
	}

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      volSpec.Name + "-" + f.Name,
			Namespace: f.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(f, f.GroupVersionKind()),
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(volSpec.Size)}},
			StorageClassName: &volSpec.StorageClass,
		},
	}
}

func isOwnedByRollout(pvc *corev1.PersistentVolumeClaim, f *oneclickiov1alpha1.Rollout) bool {
	for _, ref := range pvc.GetOwnerReferences() {
		if ref.Kind == "Rollout" && ref.Name == f.Name {
			return true
		}
	}
	return false
}

func (r *RolloutReconciler) deleteAllPVCsForRollout(ctx context.Context, f *oneclickiov1alpha1.Rollout) error {
	log := log.FromContext(ctx)

	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := r.List(ctx, pvcList, client.InNamespace(f.Namespace)); err != nil {
		log.Error(err, "Failed to list PVCs for Rollout", "Rollout.Namespace", f.Namespace)
		return err
	}

	for _, pvc := range pvcList.Items {
		if isOwnedByRollout(&pvc, f) {
			if err := r.Delete(ctx, &pvc); err != nil {
				r.Recorder.Eventf(f, corev1.EventTypeWarning, "DeletionFailed", "Failed to delete PVC %s", pvc.Name)
				return err
			}
			r.Recorder.Eventf(f, corev1.EventTypeNormal, "Deleted", "Deleted PVC %s", pvc.Name)
		}
	}

	return nil
}
