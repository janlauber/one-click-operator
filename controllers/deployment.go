package controllers

import (
	"context"
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	oneclickiov1alpha1 "github.com/janlauber/one-click-operator/api/v1alpha1"
)

func (r *RolloutReconciler) reconcileDeployment(ctx context.Context, f *oneclickiov1alpha1.Rollout) error {
	log := log.FromContext(ctx)
	deploymentName := f.Name
	namespace := f.Namespace

	result := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentDeployment := &appsv1.Deployment{}
		err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: namespace}, currentDeployment)
		if err != nil {
			if errors.IsNotFound(err) {
				// Deployment not found, create a new one
				desiredDeployment := r.deploymentForRollout(ctx, f)
				r.Recorder.Eventf(f, corev1.EventTypeNormal, "Creating", "Creating Deployment %s", deploymentName)
				return r.Create(ctx, desiredDeployment)
			}
			// Other error while fetching the Deployment
			return err
		}

		// Deployment found, check if it needs updating
		desiredDeployment := r.deploymentForRollout(ctx, f)
		if needsUpdate(currentDeployment, f) {
			// ignore the replicas field because it is managed by the HorizontalPodAutoscaler
			desiredDeployment.Spec.Replicas = currentDeployment.Spec.Replicas
			// Update the Deployment to align it with the Rollout spec
			currentDeployment.Spec = desiredDeployment.Spec
			updateErr := r.Update(ctx, currentDeployment)
			if updateErr != nil {
				r.Recorder.Eventf(f, corev1.EventTypeWarning, "UpdateFailed", "Failed to update Deployment %s", deploymentName)
				return updateErr
			}
			r.Recorder.Eventf(f, corev1.EventTypeNormal, "Updated", "Updated Deployment %s", deploymentName)
		}
		return nil
	})

	if result != nil {
		log.Error(result, "Failed to reconcile Deployment", "Deployment.Namespace", namespace, "Deployment.Name", deploymentName)
	}

	return result
}

func (r *RolloutReconciler) deploymentForRollout(ctx context.Context, f *oneclickiov1alpha1.Rollout) *appsv1.Deployment {
	labels := map[string]string{
		"one-click.dev/projectId":    f.Namespace,
		"one-click.dev/deploymentId": f.Name,
	}

	// Handle image pull secret if username and password are provided
	var imagePullSecrets []corev1.LocalObjectReference
	if f.Spec.Image.Username != "" && f.Spec.Image.Password != "" {
		secretName := f.Name + "-imagepullsecret"
		if err := reconcileImagePullSecret(ctx, r.Client, f, f.Spec.Image, secretName, f.Namespace); err != nil {
			r.Recorder.Eventf(f, corev1.EventTypeWarning, "ImagePullSecretFailed", "Failed to reconcile Image Pull Secret %s", secretName)
		} else {
			imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: secretName})
		}
	}

	// Determine the rollout strategy
	strategy := appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
	}

	if f.Spec.RolloutStrategy != "" {
		switch f.Spec.RolloutStrategy {
		case "recreate":
			strategy.Type = appsv1.RecreateDeploymentStrategyType
		case "rollingUpdate":
			strategy.Type = appsv1.RollingUpdateDeploymentStrategyType
		default:
			strategy.Type = appsv1.RollingUpdateDeploymentStrategyType
		}
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      f.Name,
			Namespace: f.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:      f.Name,
						Image:     fmt.Sprintf("%s/%s:%s", f.Spec.Image.Registry, f.Spec.Image.Repository, f.Spec.Image.Tag),
						Resources: createResourceRequirements(f.Spec.Resources),
						Env:       getEnvVars(f.Spec.Env),
					}},
					ServiceAccountName: f.Spec.ServiceAccountName,
					ImagePullSecrets:   imagePullSecrets,
					NodeSelector:       f.Spec.NodeSelector,
					Tolerations:        getTolerations(f.Spec.Tolerations),
					HostAliases:        f.Spec.HostAliases,
				},
			},
			Strategy: strategy,
		},
	}

	// if args are defined, add them to the container
	if len(f.Spec.Args) > 0 {
		dep.Spec.Template.Spec.Containers[0].Args = f.Spec.Args
	}

	// if command is defined, add it to the container
	if len(f.Spec.Command) > 0 {
		dep.Spec.Template.Spec.Containers[0].Command = f.Spec.Command
	}

	// if security context is defined, add it to the pod security context
	if !reflect.DeepEqual(f.Spec.SecurityContext, oneclickiov1alpha1.SecurityContextSpec{}) {
		dep.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			FSGroup:    &f.Spec.SecurityContext.FsGroup,
			RunAsUser:  &f.Spec.SecurityContext.RunAsUser,
			RunAsGroup: &f.Spec.SecurityContext.RunAsGroup,
		}

		capabilities := make([]corev1.Capability, len(f.Spec.SecurityContext.Capabilities.Add))
		for i, cap := range f.Spec.SecurityContext.Capabilities.Add {
			capabilities[i] = corev1.Capability(cap)
		}

		dropCapabilities := make([]corev1.Capability, len(f.Spec.SecurityContext.Capabilities.Drop))
		for i, cap := range f.Spec.SecurityContext.Capabilities.Drop {
			dropCapabilities[i] = corev1.Capability(cap)
		}

		dep.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
			Privileged:               &f.Spec.SecurityContext.Privileged,
			ReadOnlyRootFilesystem:   &f.Spec.SecurityContext.ReadOnlyRootFilesystem,
			RunAsNonRoot:             &f.Spec.SecurityContext.RunAsNonRoot,
			AllowPrivilegeEscalation: &f.Spec.SecurityContext.AllowPrivilegeEscalation,
			Capabilities: &corev1.Capabilities{
				Add:  capabilities,
				Drop: dropCapabilities,
			},
		}
	}

	// if secrets are defined, add the secret f.Name + "-secrets" as envFrom
	if len(f.Spec.Secrets) > 0 {
		dep.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{
			{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: f.Name + "-secrets",
					},
				},
			},
		}
	}

	// Update volumes and volume mounts
	if len(f.Spec.Volumes) > 0 {
		var volumes []corev1.Volume
		var volumeMounts []corev1.VolumeMount
		for _, v := range f.Spec.Volumes {
			volumes = append(volumes, corev1.Volume{
				Name: v.Name + "-" + f.Name,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: v.Name + "-" + f.Name,
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      v.Name + "-" + f.Name,
				MountPath: v.MountPath,
			})
		}
		dep.Spec.Template.Spec.Volumes = volumes
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts
	} else {
		// Handle no volumes case
		dep.Spec.Template.Spec.Volumes = nil
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = nil
	}

	// Add secret checksum to pod template annotations to trigger redeployment when secrets change
	checksum := calculateSecretsChecksum(f.Spec.Secrets)
	if dep.Spec.Template.Annotations == nil {
		dep.Spec.Template.Annotations = make(map[string]string)
	}
	dep.Spec.Template.Annotations["one-click.dev/secrets-checksum"] = checksum

	ctrl.SetControllerReference(f, dep, r.Scheme)
	return dep
}

func needsUpdate(current *appsv1.Deployment, f *oneclickiov1alpha1.Rollout) bool {
	// Check security context
	if !reflect.DeepEqual(current.Spec.Template.Spec.SecurityContext, &corev1.PodSecurityContext{
		FSGroup:    &f.Spec.SecurityContext.FsGroup,
		RunAsUser:  &f.Spec.SecurityContext.RunAsUser,
		RunAsGroup: &f.Spec.SecurityContext.RunAsGroup,
	}) {
		return true
	}

	// Check container security context
	capabilities := make([]corev1.Capability, len(f.Spec.SecurityContext.Capabilities.Add))
	for i, cap := range f.Spec.SecurityContext.Capabilities.Add {
		capabilities[i] = corev1.Capability(cap)
	}

	dropCapabilities := make([]corev1.Capability, len(f.Spec.SecurityContext.Capabilities.Drop))
	for i, cap := range f.Spec.SecurityContext.Capabilities.Drop {
		dropCapabilities[i] = corev1.Capability(cap)
	}

	if !reflect.DeepEqual(current.Spec.Template.Spec.Containers[0].SecurityContext, &corev1.SecurityContext{
		Privileged:               &f.Spec.SecurityContext.Privileged,
		ReadOnlyRootFilesystem:   &f.Spec.SecurityContext.ReadOnlyRootFilesystem,
		RunAsNonRoot:             &f.Spec.SecurityContext.RunAsNonRoot,
		AllowPrivilegeEscalation: &f.Spec.SecurityContext.AllowPrivilegeEscalation,
		Capabilities: &corev1.Capabilities{
			Add:  capabilities,
			Drop: dropCapabilities,
		},
	}) {
		return true
	}

	// Check container image
	desiredImage := fmt.Sprintf("%s/%s:%s", f.Spec.Image.Registry, f.Spec.Image.Repository, f.Spec.Image.Tag)
	if len(current.Spec.Template.Spec.Containers) == 0 || current.Spec.Template.Spec.Containers[0].Image != desiredImage {
		return true
	}

	// Check secrets
	if len(f.Spec.Secrets) > 0 {
		if len(current.Spec.Template.Spec.Containers[0].EnvFrom) == 0 {
			return true
		}

		if current.Spec.Template.Spec.Containers[0].EnvFrom[0].SecretRef.LocalObjectReference.Name != f.Name+"-secrets" {
			return true
		}
	} else {
		if len(current.Spec.Template.Spec.Containers[0].EnvFrom) > 0 {
			return true
		}
	}

	// Check environment variables
	desiredEnvVars := getEnvVars(f.Spec.Env)
	if !reflect.DeepEqual(current.Spec.Template.Spec.Containers[0].Env, desiredEnvVars) {
		return true
	}

	// Check resource requests and limits
	desiredResources := createResourceRequirements(f.Spec.Resources)
	if !reflect.DeepEqual(current.Spec.Template.Spec.Containers[0].Resources, desiredResources) {
		return true
	}

	// Check ports
	if !portsMatch(current.Spec.Template.Spec.Containers[0].Ports, f.Spec.Interfaces) {
		return true
	}

	// Check volumes
	if !volumesMatch(current.Spec.Template.Spec.Volumes, f.Spec.Volumes, f) {
		return true
	}

	// Check service account name
	if current.Spec.Template.Spec.ServiceAccountName != f.Spec.ServiceAccountName {
		return true
	}

	// Check command
	if !reflect.DeepEqual(current.Spec.Template.Spec.Containers[0].Command, f.Spec.Command) {
		return true
	}

	// Check args
	if !reflect.DeepEqual(current.Spec.Template.Spec.Containers[0].Args, f.Spec.Args) {
		return true
	}

	// Check rollout strategy
	strategy := appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
	}
	if f.Spec.RolloutStrategy != "" {
		switch f.Spec.RolloutStrategy {
		case "recreate":
			strategy.Type = appsv1.RecreateDeploymentStrategyType
		case "rollingUpdate":
			strategy.Type = appsv1.RollingUpdateDeploymentStrategyType
		default:
			strategy.Type = appsv1.RollingUpdateDeploymentStrategyType
		}
	}

	if !reflect.DeepEqual(current.Spec.Strategy, strategy) {
		return true
	}

	// Check node selector
	if !reflect.DeepEqual(current.Spec.Template.Spec.NodeSelector, f.Spec.NodeSelector) {
		return true
	}

	// Check tolerations
	if !reflect.DeepEqual(current.Spec.Template.Spec.Tolerations, getTolerations(f.Spec.Tolerations)) {
		return true
	}

	// Check host aliases
	if !reflect.DeepEqual(current.Spec.Template.Spec.HostAliases, f.Spec.HostAliases) {
		return true
	}

	// Add more checks as necessary, e.g., labels, annotations, specific configuration, etc.

	return false
}

func getTolerations(tolerations []corev1.Toleration) []corev1.Toleration {
	var result []corev1.Toleration
	for _, t := range tolerations {
		result = append(result, corev1.Toleration{
			Key:               t.Key,
			Operator:          corev1.TolerationOperator(t.Operator),
			Value:             t.Value,
			Effect:            corev1.TaintEffect(t.Effect),
			TolerationSeconds: t.TolerationSeconds,
		})
	}
	return result
}

func volumesMatch(currentVolumes []corev1.Volume, desiredVolumes []oneclickiov1alpha1.VolumeSpec, f *oneclickiov1alpha1.Rollout) bool {
	if len(currentVolumes) != len(desiredVolumes) {
		return false
	}

	desiredVolumeMap := make(map[string]oneclickiov1alpha1.VolumeSpec)
	for _, v := range desiredVolumes {
		desiredVolumeMap[v.Name+"-"+f.Name] = v
	}

	for _, currentVolume := range currentVolumes {
		volSpec, exists := desiredVolumeMap[currentVolume.Name+"-"+f.Name]
		if !exists {
			// Volume is present in Deployment but not in Rollout spec
			return false
		}

		// Check PVC name
		if currentVolume.VolumeSource.PersistentVolumeClaim.ClaimName != volSpec.Name+"-"+f.Name {
			return false
		}
		// Additional checks can be added here, such as PVC size, storage class, etc.
	}

	return true
}

func portsMatch(currentPorts []corev1.ContainerPort, interfaces []oneclickiov1alpha1.InterfaceSpec) bool {
	if len(currentPorts) != len(interfaces) {
		return false
	}

	for i, intf := range interfaces {
		if currentPorts[i].ContainerPort != intf.Port {
			return false
		}

		// check for name
		if currentPorts[i].Name != intf.Name {
			return false
		}
	}

	return true
}
