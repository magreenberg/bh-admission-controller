package webhook

import (
	"encoding/json"
	"strings"
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func mergeAnnotations(original map[string]string, added map[string]string) map[string]string {
	mergedMap := map[string]string{}
	for k, v := range original {
		if !strings.EqualFold("kubectl.kubernetes.io/last-applied-configuration", k) {
			mergedMap[k] = v
		}
	}
	for k, v := range added {
		if !strings.EqualFold("kubectl.kubernetes.io/last-applied-configuration", k) {
			mergedMap[k] = v
		}
	}
	return mergedMap
}

func updateAnnotation(requestAnnotations map[string]string, addedAnnotations map[string]string) (patch []patchOperation) {
	po := patchOperation{
		Op:    "add",
		Path:  "/metadata/annotations",
		Value: mergeAnnotations(requestAnnotations, addedAnnotations),
	}
	patch = append(patch, po)
	return patch
}

// create mutation patch for resoures
func createPatch(requestAnnotations map[string]string, addedAnnotations map[string]string) ([]byte, error) {
	var patch []patchOperation

	patch = append(patch, updateAnnotation(requestAnnotations, addedAnnotations)...)

	return json.Marshal(patch)
}
