package webhook

import (
	"strings"
)

func mergeAnnotations(original map[string]string, added map[string]string) map[string]string {
	mergedMap := map[string]string{}
	for k, v := range original {
		mergedMap[k] = v
	}
	for k, v := range added {
		mergedMap[k] = v
	}
	return mergedMap
}

func findAnnotation(annotation map[string]string, searchkey string) (string, bool) {
	for key, value := range annotation {
		if strings.EqualFold(searchkey, key) {
			return value, true
		}
	}
	return "", false
}
