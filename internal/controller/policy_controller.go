// SPDX-License-Identifier: AGPL-3.0-or-later

package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1 "github.com/scc-digitalhub/minio-operator/api/v1"
)

const policyFinalizer = "minio.scc-digitalhub.github.io/policy-finalizer"

// PolicyReconciler reconciles a Policy object
type PolicyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=minio.scc-digitalhub.github.io,resources=policies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=minio.scc-digitalhub.github.io,resources=policies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=minio.scc-digitalhub.github.io,resources=policies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	cr := &operatorv1.Policy{}
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

		adminClient, err := getAdminClient()
		if err != nil {
			log.Error(err, failedToObtainAdminClientMessage)
			return setPolicyErrorState(r, ctx, cr, err)
		}

		err = adminClient.AddCannedPolicy(context.Background(), cr.Spec.Name, []byte(cr.Spec.Content))
		if err != nil {
			log.Error(err, "Error while creating policy")
			return setPolicyErrorState(r, ctx, cr, err)
		}

		// Since MinIO may strip unused fields, retrieve its saved policy and overwrite ours
		policyInfo, err := adminClient.InfoCannedPolicyV2(context.Background(), cr.Spec.Name)
		if err != nil {
			log.Error(err, "Failed to retrieve generated policy info")
			return setPolicyErrorState(r, ctx, cr, err)
		}
		marshalled, err := json.Marshal(policyInfo.Policy)
		if err != nil {
			log.Error(err, "Failed to marshal generated policy info")
			return setPolicyErrorState(r, ctx, cr, err)
		}
		cr.Spec.Content = string(marshalled[:])
		r.Update(ctx, cr)

		// Add finalizer
		if !controllerutil.ContainsFinalizer(cr, policyFinalizer) {
			log.Info("Adding finalizer for resource")
			if ok := controllerutil.AddFinalizer(cr, policyFinalizer); !ok {
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
		if controllerutil.ContainsFinalizer(cr, policyFinalizer) {
			log.Info("Performing finalizer operations before deleting CR")

			// Perform all operations required before removing the finalizer to allow
			// the Kubernetes API to remove the custom resource.
			if err := r.finalizerOpsForPolicy(cr); err != nil {
				log.Error(err, "Finalizer operations failed")
				return setPolicyErrorState(r, ctx, cr, err)
			}

			cr.Status.State = typeDegraded

			if err := r.Status().Update(ctx, cr); err != nil {
				log.Error(err, genericStatusUpdateFailedMessage)
				return ctrl.Result{}, err
			}

			log.Info("Removing finalizer after successfully performing operations")
			if ok := controllerutil.RemoveFinalizer(cr, policyFinalizer); !ok {
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

	if cr.Status.State == typeReady {
		log.Info("Resource in Ready state")
		adminClient, err := getAdminClient()
		if err != nil {
			log.Error(err, failedToObtainAdminClientMessage)
			return setPolicyErrorState(r, ctx, cr, err)
		}

		policyInfo, err := adminClient.InfoCannedPolicyV2(context.Background(), cr.Spec.Name)
		if err != nil {
			log.Error(err, "Failed to retrieve policy info")
			return setPolicyErrorState(r, ctx, cr, err)
		}

		equivalent, err := equivalentPolicies(policyInfo.Policy, cr.Spec.Content)
		if err != nil {
			log.Error(err, "Failed to compare policies")
			return setPolicyErrorState(r, ctx, cr, err)
		} else if !equivalent {
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

		// Update policy content
		adminClient, err := getAdminClient()
		if err != nil {
			log.Error(err, failedToObtainAdminClientMessage)
			return setPolicyErrorState(r, ctx, cr, err)
		}
		err = adminClient.AddCannedPolicy(context.Background(), cr.Spec.Name, []byte(cr.Spec.Content))
		if err != nil {
			log.Error(err, "Error while updating policy")
			return setPolicyErrorState(r, ctx, cr, err)
		}

		// Since MinIO may strip unused fields, retrieve its saved policy and overwrite ours
		policyInfo, err := adminClient.InfoCannedPolicyV2(context.Background(), cr.Spec.Name)
		if err != nil {
			log.Error(err, "Failed to retrieve generated policy info")
			return setPolicyErrorState(r, ctx, cr, err)
		}
		marshalled, err := json.Marshal(policyInfo.Policy)
		if err != nil {
			log.Error(err, "Failed to marshal generated policy info")
			return setPolicyErrorState(r, ctx, cr, err)
		}
		cr.Spec.Content = string(marshalled[:])
		r.Update(ctx, cr)

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
func (r *PolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.Policy{}).
		Complete(r)
}

func setPolicyErrorState(r *PolicyReconciler, ctx context.Context, cr *operatorv1.Policy, err error) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	cr.Status.State = typeError
	cr.Status.Message = err.Error()

	if err := r.Status().Update(ctx, cr); err != nil {
		log.Error(err, genericStatusUpdateFailedMessage)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, err
}

// Perform required operations before deleting the CR
func (r *PolicyReconciler) finalizerOpsForPolicy(cr *operatorv1.Policy) error {
	adminClient, err := getAdminClient()
	if err != nil {
		return err
	}

	err = adminClient.RemoveCannedPolicy(context.Background(), cr.Spec.Name)
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

func equivalentPolicies(currentPolicy json.RawMessage, newPolicy string) (bool, error) {
	// Marshal current policy
	currentPolicyMarshalled, err := json.Marshal(currentPolicy)
	if err != nil {
		return false, err
	}

	// Compact new policy
	newPolicyBuffer := new(bytes.Buffer)
	err = json.Compact(newPolicyBuffer, []byte(newPolicy))
	if err != nil {
		return false, err
	}

	return bytes.Equal(newPolicyBuffer.Bytes(), currentPolicyMarshalled), nil
}
