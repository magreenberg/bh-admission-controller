package webhook

import (
	"encoding/json"
	corev1 "k8s.io/api/core/v1"
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func updateAnnotation(target map[string]string, added map[string]string) (patch []patchOperation) {
	mergedMap := map[string]string{}
	for k, v := range target {
		mergedMap[k] = v
	}
	for k, v := range added {
		mergedMap[k] = v
	}
	po := patchOperation{
		Op:    "add",
		Path:  "/metadata/annotations",
		Value: mergedMap,
	}
	patch = append(patch, po)
	return patch
}

// create mutation patch for resoures
func createPatch(ns *corev1.Namespace, annotations map[string]string) ([]byte, error) {
	var patch []patchOperation

	patch = append(patch, updateAnnotation(ns.Annotations, annotations)...)

	return json.Marshal(patch)
}
