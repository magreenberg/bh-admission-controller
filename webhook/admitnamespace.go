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

func admitNamespace(review *v1beta1.AdmissionReview, externalAPIURL string, externalAPITimeout int32, requesterKey string, restConfig restclient.Config) error {
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

	requester := request.UserInfo.Username
	existingAnnotations := ns.Annotations
	// logrus.Println("ns.Annotations=", ns.Annotations)
	if len(ns.Annotations) == 0 {
		// annotations are not included with "Namespace" creation
		coreclient, err := corev1client.NewForConfig(&restConfig)
		if err != nil {
			panic(err)
		}
		nsQuery, err := coreclient.Namespaces().Get(namespaceName, metav1.GetOptions{})
		if err == nil {
			existingAnnotations = nsQuery.Annotations
			// logrus.Debugln("Found existing annotations", existingAnnotations)
		}
	}
	for key, value := range existingAnnotations {
		// compatibility for OCP "oc new-project <project>"
		if strings.EqualFold("openshift.io/requester", key) {
			requester = value
			logrus.Debugln("User set to:", requester)
			// continue check whether the requesterKey exists
		} else if strings.EqualFold(requesterKey, key) {
			logrus.Infoln("Ignoring review as existing annotation found: " + key + "=" + value)
			return nil
		}
	}

	requestsHandled.Inc()
	namespaceRequestsHandled.Inc()
	logrus.Infoln("Creating annotation: " +
		requesterKey + "=" + requester +
		" for namespace/project: " + namespaceName)

	newAnnotation := map[string]string{requesterKey: requester}
	patchBytes, err := createPatch(&ns, newAnnotation)
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
	err = prepareAndInvokeExternal(externalAPIURL, externalAPITimeout, requestKind, namespaceName, requester)
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
