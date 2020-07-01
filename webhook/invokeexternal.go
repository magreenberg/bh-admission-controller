package webhook

import (
	"errors"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

func invokeexternal(apiURL string, apiTimeout int32, jsondata string) error {
	client := &http.Client{
		Timeout: time.Duration(apiTimeout) * time.Second,
	}
	// Do not use http.Post as timeout cannot be used
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(jsondata))
	if err != nil {
		logrus.Errorln("http.NewRequest failed:", err)
		return err
	}
	req.Header.Add("Accept", "application/json")
	logrus.WithFields(logrus.Fields{
		"URL":     apiURL,
		"Timeout": apiTimeout,
		"JSON":    jsondata,
	}).Debug("Invoking external URL")
	response, err := client.Do(req)
	if err != nil {
		logrus.Errorln("External API failed:", err)
		return err
	}

	defer response.Body.Close()

	bytes, _ := ioutil.ReadAll(response.Body)

	contextLogger := logrus.WithFields(logrus.Fields{
		"HTTP Status Code": response.StatusCode,
		"response":         string(bytes),
	})

	if response.StatusCode != http.StatusOK {
		contextLogger.Error("External API invocation FAILED")
		return errors.New("Failed")
	}
	contextLogger.Infoln("External API invocation succeeded")
	return nil
}
