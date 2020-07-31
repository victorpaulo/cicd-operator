/*


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

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cicdv1alpha1 "github.com/victorpaulo/cicd-operator/api/v1alpha1"
	"github.com/victorpaulo/cicd-operator/helpers"
)

//DeploymentMap map to hold the CIBuild objects
var DeploymentMap = make(map[string]*cicdv1alpha1.CIBuild)

const ciBuildFinalizer = "finalizer.cicd.cicd.com"

// CIBuildReconciler reconciles a CIBuild object
type CIBuildReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=cicd.cicd.com,resources=cibuilds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cicd.cicd.com,resources=cibuildren,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cicd.cicd.com,resources=cibuilds/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

//Reconcile reconcile method
func (r *CIBuildReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("cibuild", req.NamespacedName)

	// Fetch the CIBuild instance
	instance := &cicdv1alpha1.CIBuild{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	err = r.createBuildJobFromTemplate(log, instance)
	if err != nil {
		log.Error(err, "An unexpected error has happen when creating a Job")
		return ctrl.Result{}, err
	}

	// Check if the CIBuild instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isCIBuildMarkedToBeDeleted := instance.GetDeletionTimestamp() != nil
	if isCIBuildMarkedToBeDeleted {
		if contains(instance.GetFinalizers(), ciBuildFinalizer) {
			// Run finalization logic for ciBuildFinalizer. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeCIBuild(log, instance); err != nil {
				return ctrl.Result{}, err
			}

			// Remove ciBuildFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			controllerutil.RemoveFinalizer(instance, ciBuildFinalizer)
			err := r.Update(ctx, instance)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer for this CR
	if !contains(instance.GetFinalizers(), ciBuildFinalizer) {
		if err := r.addFinalizer(log, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("CIBuild reconciliation success")
	return ctrl.Result{}, nil
}

//Creates a Job based on Template for Building from Source Code
func (r *CIBuildReconciler) createBuildJobFromTemplate(log logr.Logger, cr *cicdv1alpha1.CIBuild) error {
	operatorHelper := helpers.NewTemplateHelper(helpers.OperatorParameters{
		ApplicationNamespace: cr.Namespace,
		JobName:              cr.Name,
		ImageDestination:     cr.Spec.ImageDestination,
		SCMRepoURL:           cr.Spec.SCMRepoURL,
		SCMBranch:            cr.Spec.SCMBranch,
		SCMSecret:            cr.Spec.SCMSecret,
		SubContext:           cr.Spec.SubContext,
	})
	job, err := operatorHelper.CreateResource("build")

	if err != nil {
		log.Error(err, "Error happening creating the Job template")
		return err
	}

	found := &batchv1.Job{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}, found)

	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a build Job from template", "job.Namespace", cr.Namespace, "job.Name", cr.Name)
		err = r.Client.Create(context.TODO(), job)

		if err != nil {
			return err
		}

		log.Info(fmt.Sprintf("Defining DeploymentMap for JobName [%s]", cr.Name))
		DeploymentMap[cr.Name] = cr

	}

	return err
}

//SetupWithManager method to watch the resources
func (r *CIBuildReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cicdv1alpha1.CIBuild{}).
		Complete(r)
}

func (r *CIBuildReconciler) finalizeCIBuild(log logr.Logger, cr *cicdv1alpha1.CIBuild) error {
	// TODO(user): Add the cleanup steps that the operator
	// needs to do before the CR can be deleted. Examples
	// of finalizers include performing backups and deleting
	// resources that are not owned by this CR, like a PVC.
	r.deleteJob(log, cr)
	r.deleteDeployment(log, cr)
	log.Info("Successfully finalized CIBuild")
	return nil
}

func (r *CIBuildReconciler) addFinalizer(log logr.Logger, cr *cicdv1alpha1.CIBuild) error {
	log.Info("Adding Finalizer for the CIBuild")
	controllerutil.AddFinalizer(cr, ciBuildFinalizer)

	// Update CR
	err := r.Update(context.TODO(), cr)
	if err != nil {
		log.Error(err, "Failed to update CIBuild with finalizer")
		return err
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (r *CIBuildReconciler) deleteJob(log logr.Logger, cr *cicdv1alpha1.CIBuild) error {
	job := &batchv1.Job{
		TypeMeta: cr.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		},
		Spec: batchv1.JobSpec{},
	}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, job)

	log.Info(fmt.Sprintf("Is the Job [%s] found? Err = [%v]", job.Name, err))

	if err != nil && errors.IsNotFound(err) {
		log.Info(fmt.Sprintf("Job [%s] not found, doing nothing", job.Name))

	} else if err == nil {
		log.Info(fmt.Sprintf("Deleting the Job [%s] ", job.Name))

		//TO-DO: Remove orphan pods after job deletion
		// background := metav1.DeletePropagationBackground
		// deleteOptions := metav1.DeleteOptions{PropagationPolicy: &background}
		// deleteOptions.
		r.Client.Delete(context.TODO(), job)
	}
	return nil
}

func (r *CIBuildReconciler) deleteDeployment(log logr.Logger, cr *cicdv1alpha1.CIBuild) error {
	deployment := &appsv1.Deployment{
		TypeMeta: cr.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deploy-" + cr.Name,
			Namespace: cr.Namespace,
		},
		Spec: appsv1.DeploymentSpec{},
	}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, deployment)

	log.Info(fmt.Sprintf("Is the Deployment [%s] found? Err = [%v]", deployment.Name, err))

	if err != nil && errors.IsNotFound(err) {
		log.Info("Deployment not found, doing nothing")

	} else if err == nil {
		log.Info("Deleting the Deployment")
		r.Client.Delete(context.TODO(), deployment)
	}
	return nil
}
