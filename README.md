# bh-admission-controller

Mutating Admission Controller Webhook

* Adds annotation to project/namespace
* Invokes external API with the username of the requester

Code based on https://github.com/ContainerSolutions/go-validation-admission-controller.git

# Dependencies

* Go >= 1.11
* Kubernetes >= 1.11
* openssl
* oc or kubectl

# Configuration

If AdmissionWebhooks are not enabled, add the following to /etc/origin/master/master-config.yaml:
```
admissionConfig:
  pluginConfig:
    ValidatingAdmissionWebhook: 
      configuration:
        kind: DefaultAdmissionConfig
        apiVersion: v1
        disable: false 
    MutatingAdmissionWebhook: 
      configuration:
        kind: DefaultAdmissionConfig
        apiVersion: v1
        disable: false 
```
Then run:
```
    master-restart api
    master-restart controllers
```

# Testing

```
    $ go test ./...
```

# Container Build

## Building the Container Image

Create an environment variable REGISTRY with the URL to your container registry.
Build the image using either the Red Hat UBI base image of the golang image.

### Network Connected Go Build Using Red Hat UBI
```
    $ docker build -t ${REGISTRY}/bh-admission:latest .
```
### Network Disconnected Go Build Using golang:1.12.4-alpine
```
    $ docker build -f Dockerfile.golang-1.12.4-alpine -t ${REGISTRY}/bh-admission:latest .
```
Alternatively, create a debug build by running:
```
    $ docker build -f Dockerfile.golang-1.12.4-alpine -t ${REGISTRY}/bh-admission:latest -f Dockerfile.golang-1.12.4-alpine.debug .
```
## Push
Push the image to your registry.
```
    $ docker push ${REGISTRY}/bh-admission:latest
```
# Running

## Image Name
Before running the deployment:
- Update the "image:" line in the file deploy.yaml to match your ${REGISTRY} above.
- Update the vales in the configmap.yaml file

## Run Commands
Run the following commands:
```
    $ oc new-project bh-admission
    $ ./gen-cert.sh
    $ ./ca-bundle.sh
    $ oc apply -f configmap.yaml
    $ oc apply -f deploy.yaml
```

## Testing
First watch for the running pod. For example:
```
    $ oc get pods
    NAME                                  READY   STATUS    RESTARTS   AGE
    bh-admission-789846c97-kqm6v   1/1     Running   0          7s
```

Create a project or namespace. For example:
```
    $ oc new-project mynewproject
```

Check for "mycompany.com/requester" the annotation. For example:
```
    $ oc get project mynewproject -o jsonpath='{ .metadata.annotations }' 
    map[mycompany.com/requester:kube:admin
    openshift.io/display-name: 
    openshift.io/sa.scc.mcs:s0:c25,c0
    openshift.io/sa.scc.supplemental-groups:1000600000/10000
    openshift.io/sa.scc.uid-range:1000600000/10000]
```

# Tuning
A ConfigMap is created with the following default values:
```
    external_api_url=https://localhost:8080
    external_api_timeout=10
    requester_key=mycompany.com/requester
    listen_addr=0.0.0.0:8080
```
The values can be updated in the deploy.yaml file.

# Cleanup
Run the following commands to delete objects created:
```
    $ oc delete deployment bh-admission -n bh-admission
    $ oc delete MutatingWebhookConfiguration/bh-admission
    $ oc delete project bh-admission
    $ oc delete csr/bh-admission.bh-admission
    $ oc delete project mynewproject
```

# Debugging
## Port Forwarding
Port forward from the development host to port 40000 on the pod
```
    $ oc get pods
    $ oc port-forward podname 40000

```
## JSON request
The following example, run from within a pod in the same namespace, will simulate an admission request:
```
$ curl -d @- -k https://bh-admission <<EOF
{"kind":"AdmissionReview","apiVersion":"admission.k8s.io/v1beta1","request":{"uid":"b1b2eb30-5f71-4f39-831c-00395af68ccd","kind":{"group":"","version":"v1","kind":"Namespace"},"resource":{"group":"","version":"v1","resource":"namespaces"},"requestKind":{"group":"","version":"v1","kind":"Namespace"},"requestResource":{"group":"","version":"v1","resource":"namespaces"},"name":"junk8","operation":"CREATE","userInfo":{"username":"michael","groups":["system:cluster-admins","system:authenticated"],"extra":{"scopes.authorization.openshift.io":["user:full"]}},"object":{"kind":"Namespace","apiVersion":"v1","metadata":{"name":"junk8","creationTimestamp":null,"managedFields":[{"manager":"oc","operation":"Update","apiVersion":"v1","time":"2020-11-02T09:21:18Z","fieldsType":"FieldsV1","fieldsV1":{"f:status":{"f:phase":{}}}}]},"spec":{},"status":{"phase":"Active"}},"oldObject":null,"dryRun":false,"options":{"kind":"CreateOptions","apiVersion":"meta.k8s.io/v1"}},"response":{"uid":"","allowed":true,"patch":"W3sib3AiOiJhZGQiLCJwYXRoIjoiL21ldGFkYXRhL2Fubm90YXRpb25zIiwidmFsdWUiOnsiYm5ocC5jbG91ZGlhL2VudiI6ImJ1aWxkIiwiYm5ocC5jbG91ZGlhL293bmVyIjoibWljaGFlbCIsIm15Y29tcGFueS5jb20vcmVxdWVzdGVyIjoibWljaGFlbCJ9fV0=","patchType":"JSONPatch"}}
EOF
```