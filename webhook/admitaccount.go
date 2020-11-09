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

	if len(requestName) == 0 {
		// backwards compatibility for OCP 3
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
		// logrus.Debugln("Unmarshalled Raw:", sa)
		requestName = sa.GetName()
		logrus.Debugln("Name set to:", requestName)
	}

	// Ensure that the request will succeed
	review.Response = &v1beta1.AdmissionResponse{
		Allowed: true,
		Result: &metav1.Status{
			Status: metav1.StatusSuccess,
		},
	}

	if strings.EqualFold("ServiceAccount", requestKind) {
		// ignore ServiceAccounts created automatically during project/namespace creation
		if strings.EqualFold("system:serviceaccount:openshift-infra:serviceaccount-controller", request.UserInfo.Username) ||
			strings.EqualFold("system:serviceaccount:kube-system:service-account-controller", request.UserInfo.Username) {
			logrus.Debugln("Ignoring automatically generated service account:", requestName)
			return nil
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
	}

	requestsHandled.Inc()
	accountRequestsHandled.Inc()
	err := prepareAndInvokeExternal(externalAPIURL, externalAPITimeout, requestKind, request.Namespace, requestName, clusterName)
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
	return nil
}
