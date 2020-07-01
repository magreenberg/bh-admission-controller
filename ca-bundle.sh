#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

# choose kubectl or oc
if kubectl version > /dev/null 2>&1; then
    KUBECTL="kubectl"
elif oc version > /dev/null 2>&1;then
    KUBECTL="oc"
else
    echo "Either \"kubectl\" or \"oc\" must be installed"
    exit 1
fi

export CA_BUNDLE=$(${KUBECTL} get configmap -n kube-system extension-apiserver-authentication -o=jsonpath='{.data.client-ca-file}' | base64 | tr -d '\n')

sed -i "s/caBundle: .*$/caBundle: ${CA_BUNDLE}/g" ./deploy.yaml
