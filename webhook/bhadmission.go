package webhook

import (
	"encoding/json"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"time"
)

// BhAdmission request
type BhAdmission struct {
	ExternalAPIURL     string
	ExternalAPITimeout int32
	RequesterKey       string
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

type externalValues struct {
	Kind        string
	Namespace   string
	AccountName string
}

const (
	prefix = "BhAdmission"
)

var (

	// TODO - add counters for user/serviceaccounts
	requestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: prefix + "_requests_total",
		Help: "The total number of processed requests",
	})
	requestsHandled = promauto.NewCounter(prometheus.CounterOpts{
		Name: prefix + "_requests_handled",
		Help: "The total number of processed requests",
	})
	requestsError = promauto.NewCounter(prometheus.CounterOpts{
		Name: prefix + "_requests_error",
		Help: "The total number of requests in error",
	})
	requestsDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    prefix + "_requests_duration",
		Help:    "The durations of requests",
		Buckets: prometheus.LinearBuckets(1, 3, 5),
	})
	externalAPIError = promauto.NewCounter(prometheus.CounterOpts{
		Name: prefix + "_external_api_error",
		Help: "The total number of external API invocations in error",
	})
	externalAPIDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    prefix + "_external_api_duration",
		Help:    "The durations of requests",
		Buckets: prometheus.LinearBuckets(1, 3, 5),
	})
)

func updateAnnotation(target map[string]string, added map[string]string) (patch []patchOperation) {
	for key, value := range added {
		if target == nil || target[key] == "" {
			target = map[string]string{}
			patch = append(patch, patchOperation{
				Op:   "add",
				Path: "/metadata/annotations",
				Value: map[string]string{
					key: value,
				},
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  "/metadata/annotations/" + key,
				Value: value,
			})
		}
	}
	return patch
}

// create mutation patch for resoures
func createPatch(ns *corev1.Namespace, annotations map[string]string) ([]byte, error) {
	var patch []patchOperation

	patch = append(patch, updateAnnotation(ns.Annotations, annotations)...)

	return json.Marshal(patch)
}

func prepareAndInvokeExternal(externalAPIURL string, externalAPITimeout int32, requestKind string, namespace string, accountName string) error {
	var err error
	if len(externalAPIURL) > 0 {
		externalValues := &externalValues{
			Kind:        requestKind,
			Namespace:   namespace,
			AccountName: accountName,
		}
		jsonStr, err := json.Marshal(externalValues)
		if err != nil {
			logrus.Errorln("Can't marshal externalValues", err)
		} else {
			startExternalAPITime := time.Now()
			err = invokeexternal(externalAPIURL, externalAPITimeout, string(jsonStr))
			if err != nil {
				// logrus.Errorln("Invoke external failed:", err)
				externalAPIError.Inc()
			}
			elapsedExternalAPI := time.Since(startExternalAPITime)
			// logrus.Debugln("externalAPI elapsed time=", elapsedExternalAPI.Seconds())
			externalAPIDuration.Observe(float64(elapsedExternalAPI.Seconds()))
		}
	}
	return err
}

func admitNamespace(review *v1beta1.AdmissionReview, externalAPIURL string, externalAPITimeout int32, requesterKey string) error {
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
				Status: "Failure",
				Message: "Failed to unmarshal: " + err.Error(),
			},
		}
		requestsError.Inc()
		return nil
	}
	// logrus.Debugln("Unmarshalled Raw:", ns)

	namespaceName := request.Name
	if len(namespaceName) == 0 {
		// backwards compatibility for OCP 3
		namespaceName = ns.GetName()
		logrus.Debugln("Namespace set to:", namespaceName)
	}

	requester := request.UserInfo.Username
	for key, value := range ns.Annotations {
		// compatibility for OCP "oc new-project <project>"
		if strings.EqualFold("openshift.io/requester", key) {
			requester = value
			logrus.Debugln("User set to:", requester)
			break
		} else if strings.EqualFold(requesterKey, key) {
			logrus.Infoln("Ignoring review as existing annotation found: " + key + "=" + value)
			return nil
		}
	}

	requestsHandled.Inc()
	logrus.Infoln("Creating annotation: " +
		requesterKey + "=" + requester +
		" for namespace/project: " + namespaceName)

	annotations := map[string]string{requesterKey: requester}
	patchBytes, err := createPatch(&ns, annotations)
	if err != nil {
		review.Response = &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Status: "Failure",
				Message: "createPatch failed: " + err.Error(),
			},
		}
		requestsError.Inc()
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
				Status: "Failure", 
				Message: "invokeExternal failed: " + err.Error(),
			},
		}
		requestsError.Inc()
	}
	return nil
}

func admitUserSA(review *v1beta1.AdmissionReview, externalAPIURL string, externalAPITimeout int32) error {
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
					Status: "Failure",
					Message: "Failed to unmarshal service account information:" + err.Error(),
				},
			}
			requestsError.Inc()
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
			Status: "Success",
		},
	}

	if strings.EqualFold("ServiceAccount", requestKind) {
		// ignore ServiceAccounts created automatically during project/namespace creation
		if strings.EqualFold("system:serviceaccount:openshift-infra:serviceaccount-controller", request.UserInfo.Username) ||
			strings.EqualFold("system:serviceaccount:kube-system:service-account-controller", request.UserInfo.Username) {
			logrus.Debugln("Ignoring automatically generated service account:", requestName)
			return nil
		}
	}

	err := prepareAndInvokeExternal(externalAPIURL, externalAPITimeout, requestKind, request.Namespace, requestName)
	if err != nil {
		review.Response = &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Status: "Failure", 
				Message: "invokeExternal failed: " + err.Error(),
			},
		}
		requestsError.Inc()
	}
	return nil
}

// HandleAdmission invoked when a new namespace or project is created
func (bhAdmission *BhAdmission) HandleAdmission(review *v1beta1.AdmissionReview) error {
	logrus.WithFields(logrus.Fields{
		"Operation": review.Request.Operation,
		"Kind":      review.Request.Kind.Kind,
		"Name":      review.Request.Name,
		"User":      review.Request.UserInfo.Username,
	}).Info("NEW REQUEST for HandleAdmission")

	request := review.Request
	reqKind := request.Kind
	if request.Operation == v1beta1.Create {
		requestKind := reqKind.Kind
		if strings.EqualFold("Namespace", requestKind) ||
			strings.EqualFold("Project", requestKind) {
			requestsTotal.Inc()
			startRequestTime := time.Now()
			_ = admitNamespace(review, bhAdmission.ExternalAPIURL, bhAdmission.ExternalAPITimeout, bhAdmission.RequesterKey)
			elapsed := time.Since(startRequestTime)
			// logrus.Debugln("request elapsed time=", elapsed.Seconds())
			requestsDuration.Observe(float64(elapsed.Seconds()))
		} else if strings.EqualFold("User", requestKind) ||
			strings.EqualFold("ServiceAccount", requestKind) {
			requestsTotal.Inc()
			startRequestTime := time.Now()
			_ = admitUserSA(review, bhAdmission.ExternalAPIURL, bhAdmission.ExternalAPITimeout)
			elapsed := time.Since(startRequestTime)
			// logrus.Debugln("request elapsed time=", elapsed.Seconds())
			requestsDuration.Observe(float64(elapsed.Seconds()))
		} else {
			logrus.Debug("Ignoring AdmissingRequest for type:", reqKind.Kind)
		}
	}
	return nil
}
