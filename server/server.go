package server

import (
	"crypto/tls"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/json"
	"net/http"
)

// AdmissionController interface
type AdmissionController interface {
	HandleAdmission(review *v1beta1.AdmissionReview) error
}

// AdmissionControllerServer struct
type AdmissionControllerServer struct {
	AdmissionController AdmissionController
	Decoder             runtime.Decoder
}

func (acs *AdmissionControllerServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// logrus.Debugln("ServerHTTP new request: ", r)
	var body []byte
	if data, err := ioutil.ReadAll(r.Body); err == nil {
		body = data
	}
	// logrus.Debugln("RequestBody: ", body)
	review := &v1beta1.AdmissionReview{}
	_, _, err := acs.Decoder.Decode(body, nil, review)
	if err != nil {
		logrus.Errorln("Can't decode request", err)
	}
	// logrus.Debugln("AdmissionReview: ", review)
	// ignore errors for now
	_ = acs.AdmissionController.HandleAdmission(review)
	if responseInBytes, err := json.Marshal(review); err != nil {
		logrus.Errorln("Can't marshal request", err)
	} else {
		if _, err := w.Write(responseInBytes); err != nil {
			logrus.Errorln("Failed to write response", err)
		}
	}
}

// GetAdmissionServerNoSSL function
func GetAdmissionServerNoSSL(ac AdmissionController, listenOn string) *http.Server {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)
	server := &http.Server{
		Handler: &AdmissionControllerServer{
			AdmissionController: ac,
			Decoder:             codecs.UniversalDeserializer(),
		},
		Addr: listenOn,
	}
	return server
}

// GetAdmissionValidationServer function
func GetAdmissionValidationServer(ac AdmissionController, tlsCert, tlsKey, listenOn string) *http.Server {
	sCert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
	server := GetAdmissionServerNoSSL(ac, listenOn)
	server.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{sCert},
	}
	if err != nil {
		logrus.Error(err)
	}
	return server
}
