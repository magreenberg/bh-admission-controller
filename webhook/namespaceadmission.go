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

// NamespaceAdmission request
type NamespaceAdmission struct {
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
	Kind string
	Name string
	User string
}

const (
	prefix = "namespaceadmission"
)

var (
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
		Name: prefix + "_requests_duration",
		Help: "The durations of requests",
		Buckets: prometheus.LinearBuckets(1, 3, 5),
	})
	externalAPIError = promauto.NewCounter(prometheus.CounterOpts{
		Name: prefix + "_external_api_error",
		Help: "The total number of external API invocations in error",
	})
	externalAPIDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: prefix + "_external_api_duration",
		Help: "The durations of requests",
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

// HandleAdmission invoked when a new namespace or project is created
func (namespaceAdmission *NamespaceAdmission) HandleAdmission(review *v1beta1.AdmissionReview) error {
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

			var ns corev1.Namespace
			if err := json.Unmarshal(request.Object.Raw, &ns); err != nil {
				logrus.Errorln("Failed to unmarshal:", err)
				review.Response = &v1beta1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Message: "Failed!",
					},
				}
				requestsError.Inc()
				return nil
			}
			//logrus.Debugln("Unmarshalled Raw:", ns)

			namespaceName := request.Name
			if len(namespaceName) == 0 {
				// backwards compatibility for OCP 3
				namespaceName = ns.GetName()
				logrus.Debugln("Namespace set to:", namespaceName)
			}

			foundRequester := false
			requester := request.UserInfo.Username
			for key, value := range ns.Annotations {
				// compatibility for OCP "oc new-project <project>"
				if strings.EqualFold("openshift.io/requester", key) {
					requester = value
				} else if strings.EqualFold(namespaceAdmission.RequesterKey, key) {
					foundRequester = true
					logrus.Infoln("Found existing annotation: " + key + " " + value)
				}
			}

			if !foundRequester {
				requestsHandled.Inc()
				logrus.Infoln("Creating annotation: " +
					namespaceAdmission.RequesterKey +
					"=" +
					requester +
					" in " +
					namespaceName)

				annotations := map[string]string{namespaceAdmission.RequesterKey: requester}
				patchBytes, err := createPatch(&ns, annotations)
				if err != nil {
					review.Response = &v1beta1.AdmissionResponse{
						Allowed: true,
						Result: &metav1.Status{
							Message: "createPatch failed",
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

				if len(namespaceAdmission.ExternalAPIURL) > 0 {
					externalValues := &externalValues{
						Kind: requestKind,
						Name: namespaceName,
						User: requester,
					}
					jsonStr, err := json.Marshal(externalValues)
					if err != nil {
						logrus.Errorln("Can't marshal externalValues", err)
					} else {
						startExternalAPITime := time.Now()
						err = invokeexternal(namespaceAdmission.ExternalAPIURL, namespaceAdmission.ExternalAPITimeout, string(jsonStr))
						if err != nil {
							// logrus.Errorln("Invoke external failed:", err)
							externalAPIError.Inc()
						}
						elapsedExternalAPI := time.Since(startExternalAPITime)
						logrus.Debugln("externalAPI elapsed time=", elapsedExternalAPI.Seconds())
						externalAPIDuration.Observe(float64(elapsedExternalAPI.Seconds()))
					}
				}
				elapsed := time.Since(startRequestTime)
				logrus.Debugln("request elapsed time=", elapsed.Seconds())
				requestsDuration.Observe(float64(elapsed.Seconds()))
				return nil
			}
		}
	}

	review.Response = &v1beta1.AdmissionResponse{
		Allowed: true,
		Result: &metav1.Status{
			Message: "SUCCESS",
		},
	}
	return nil
}
