package controllers

import (
	"context"
	"fmt"
	"reflect"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	oneclickiov1alpha1 "github.com/janlauber/one-click-operator/api/v1alpha1"
)

func (r *RolloutReconciler) reconcileCronJobs(ctx context.Context, instance *oneclickiov1alpha1.Rollout) error {
	log := log.FromContext(ctx)

	// Track the CronJobs defined in the Rollout spec
	definedCronJobs := make(map[string]oneclickiov1alpha1.CronJobSpec)
	for _, cronJobSpec := range instance.Spec.CronJobs {
		definedCronJobs[cronJobSpec.Name] = cronJobSpec
		cronJob := &batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cronJobSpec.Name,
				Namespace: instance.Namespace,
			},
			Spec: batchv1.CronJobSpec{
				Suspend:  &cronJobSpec.Suspend,
				Schedule: cronJobSpec.Schedule,
				JobTemplate: batchv1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:    cronJobSpec.Name,
										Image:   fmt.Sprintf("%s/%s:%s", cronJobSpec.Image.Registry, cronJobSpec.Image.Repository, cronJobSpec.Image.Tag),
										Command: cronJobSpec.Command,
										Args:    cronJobSpec.Args,
										Env:     getEnvVars(cronJobSpec.Env),
										Resources: corev1.ResourceRequirements{
											Limits: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse(cronJobSpec.Resources.Limits.CPU),
												corev1.ResourceMemory: resource.MustParse(cronJobSpec.Resources.Limits.Memory),
											},
											Requests: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse(cronJobSpec.Resources.Requests.CPU),
												corev1.ResourceMemory: resource.MustParse(cronJobSpec.Resources.Requests.Memory),
											},
										},
									},
								},
								RestartPolicy: corev1.RestartPolicyOnFailure,
							},
						},
						BackoffLimit: &cronJobSpec.BackoffLimit,
					},
				},
			},
		}

		// Set Rollout instance as the owner and controller
		if err := ctrl.SetControllerReference(instance, cronJob, r.Scheme); err != nil {
			return err
		}

		// Check if this CronJob already exists
		found := &batchv1.CronJob{}
		err := r.Get(ctx, types.NamespacedName{Name: cronJob.Name, Namespace: cronJob.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			log.Info("Creating a new CronJob", "CronJob.Namespace", cronJob.Namespace, "CronJob.Name", cronJob.Name)
			err = r.Create(ctx, cronJob)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			// Update existing CronJob if needed
			if needsUpdateCronJob(found, cronJob) {
				found.Spec = cronJob.Spec
				log.Info("Updating existing CronJob", "CronJob.Namespace", cronJob.Namespace, "CronJob.Name", cronJob.Name)
				err = r.Update(ctx, found)
				if err != nil {
					return err
				}
			}
		}
	}

	// List all CronJobs in the namespace to find any that are not defined in the Rollout spec
	var existingCronJobs batchv1.CronJobList
	if err := r.List(ctx, &existingCronJobs, client.InNamespace(instance.Namespace), client.MatchingFields{"metadata.ownerReferences.uid": string(instance.UID)}); err != nil {
		return err
	}

	for _, existingCronJob := range existingCronJobs.Items {
		if _, exists := definedCronJobs[existingCronJob.Name]; !exists {
			// CronJob is not defined in the Rollout spec, so delete it
			log.Info("Deleting CronJob not defined in Rollout spec", "CronJob.Namespace", existingCronJob.Namespace, "CronJob.Name", existingCronJob.Name)
			if err := r.Delete(ctx, &existingCronJob); err != nil {
				return err
			}
		}
	}

	return nil
}

// Helper function to check if the CronJob needs to be updated
func needsUpdateCronJob(current *batchv1.CronJob, desired *batchv1.CronJob) bool {
	return !reflect.DeepEqual(current.Spec, desired.Spec)
}

// func getEnvVars(envVars []oneclickiov1alpha1.EnvVar) []corev1.EnvVar {
// 	var envs []corev1.EnvVar
// 	for _, env := range envVars {
// 		envs = append(envs, corev1.EnvVar{
// 			Name:  env.Name,
// 			Value: env.Value,
// 		})
// 	}
// 	return envs
// }
