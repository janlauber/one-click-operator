package controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	oneclickiov1alpha1 "github.com/janlauber/one-click-operator/api/v1alpha1"
)

func getEnvVars(envVars []oneclickiov1alpha1.EnvVar) []corev1.EnvVar {
	var envs []corev1.EnvVar
	for _, env := range envVars {
		envs = append(envs, corev1.EnvVar{
			Name:  env.Name,
			Value: env.Value,
		})
	}
	return envs
}

func createResourceRequirements(resources oneclickiov1alpha1.ResourceRequirements) corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(resources.Requests.CPU),
			corev1.ResourceMemory: resource.MustParse(resources.Requests.Memory),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(resources.Limits.CPU),
			corev1.ResourceMemory: resource.MustParse(resources.Limits.Memory),
		},
	}
}

func reconcileImagePullSecret(ctx context.Context, client client.Client, instance *oneclickiov1alpha1.Rollout, imageSpec oneclickiov1alpha1.ImageSpec, secretName string, namespace string) error {
	auth := base64.StdEncoding.EncodeToString([]byte(imageSpec.Username + ":" + imageSpec.Password))
	dockerConfigEntry := map[string]interface{}{
		"username": imageSpec.Username,
		"password": imageSpec.Password,
		"auth":     auth,
	}
	dockerConfigJSON := map[string]interface{}{
		"auths": map[string]interface{}{
			imageSpec.Registry: dockerConfigEntry,
		},
	}
	dockerConfigJSONBytes, err := json.Marshal(dockerConfigJSON)
	if err != nil {
		return err
	}

	secretData := map[string][]byte{
		".dockerconfigjson": dockerConfigJSONBytes,
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: secretData,
	}

	// Set the owner reference
	if err := controllerutil.SetControllerReference(instance, secret, client.Scheme()); err != nil {
		return err
	}

	// Check if secret already exists, if not, create it
	foundSecret := &corev1.Secret{}
	err = client.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, foundSecret)
	if err != nil && errors.IsNotFound(err) {
		// Secret not found, create it
		return client.Create(ctx, secret)
	} else if err != nil {
		// Error occurred while checking for secret
		return err
	}

	// Secret found, update it
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		foundSecret.Data = secretData
		return client.Update(ctx, foundSecret)
	})
}
