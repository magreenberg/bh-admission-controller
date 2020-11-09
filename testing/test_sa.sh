serviceaccount_LIST="serviceaccount-created-with-oc serviceaccount-from-yaml-1 serviceaccount-created-from-yaml-with-annotation-1"
oc delete serviceaccount ${serviceaccount_LIST}
sleep 10

echo "== serviceaccount-created-with-oc =="
while ! oc create serviceaccount serviceaccount-created-with-oc;do sleep 5;done
#oc get serviceaccount serviceaccount-created-with-oc -o jsonpath='{ .metadata.annotations }' | grep requester

if [ "$#" -eq 1 ];then
	exit 0
fi
echo "== serviceaccount-created-from-yaml-1 =="
while ! oc apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: serviceaccount-from-yaml-1
EOF
do sleep 5;done
#oc get serviceaccount serviceaccount-from-yaml-1 -o jsonpath='{ .metadata.annotations }' | grep requester

echo "== serviceaccount-created-from-yaml-with-annotation-1 =="
while ! oc apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: serviceaccount-created-from-yaml-with-annotation-1
  annotations:
    myannotation: "junk"
    openshift.io/requester: "fred"
EOF
do sleep 5;done
#oc get serviceaccount serviceaccount-from-yaml-1 -o jsonpath='{ .metadata.annotations }'


for i in ${serviceaccount_LIST};do
	echo "=== $i ==="
	oc get serviceaccount $i -o json | jq .metadata.annotations
	echo
done
