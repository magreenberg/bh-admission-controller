USER_LIST="user-created-with-oc user-from-yaml-1 user-created-from-yaml-with-annotation-1"
oc delete user ${USER_LIST}
sleep 10

echo "== user-created-with-oc =="
while ! oc create user user-created-with-oc;do sleep 5;done
#oc get user user-created-with-oc -o jsonpath='{ .metadata.annotations }' | grep requester

if [ "$#" -eq 1 ];then
	exit 0
fi
echo "== user-created-from-yaml-1 =="
while ! oc apply -f - <<EOF
apiVersion: user.openshift.io/v1
kind: User
metadata:
  name: user-from-yaml-1
EOF
do sleep 5;done
#oc get user user-from-yaml-1 -o jsonpath='{ .metadata.annotations }' | grep requester

echo "== user-created-from-yaml-with-annotation-1 =="
while ! oc apply -f - <<EOF
apiVersion: user.openshift.io/v1
kind: User
metadata:
  name: user-created-from-yaml-with-annotation-1
  annotations:
    myannotation: "junk"
    openshift.io/requester: "fred"
EOF
do sleep 5;done
#oc get user user-from-yaml-1 -o jsonpath='{ .metadata.annotations }'


for i in ${USER_LIST};do
	echo "=== $i ==="
	oc get user $i -o json | jq .metadata.annotations
	echo
done
