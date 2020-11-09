package webhook

import (
	"strings"
)

func mergeAnnotation(target map[string]string, added map[string]string) {
	mergedMap := map[string]string{}
	for k, v := range target {
		mergedMap[k] = v
	}
	for k, v := range added {
		mergedMap[k] = v
	}
}

func findAnnotation(annotation map[string]string, searchkey string) (string, bool) {
	for key, value := range annotation {
		// compatibility for OCP "oc new-project <project>"
		if strings.EqualFold(searchkey, key) {
			return value, true
		}
	}
	return "", false
}
