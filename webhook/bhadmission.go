package webhook

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"runtime/debug"
	"strings"
	"time"

	restclient "k8s.io/client-go/rest"
)

// BhAdmission request
type BhAdmission struct {
	ExternalAPIURL     string
	ExternalAPITimeout int32
	RequesterKey       string
	RestConfig         restclient.Config
	ClusterName        string
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
	prefix          = "bhadmission"
	prefixNamespace = "namespace"
	prefixAccount   = "account"
)

var (
	requestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: prefix + "_requests_total",
		Help: "The total number of processed requests",
	})
	namespaceRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: prefix + "_" + prefixNamespace + "_requests_total",
		Help: "The total number of processed namespace requests",
	})
	accountRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: prefix + "_" + prefixAccount + "_requests_total",
		Help: "The total number of processed account requests",
	})
	requestsHandled = promauto.NewCounter(prometheus.CounterOpts{
		Name: prefix + "_requests_handled",
		Help: "The total number of processed requests",
	})
	namespaceRequestsHandled = promauto.NewCounter(prometheus.CounterOpts{
		Name: prefix + "_" + prefixNamespace + "_requests_handled",
		Help: "The total number of processed namespace requests",
	})
	accountRequestsHandled = promauto.NewCounter(prometheus.CounterOpts{
		Name: prefix + "_" + prefixAccount + "_requests_handled",
		Help: "The total number of processed account requests",
	})
	requestsError = promauto.NewCounter(prometheus.CounterOpts{
		Name: prefix + "_requests_error",
		Help: "The total number of requests in error",
	})
	namespaceRequestsError = promauto.NewCounter(prometheus.CounterOpts{
		Name: prefix + "_" + prefixNamespace + "_requests_error",
		Help: "The total number of namespace requests in error",
	})
	accountRequestsError = promauto.NewCounter(prometheus.CounterOpts{
		Name: prefix + "_" + prefixAccount + "_requests_error",
		Help: "The total number of accounts requests in error",
	})
	requestsDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    prefix + "_requests_duration",
		Help:    "The durations of all requests",
		Buckets: prometheus.LinearBuckets(1, 3, 5),
	})
	namespaceRequestsDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    prefix + "_" + prefixNamespace + "_requests_duration",
		Help:    "The durations of namespace requests",
		Buckets: prometheus.LinearBuckets(1, 3, 5),
	})
	accountRequestsDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    prefix + "_" + prefixAccount + "_requests_duration",
		Help:    "The durations of account requests",
		Buckets: prometheus.LinearBuckets(1, 3, 5),
	})
	externalAPIError = promauto.NewCounter(prometheus.CounterOpts{
		Name: prefix + "_external_api_error",
		Help: "The total number of external API invocations in error",
	})
	externalAPIDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    prefix + "_external_api_duration",
		Help:    "The durations of external API invocations",
		Buckets: prometheus.LinearBuckets(1, 3, 5),
	})
)

// HandleAdmission invoked when a new namespace or project is created
func (bhAdmission *BhAdmission) HandleAdmission(review *v1beta1.AdmissionReview) error {
	defer func() {
		if r := recover(); r != nil {
			logrus.Error("Recovering from panic:\n", string(debug.Stack()))
			review.Response = &v1beta1.AdmissionResponse{
				Allowed: true,
				Result: &metav1.Status{
					Status:  metav1.StatusFailure,
					Message: "Internal error",
				},
			}
			return
		}
	}()

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
			namespaceRequestsTotal.Inc()
			startRequestTime := time.Now()
			_ = admitNamespace(review, bhAdmission.ExternalAPIURL, bhAdmission.ExternalAPITimeout, bhAdmission.RequesterKey, bhAdmission.RestConfig)
			elapsed := time.Since(startRequestTime)
			// logrus.Debugln("request elapsed time=", elapsed.Seconds())
			requestsDuration.Observe(float64(elapsed.Seconds()))
			namespaceRequestsDuration.Observe(float64(elapsed.Seconds()))
		} else if strings.EqualFold("User", requestKind) ||
			strings.EqualFold("ServiceAccount", requestKind) {
			requestsTotal.Inc()
			startRequestTime := time.Now()
			accountRequestsTotal.Inc()
			_ = admitAccount(review, bhAdmission.ExternalAPIURL, bhAdmission.ExternalAPITimeout, bhAdmission.RestConfig)
			elapsed := time.Since(startRequestTime)
			// logrus.Debugln("request elapsed time=", elapsed.Seconds())
			requestsDuration.Observe(float64(elapsed.Seconds()))
			accountRequestsDuration.Observe(float64(elapsed.Seconds()))
		} else {
			logrus.Debug("Ignoring AdmissingRequest for type:", reqKind.Kind)
		}
	}
	return nil
}
