/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"slices"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/minio/madmin-go/v3"
	miniov1 "github.com/scc-digitalhub/minio-operator/api/v1"
	operatorv1 "github.com/scc-digitalhub/minio-operator/api/v1"
)

const userFinalizer = "minio.scc-digitalhub.github.io/user-finalizer"

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=minio.scc-digitalhub.github.io,resources=users,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=minio.scc-digitalhub.github.io,resources=users/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=minio.scc-digitalhub.github.io,resources=users/finalizers,verbs=update

func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	cr := &operatorv1.User{}
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
			return setUserErrorState(r, ctx, cr, err)
		}

		// Does not return error if user already exists
		err = adminClient.SetUser(context.Background(), cr.Spec.AccessKey, cr.Spec.SecretKey, madmin.AccountStatus(cr.Spec.AccountStatus))
		if err != nil {
			log.Error(err, "Error while creating user")
			return setUserErrorState(r, ctx, cr, err)
		}

		// Add finalizer
		if !controllerutil.ContainsFinalizer(cr, userFinalizer) {
			log.Info("Adding finalizer for resource")
			if ok := controllerutil.AddFinalizer(cr, userFinalizer); !ok {
				log.Error(err, "Failed to add finalizer to the custom resource")
				return ctrl.Result{Requeue: true}, nil
			}

			if err = r.Update(ctx, cr); err != nil {
				log.Error(err, "Failed to update custom resource to add finalizer")
				return ctrl.Result{}, err
			}
		}

		// Set policies
		if cr.Spec.AccountStatus == "enabled" && len(cr.Spec.Policies) > 0 {
			req := madmin.PolicyAssociationReq{
				Policies: cr.Spec.Policies,
				User:     cr.Spec.AccessKey,
			}

			_, err := adminClient.AttachPolicy(context.Background(), req)
			if err != nil && !strings.Contains(err.Error(), "policy update has no net effect") {
				log.Error(err, "Error while assigning policies to user")
				return setUserErrorState(r, ctx, cr, err)
			}
		}

		if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
			log.Error(err, "Failed to re-fetch resource")
			return ctrl.Result{}, err
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
		if controllerutil.ContainsFinalizer(cr, userFinalizer) {
			log.Info("Performing finalizer operations before deleting CR")

			// Perform all operations required before removing the finalizer to allow
			// the Kubernetes API to remove the custom resource.
			if err := r.finalizerOpsForUser(cr); err != nil {
				log.Error(err, "Finalizer operations failed")
				return ctrl.Result{Requeue: true}, nil
			}

			cr.Status.State = typeDegraded

			if err := r.Status().Update(ctx, cr); err != nil {
				log.Error(err, genericStatusUpdateFailedMessage)
				return ctrl.Result{}, err
			}

			log.Info("Removing finalizer after successfully performing operations")
			if ok := controllerutil.RemoveFinalizer(cr, userFinalizer); !ok {
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
		adminClient, err := getAdminClient()
		if err != nil {
			log.Error(err, failedToObtainAdminClientMessage)
			return setUserErrorState(r, ctx, cr, err)
		}

		// We are unable to check if the secret key has changed, so we just set it again
		err = adminClient.SetUser(context.Background(), cr.Spec.AccessKey, cr.Spec.SecretKey, madmin.AccountStatus(cr.Spec.AccountStatus))
		if err != nil {
			log.Error(err, "Error setting user")
			return setUserErrorState(r, ctx, cr, err)
		}

		// Check policies
		userInfo, err := adminClient.GetUserInfo(context.Background(), cr.Spec.AccessKey)
		if err != nil {
			log.Error(err, "Unable to retrieve user info")
			return setUserErrorState(r, ctx, cr, err)
		}

		currentPolicies := strings.Split(userInfo.PolicyName, ",")
		toDetach, toAttach := arrayDifference(cr.Spec.Policies, currentPolicies)
		if cr.Spec.AccountStatus == "enabled" && len(toDetach) > 0 {
			req := madmin.PolicyAssociationReq{
				Policies: toDetach,
				User:     cr.Spec.AccessKey,
			}
			_, err := adminClient.DetachPolicy(context.Background(), req)
			if err != nil {
				log.Error(err, "Error detaching policies")
				return setUserErrorState(r, ctx, cr, err)
			}
		}
		if cr.Spec.AccountStatus == "enabled" && len(toAttach) > 0 {
			req := madmin.PolicyAssociationReq{
				Policies: toAttach,
				User:     cr.Spec.AccessKey,
			}
			_, err := adminClient.AttachPolicy(context.Background(), req)
			if err != nil {
				log.Error(err, "Error attaching policies")
				return setUserErrorState(r, ctx, cr, err)
			}
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
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&miniov1.User{}).
		Complete(r)
}

// Perform required operations before deleting the CR
func (r *UserReconciler) finalizerOpsForUser(cr *operatorv1.User) error {
	adminClient, err := getAdminClient()
	if err != nil {
		return err
	}

	err = adminClient.RemoveUser(context.Background(), cr.Spec.AccessKey)
	if err != nil {
		if !strings.Contains(err.Error(), "does not exist") {
			return err
		}
	}

	// The following implementation will raise an event
	r.Recorder.Event(cr, "Warning", "Deleting",
		fmt.Sprintf("Custom Resource %s is being deleted from the namespace %s",
			cr.Name,
			cr.Namespace))

	return nil
}

func setUserErrorState(r *UserReconciler, ctx context.Context, cr *operatorv1.User, err error) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	cr.Status.State = typeError
	cr.Status.Message = err.Error()

	if err := r.Status().Update(ctx, cr); err != nil {
		log.Error(err, genericStatusUpdateFailedMessage)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, err
}

func arrayDifference(a []string, b []string) ([]string, []string) {
	var missing []string
	var additional []string

	for _, element := range b {
		if !slices.Contains(a, element) && element != "" {
			missing = append(missing, element)
		}
	}

	for _, element := range a {
		if !slices.Contains(b, element) && element != "" {
			additional = append(additional, element)
		}
	}

	return missing, additional
}
