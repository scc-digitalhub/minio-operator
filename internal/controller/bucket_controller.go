// SPDX-License-Identifier: AGPL-3.0-or-later

package controller

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7"

	operatorv1 "github.com/scc-digitalhub/minio-operator/api/v1"
)

const bucketFinalizer = "minio.scc-digitalhub.github.io/bucket-finalizer"

const envEmptyBucketOnDelete = "MINIO_EMPTY_BUCKET_ON_DELETE"

// BucketReconciler reconciles a Bucket object
type BucketReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=minio.scc-digitalhub.github.io,resources=buckets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=minio.scc-digitalhub.github.io,resources=buckets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=minio.scc-digitalhub.github.io,resources=buckets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *BucketReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	cr := &operatorv1.Bucket{}
	err := r.Get(ctx, req.NamespacedName, cr)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found, it usually means that it was deleted or not created
			log.Info("resource not found; ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	// If status is unknown, set Creating
	if cr.Status.State == "" {
		log.Info("State unspecified, updating to creating")
		cr.Status.State = typeCreating
		if err = r.Status().Update(ctx, cr); err != nil {
			log.Error(err, genericStatusUpdateFailedMessage)
			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	}

	// Create resource, if it doesn't exist
	if cr.Status.State == typeCreating {
		log.Info("Creating resource")

		client, err := getClient()
		if err != nil {
			log.Error(err, failedToObtainClientMessage)
			return setBucketErrorState(r, ctx, cr, err)
		}

		// Create resource
		err = client.MakeBucket(context.Background(), cr.Spec.Name, minio.MakeBucketOptions{})
		if err != nil && !strings.Contains(err.Error(), "Your previous request to create the named bucket succeeded and you already own it") {
			log.Error(err, "Error while creating bucket")
			return setBucketErrorState(r, ctx, cr, err)
		}

		if cr.Spec.Quota != 0 {
			err = setQuota(cr.Spec.Name, cr.Spec.Quota)
			if err != nil {
				log.Error(err, "Failed to set quota")
				return setBucketErrorState(r, ctx, cr, err)
			}
		}

		// Add finalizer
		if !controllerutil.ContainsFinalizer(cr, bucketFinalizer) {
			log.Info("Adding finalizer for resource")
			if ok := controllerutil.AddFinalizer(cr, bucketFinalizer); !ok {
				log.Error(err, "Failed to add finalizer to the custom resource")
				return ctrl.Result{Requeue: true}, nil
			}

			if err = r.Update(ctx, cr); err != nil {
				log.Error(err, "Failed to update custom resource to add finalizer")
				return ctrl.Result{}, err
			}

			if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
				log.Error(err, "Failed to re-fetch resource")
				return ctrl.Result{}, err
			}
		}

		cr.Status.State = typeReady
		if err = r.Status().Update(ctx, cr); err != nil {
			log.Error(err, genericStatusUpdateFailedMessage)
			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	}

	// Check if the instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isMarkedToBeDeleted := cr.GetDeletionTimestamp() != nil
	if isMarkedToBeDeleted {
		log.Info("Resource marked to be deleted")
		if controllerutil.ContainsFinalizer(cr, bucketFinalizer) {
			log.Info("Performing finalizer operations before deleting CR")

			// Perform all operations required before removing the finalizer to allow
			// the Kubernetes API to remove the custom resource.
			if err := r.finalizerOpsForBucket(cr); err != nil {
				log.Error(err, "Finalizer operations failed")
				return setBucketErrorState(r, ctx, cr, err)
			}

			cr.Status.State = typeDegraded

			if err := r.Status().Update(ctx, cr); err != nil {
				log.Error(err, genericStatusUpdateFailedMessage)
				return ctrl.Result{}, err
			}

			log.Info("Removing finalizer after successfully performing operations")
			if ok := controllerutil.RemoveFinalizer(cr, bucketFinalizer); !ok {
				log.Error(err, "failed to remove finalizer")
				return ctrl.Result{Requeue: true}, nil
			}

			if err := r.Update(ctx, cr); err != nil {
				log.Error(err, "failed to update resource")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Check if resource needs updating
	if cr.Status.State == typeReady {
		log.Info("Resource in Ready state")

		// Check quota
		adminClient, err := getAdminClient()
		if err != nil {
			log.Error(err, failedToObtainAdminClientMessage)
			return setBucketErrorState(r, ctx, cr, err)
		}

		quota, err := adminClient.GetBucketQuota(context.Background(), cr.Spec.Name)
		if err != nil {
			log.Error(err, "Failed to check resource properties")
		}

		if quota.Quota != cr.Spec.Quota {
			cr.Status.State = typeUpdating
			if err = r.Status().Update(ctx, cr); err != nil {
				log.Error(err, genericStatusUpdateFailedMessage)
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	// Update resource
	if cr.Status.State == typeUpdating {
		log.Info("Updating resource")

		// Set quota
		err = setQuota(cr.Spec.Name, cr.Spec.Quota)
		if err != nil {
			log.Error(err, "Failed to set quota")
			return setBucketErrorState(r, ctx, cr, err)
		}

		// Update status
		cr.Status.State = typeReady
		if err = r.Status().Update(ctx, cr); err != nil {
			log.Error(err, genericStatusUpdateFailedMessage)
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// Error state
	if cr.Status.State == typeError {
		log.Info("Resource in error state")
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.Bucket{}).
		Complete(r)
}

// Perform required operations before deleting the CR
func (r *BucketReconciler) finalizerOpsForBucket(cr *operatorv1.Bucket) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	emptyBucketOnDelete, err := readEmptyBucketOnDelete()
	if err != nil {
		return err
	}

	// According to the documentation, client.RemoveObjects
	// only deletes up to 1000 objects, hence the for loop
	for {
		err = client.RemoveBucket(context.Background(), cr.Spec.Name)
		if err == nil || strings.Contains(err.Error(), "does not exist") {
			err = nil
			break
		} else if strings.Contains(err.Error(), "not empty") && emptyBucketOnDelete {
			// List objects
			listOpts := minio.ListObjectsOptions{
				Recursive:    true,
				WithVersions: true,
			}
			objectsCh := client.ListObjects(context.Background(), cr.Spec.Name, listOpts)

			// Delete them
			client.RemoveObjects(context.Background(), cr.Spec.Name, objectsCh, minio.RemoveObjectsOptions{GovernanceBypass: true})
		} else {
			break
		}
	}

	if err != nil {
		return err
	}

	// The following implementation will raise an event
	r.Recorder.Event(cr, "Warning", "Deleting",
		fmt.Sprintf("Custom Resource %s is being deleted from the namespace %s",
			cr.Name,
			cr.Namespace))

	return nil
}

func setBucketErrorState(r *BucketReconciler, ctx context.Context, cr *operatorv1.Bucket, err error) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	cr.Status.State = typeError
	cr.Status.Message = err.Error()

	if err := r.Status().Update(ctx, cr); err != nil {
		log.Error(err, genericStatusUpdateFailedMessage)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, err
}

func setQuota(bucketName string, value uint64) error {
	adminClient, err := getAdminClient()
	if err != nil {
		return err
	}

	quota := &madmin.BucketQuota{
		Quota: value,
		Type:  madmin.HardQuota,
	}

	err = adminClient.SetBucketQuota(context.Background(), bucketName, quota)
	if err != nil {
		return err
	}

	return nil
}

func readEmptyBucketOnDelete() (bool, error) {
	emptyBucketOnDelete := false

	emptyBucketOnDeleteString, found := os.LookupEnv(envEmptyBucketOnDelete)
	if found {
		emptyBucketOnDeleteParsed, err := strconv.ParseBool(emptyBucketOnDeleteString)
		if err != nil {
			return false, fmt.Errorf("%s must be either true or false", envEmptyBucketOnDelete)
		}
		emptyBucketOnDelete = emptyBucketOnDeleteParsed
	}

	return emptyBucketOnDelete, nil
}
