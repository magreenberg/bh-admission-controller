package webhook

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"strings"
)

func admitNamespace(review *v1beta1.AdmissionReview, externalAPIURL string, externalAPITimeout int32, restConfig restclient.Config, clusterName string) error {
	var err error
	request := review.Request
	reqKind := request.Kind
	requestKind := reqKind.Kind
	var ns corev1.Namespace
	if err = json.Unmarshal(request.Object.Raw, &ns); err != nil {
		logrus.Errorln("Failed to unmarshal:", err)
		review.Response = &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Status:  metav1.StatusFailure,
				Message: "Failed to unmarshal: " + err.Error(),
			},
		}
		requestsError.Inc()
		namespaceRequestsError.Inc()
		return nil
	}
	// logrus.Debugln("Unmarshalled ns:", ns)

	namespaceName := request.Name
	if len(namespaceName) == 0 {
		// backwards compatibility for OCP 3
		namespaceName = ns.GetName()
		logrus.Debugln("Namespace set to:", namespaceName)
	}

	// ignore existing objects

	// A creationTimestamp in the request signifies an existing object
	//logrus.Debugln("ns.ObjectMeta.GetCreationTimestamp=", ns.ObjectMeta.GetCreationTimestamp())
	t := ns.ObjectMeta.GetCreationTimestamp()
	if !t.IsZero() {
		logrus.Info("Inoring create request for project/namespace with creationTime:", namespaceName)
		return nil
	}

	// Check whether the object exists
	coreclient, err := corev1client.NewForConfig(&restConfig)
	if err != nil {
		panic(err)
	}
	_, err = coreclient.Namespaces().Get(namespaceName, metav1.GetOptions{})
	if err == nil {
		logrus.Info("Inoring create request for existing project/namespace:", namespaceName)
		return nil
	}

	requester := request.UserInfo.Username
	requestedAnnotations := ns.Annotations
	// // logrus.Println("ns.Annotations=", ns.Annotations)
	// if len(ns.Annotations) == 0 {
	// 	// annotations are not included with "Namespace" creation
	// 	coreclient, err := corev1client.NewForConfig(&restConfig)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	nsQuery, err := coreclient.Namespaces().Get(namespaceName, metav1.GetOptions{})
	// 	if err == nil {
	// 		requestedAnnotations = nsQuery.Annotations
	// 		// logrus.Debugln("Found existing annotations", requestedAnnotations)
	// 	}
	// }
	for key, value := range requestedAnnotations {
		// compatibility for OCP "oc new-project <project>"
		if strings.EqualFold("openshift.io/requester", key) {
			requester = value
			logrus.WithFields(logrus.Fields{
				"from request.UserInfo.Username":     request.UserInfo.Username,
				"to provided openshift.io/requester": requester,
			}).Debugln("requester changed")
			// continue check whether the requesterKey exists
		}
	}

	requestsHandled.Inc()
	namespaceRequestsHandled.Inc()

	// logrus.Infoln("Creating annotations: " +
	// 	requesterKey + "=" + requester +
	// 	" for namespace/project: " + namespaceName)

	newAnnotations := map[string]string{
		"bnhp.com/requester": requester,
		"bnhp.cloudia/owner": requester,
		"bnhp.cloudia/env":   "build",
	}

	finalAnnotations := mergeAnnotations(requestedAnnotations, newAnnotations)

	patchBytes, err := createPatch(&ns, finalAnnotations)
	if err != nil {
		review.Response = &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Status:  metav1.StatusFailure,
				Message: "createPatch failed: " + err.Error(),
			},
		}
		requestsError.Inc()
		namespaceRequestsError.Inc()
		return nil
	}

	logrus.Debugln("AdmissionResponse:", string(patchBytes))
	review.Response = &v1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *v1beta1.PatchType {
			pt := v1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}
	err = prepareAndInvokeExternal(externalAPIURL, externalAPITimeout, requestKind, namespaceName, requester, clusterName)
	if err != nil {
		review.Response = &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Status:  metav1.StatusFailure,
				Message: "invokeExternal failed: " + err.Error(),
			},
		}
		requestsError.Inc()
		namespaceRequestsError.Inc()
	}
	return nil
}
