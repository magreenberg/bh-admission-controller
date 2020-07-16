package main

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"namespace-admission-controller/server"
	"namespace-admission-controller/webhook"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	propertyFile = "/etc/webhook/bh-admission-config/bh-admission.properties"
	// TLSCert is the TLS certificate
	TLSCert = "/etc/webhook/certs/cert.pem"
	// TLSKey is the TLS key
	TLSKey                  = "/etc/webhook/certs/key.pem"
	listenAddrKey           = "listen_addr"
	listenAddrDefaultValue  = "0.0.0.0:8080"
	metricsAddrKey          = "metrics_addr"
	metricsAddrDefaultValue = ":2112"
	externalAPIURLKey       = "external_api_url"
	externalAPITimeoutKey   = "external_api_timeout"
	requesterKey            = "requester_key"
)

func main() {
	// set up defaults
	viper.SetDefault(listenAddrKey, listenAddrDefaultValue)
	viper.SetDefault(metricsAddrKey, metricsAddrDefaultValue)
	viper.SetDefault(externalAPITimeoutKey, 12)
	viper.SetDefault(requesterKey, "company.com/requester")
	viper.AutomaticEnv()

	// override defaults with property file values
	viper.SetConfigFile(propertyFile)
	err := viper.ReadInConfig()
	if err != nil {
		logrus.Infoln("Config file "+propertyFile+":", err)
	}
	logrus.Println(viper.AllSettings())

	if viper.GetBool("DEBUG") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	go func() {
		metricsAddr := viper.GetString(metricsAddrKey)
		// blocking method needs to run in a separate thread
		logrus.Println("metrics starting to listen on ", metricsAddr)
		http.Handle("/metrics", promhttp.Handler())
		err = http.ListenAndServe(metricsAddr, nil)
		if err != nil {
			logrus.Errorln("Failed to start metrics listener:", err)
			os.Exit(1)
		}
	}()

	listenAddr := viper.GetString(listenAddrKey)
	nsac := webhook.BhAdmission{
		ExternalAPIURL:     viper.GetString(externalAPIURLKey),
		ExternalAPITimeout: viper.GetInt32(externalAPITimeoutKey),
		RequesterKey:       viper.GetString(requesterKey),
	}
	s := server.GetAdmissionValidationServer(&nsac, TLSCert, TLSKey, listenAddr)
	logrus.Println("Webhook starting to listen on ", listenAddr)
	err = s.ListenAndServeTLS("", "")
	if err != nil {
		logrus.Errorln("Failed to start ListenAndServeTLS:", err)
		os.Exit(1)
	}
}
