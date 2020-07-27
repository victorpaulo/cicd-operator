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

	"github.com/cloudflare/cfssl/log"
	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cicdv1alpha1 "github.com/victorpaulo/cicd-operator/api/v1alpha1"
	"github.com/victorpaulo/cicd-operator/helpers"
)

//DeploymentMap map to hold the CIBuild objects
var DeploymentMap = make(map[string]*cicdv1alpha1.CIBuild)

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
	_ = context.Background()
	_ = r.Log.WithValues("cibuild", req.NamespacedName)

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

	err = r.createBuildJobFromTemplate(instance)
	if err != nil {
		log.Error(err, "An unexpected error has happen when creating a Job")
		return ctrl.Result{}, err
	}

	log.Info("CIBuild reconciliation success")
	return ctrl.Result{}, nil
}

//Creates a Job based on Template for Building from Source Code
func (r *CIBuildReconciler) createBuildJobFromTemplate(cr *cicdv1alpha1.CIBuild) error {
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
