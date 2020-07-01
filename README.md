# namespace-admission-controller

Mutating Admission Controller Webhook

* adds annotation to project/namespace
* invoke external API with requester

Code based on https://github.com/ContainerSolutions/go-validation-admission-controller.git

# Dependencies

* Go >= 1.11
* Kubernetes >= 1.11
* openssl
* oc

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
go test ./...
```

# Container Build

```
REPOSITORY="$(oc get route/default-route -n openshift-image-registry -o=jsonpath='{.spec.host}' 2>/dev/null || oc get route/docker-registry -n default -o=jsonpath='{.spec.host}')/openshift"
docker login -u unused -p $(oc whoami -t) ${REPOSITORY}
docker build -t ${REPOSITORY}/namespace-admission:latest .
docker push ${REPOSITORY}/namespace-admission:latest
```

# Running

```
oc new-project namespace-admission
./gen-cert.sh
./ca-bundle.sh
oc apply -f deploy.yaml
```
