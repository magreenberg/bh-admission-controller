package main

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"namespace-admission-controller/server"
	"namespace-admission-controller/webhook"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var (
	admissionRequestNS = v1beta1.AdmissionReview{
		TypeMeta: v1.TypeMeta{
			Kind: "AdmissionReview",
		},
		Request: &v1beta1.AdmissionRequest{
			UID: "e911857d-c318-11e8-bbad-025000000001",
			Kind: v1.GroupVersionKind{
				Kind: "Namespace",
			},
			Operation: "CREATE",
			Object: runtime.RawExtension{
				Raw: []byte(`{"metadata": {
        						"name": "test",
        						"uid": "e911857d-c318-11e8-bbad-025000000001",
						        "creationTimestamp": "2018-09-28T12:20:39Z"
      						}}`),
			},
		},
	}
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

func decodeResponse(body io.ReadCloser) *v1beta1.AdmissionReview {
	response, _ := ioutil.ReadAll(body)
	review := &v1beta1.AdmissionReview{}
	_, _, _ = codecs.UniversalDeserializer().Decode(response, nil, review)
	return review
}

func encodeRequest(review *v1beta1.AdmissionReview) []byte {
	ret, err := json.Marshal(review)
	if err != nil {
		logrus.Errorln(err)
	}
	return ret
}

func TestServeReturnsCorrectJson(t *testing.T) {
	nsc := &webhook.BhAdmission{}
	server := httptest.NewServer(server.GetAdmissionServerNoSSL(nsc, ":8080").Handler)
	requestString := string(encodeRequest(&admissionRequestNS))
	myr := strings.NewReader(requestString)
	r, err := http.Post(server.URL, "application/json", myr)
	if err != nil {
		t.Error("Post failed")
		return
	}
	defer r.Body.Close()
	review := decodeResponse(r.Body)

	if review.Request.UID != admissionRequestNS.Request.UID {
		t.Error("Request and response UID don't match")
	}
}
