
# Sample CI/CD Operator - [Medium article](https://medium.com/@victorpaulo/from-zero-to-kubernetes-operator-dd06436b9d89)
This operator was created using Operator-SDK version 1.0.

## Creating your K8s Cluster using KinD
Run the following [bash script](https://gist.github.com/victorpaulo/257c5778ebc0bf92fcd9ac5c75f84d4a) in your environment;

## Creating the Operator
#### create the directory of your project
```
$ mkdir cicd-operator
$ cd cicd-operator
```
### This command will create the structure for your project
`$ operator-sdk init --domain=cicd.com --repo=github.com/victorpaulo/cicd-operator`
### This command will create the CRDs and Controller for your operator
`$ operator-sdk create api --group=cicd --version=v1alpha1 --kind=CIBuild --resource=true --controller=true`

## Implement the logic in this sample or just fork and run it in your own k8s cluster

### Building and Deploying the Operator
```
$ make docker-build docker-push IMG=kind-registry:5000/cicd-operator:v0.0.1
$ make deploy IMG=kind-registry:5000/cicd-operator:v0.0.1
```

## Generating the secret with SSH to connect on Git/Bitbucket
```
$ ssh-keygen -t rsa -N "" -f mykey
#Import the mykey.pub in the GitHub or Bitbucket via web console preferences option.

#Github
$ ssh-keyscan github.com > /tmp/known_hosts

#Bitbucket
$ ssh-keyscan bitbucket.org > /tmp/known_hosts

# Creating the secret
$ kubectl create secret generic git-creds --from-file=ssh=mykey --from-file=known_hosts=/tmp/known_hosts

```

## Creating the CRs
### this CR points to a IBM App Connect (ACE) code
`$ kubectl apply -f config/samples/cicd_v1alpha1_cibuild_ace.yaml`
### this CR points to a Go application
`$ kubectl apply -f config/samples/cicd_v1alpha1_cibuild_go.yaml`
### this CR points to a Nodejs application
`$ kubectl apply -f config/samples/cicd_v1alpha1_cibuild_nodejs.yaml`
### this CR points to a Java application
`$ kubectl apply -f config/samples/cicd_v1alpha1_cibuild_java.yaml`

## Installing Cert Manager in your cluster (Required by the Webhook)

```
$ kubectl apply - validate=false -f https://github.com/jetstack/cert-manager/releases/download/v0.16.0/cert-manager.yaml
```
