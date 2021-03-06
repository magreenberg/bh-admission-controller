package webhook

import (
	"encoding/json"
	userv1client "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"strings"
)

func admitAccount(review *v1beta1.AdmissionReview, externalAPIURL string, externalAPITimeout int32, restConfig restclient.Config, clusterName string) error {
	request := review.Request
	requestKind := request.Kind.Kind
	requestName := request.Name
	requester := request.UserInfo.Username
	identifierType := "sa"
	var err error
	var patchBytes []byte

	newAnnotations := map[string]string{
		"bnhp.com/requester": requester,
		"bnhp.cloudia/owner": requester,
		"bnhp.cloudia/env":   "build",
	}

	if strings.EqualFold("ServiceAccount", requestKind) {
		// ignore ServiceAccounts created automatically during project/namespace creation
		if strings.EqualFold("system:serviceaccount:openshift-infra:serviceaccount-controller", request.UserInfo.Username) ||
			strings.EqualFold("system:serviceaccount:kube-system:service-account-controller", request.UserInfo.Username) {
			logrus.Debugln("Ignoring automatically generated service account:", requestName)
			return nil
		}
		var sa corev1.ServiceAccount
		if err := json.Unmarshal(request.Object.Raw, &sa); err != nil {
			logrus.Errorln("Failed to unmarshal service account information:", err)
			review.Response = &v1beta1.AdmissionResponse{
				Allowed: true,
				Result: &metav1.Status{
					Status:  metav1.StatusFailure,
					Message: "Failed to unmarshal service account information:" + err.Error(),
				},
			}
			requestsError.Inc()
			accountRequestsError.Inc()
			return nil
		}
		if len(requestName) == 0 {
			// backwards compatibility for OCP 3

			// logrus.Debugln("Unmarshalled Raw:", sa)
			requestName = sa.GetName()
			logrus.Debugln("Name set to:", requestName)
		}
		coreclient, err := corev1client.NewForConfig(&restConfig)
		if err != nil {
			panic(err)
		}
		_, err = coreclient.ServiceAccounts(request.Namespace).Get(requestName, metav1.GetOptions{})
		if err == nil {
			logrus.WithFields(logrus.Fields{
				"Namespace":      request.Namespace,
				"ServiceAccount": requestName,
			}).Info("Ignoring CREATE request for existing service account")
			return nil
		}
		patchBytes, err = createPatch(sa.Annotations, newAnnotations)
	} else {
		// check for existing entry with the same name
		if strings.EqualFold("User", requestKind) {
			userClient, err := userv1client.NewForConfig(&restConfig)
			if err != nil {
				panic(err)
			}
			_, err = userClient.Users().Get(requestName, metav1.GetOptions{})
			if err == nil {
				logrus.Info("Ignoring CREATE request for existing user:", requestName)
				return nil
			}
		}
		identifierType = "user"

		// temporarily unmarshal as service account to prevent the need for OpenShift includes
		var sa corev1.ServiceAccount
		if err := json.Unmarshal(request.Object.Raw, &sa); err != nil {
			logrus.Errorln("Failed to unmarshal user information:", err)
			review.Response = &v1beta1.AdmissionResponse{
				Allowed: true,
				Result: &metav1.Status{
					Status:  metav1.StatusFailure,
					Message: "Failed to unmarshal user information:" + err.Error(),
				},
			}
			requestsError.Inc()
			accountRequestsError.Inc()
			return nil
		}
		// TODO - check whether annotations can be passed when creating user
		patchBytes, err = createPatch(sa.Annotations, newAnnotations)
	}

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

	identifier := request.Namespace + "-" + requestName
	err = prepareAndInvokeExternal(externalAPIURL, externalAPITimeout, identifierType, identifier, clusterName)
	if err != nil {
		review.Response = &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Status:  metav1.StatusFailure,
				Message: "invokeExternal failed: " + err.Error(),
			},
		}
		requestsError.Inc()
		accountRequestsError.Inc()
	}

	requestsHandled.Inc()
	accountRequestsHandled.Inc()

	logrus.Debugln("AdmissionResponse:", string(patchBytes))
	review.Response = &v1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *v1beta1.PatchType {
			pt := v1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}
	return nil
}
