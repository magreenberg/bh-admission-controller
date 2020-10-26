package webhook

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"time"
)
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
