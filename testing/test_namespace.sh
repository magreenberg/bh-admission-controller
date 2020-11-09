NS_LIST="namespace-created-with-oc namespace-from-yaml-1 namespace-created-from-yaml-with-annotation-1 project-created-with-oc project-from-yaml-1 project-created-from-yaml-with-annotation-1"
oc delete project ${NS_LIST}
sleep 10

echo "== namespace-created-with-oc =="
while ! oc create namespace namespace-created-with-oc;do sleep 5;done
#oc get namespace namespace-created-with-oc -o jsonpath='{ .metadata.annotations }' | grep requester

if [ "$#" -eq 1 ];then
	exit 0
fi
echo "== namespace-created-from-yaml-1 =="
while ! oc apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: namespace-from-yaml-1
EOF
do sleep 5;done
#oc get namespace namespace-from-yaml-1 -o jsonpath='{ .metadata.annotations }' | grep requester

echo "== namespace-created-from-yaml-with-annotation-1 =="
while ! oc apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: namespace-created-from-yaml-with-annotation-1
  annotations:
    myannotation: "junk"
    openshift.io/requester: "fred"
EOF
do sleep 5;done
#oc get namespace namespace-from-yaml-1 -o jsonpath='{ .metadata.annotations }'



echo "== project-created-with-oc =="
while ! oc new-project project-created-with-oc;do sleep 5;done
#oc get project project-created-with-oc -o jsonpath='{ .metadata.annotations }' | grep requester

echo "== project-from-yaml-1 =="
while ! oc apply -f - <<EOF
apiVersion: v1
kind: Project
metadata:
  name: project-from-yaml-1
EOF
do sleep 5;done
#oc get project project-from-yaml-1 -o jsonpath='{ .metadata.annotations }' | grep requester

echo "== project-created-from-yaml-with-annotation-1 =="
while ! oc apply -f - <<EOF
apiVersion: v1
kind: Project
metadata:
  name: project-created-from-yaml-with-annotation-1
  annotations:
    myannotation: "junk"
    openshift.io/requester: "fred"
EOF
do sleep 5;done
#oc get project project-from-yaml-1 -o jsonpath='{ .metadata.annotations }'


for i in ${NS_LIST};do
	echo "=== $i ==="
	oc get project $i -o json | jq .metadata.annotations
	echo
done
