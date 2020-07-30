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
	"github.com/victorpaulo/cicd-operator/api/v1alpha1"
	"github.com/victorpaulo/cicd-operator/helpers"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CDeployReconciler reconciles a CDeploy object
type CDeployReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=cicd.cicd.com,resources=cdeploys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cicd.cicd.com,resources=cdeploys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

//Reconcile reconciles loop
func (r *CDeployReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("cdeploy", req.NamespacedName)

	job := &batchv1.Job{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, job)

	if err != nil {
		return ctrl.Result{}, err
	}

	if job != nil {
		if len(job.Status.Conditions) > 0 && (job.Status.Conditions[0].Type == batchv1.JobComplete) {

			// log.Info(fmt.Sprintf("[CIDEPLOY] - Job found [%v]", job))
			ciBuild := DeploymentMap[job.Name]
			if ciBuild != nil {
				// log.Info(fmt.Sprintf("[CIBUILD] - found [%v]", ciBuild))

				err = r.createDeploymentFromTemplate(ciBuild)

				if err != nil {
					log.Error(err, "An unexpected error happening creating the Deployment")
					return ctrl.Result{}, err
				}
				//Delete the job to avoid a new deployment
				delete(DeploymentMap, job.Name)
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *CDeployReconciler) createDeploymentFromTemplate(cr *v1alpha1.CIBuild) error {
	operatorHelper := helpers.NewTemplateHelper(helpers.OperatorParameters{
		ApplicationNamespace: cr.Namespace,
		JobName:              cr.Name,
		ImageDestination:     cr.Spec.ImageDestination,
		SCMRepoURL:           cr.Spec.SCMRepoURL,
		SCMBranch:            cr.Spec.SCMBranch,
		SCMSecret:            cr.Spec.SCMSecret,
		SubContext:           cr.Spec.SubContext,
	})
	deploy, err := operatorHelper.CreateResource("deploy")

	log.Info(fmt.Sprintf("Deployment obj from Template [%s]", deploy))

	if err != nil {
		log.Error(err, "Error happening creating a deployment template")
		return err
	}

	found := &appsv1.Deployment{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}, found)

	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a Deployment from a template", "deployment.Namespace", cr.Namespace, "deployment.Name", "test")
		err = r.Client.Create(context.TODO(), deploy)

		if err != nil {
			return err
		}
	}

	return err
}

func (r *CDeployReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.Job{}).
		Complete(r)
}
