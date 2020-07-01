package main

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"namespace-admission-controller/server"
	"namespace-admission-controller/webhook"
	"os"
)

const (
	propertyFile = "/etc/webhook/namespace-admission-config/namespace-admission.properties"
	// TLSCert is the TLS certificate
	TLSCert = "/etc/webhook/certs/cert.pem"
	// TLSKey is the TLS key
	TLSKey                 = "/etc/webhook/certs/key.pem"
	listenAddrKey          = "listen_addr"
	listenAddrDefaultValue = "0.0.0.0:8080"
	externalAPIURLKey      = "external_api_url"
	externalAPITimeoutKey  = "external_api_timeout"
	requesterKey           = "requester_key"
)

func main() {
	// set up defaults
	viper.SetDefault(listenAddrKey, listenAddrDefaultValue)
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

	listenAddr := viper.GetString(listenAddrKey)

	nsac := webhook.NamespaceAdmission{
		ExternalAPIURL:     viper.GetString(externalAPIURLKey),
		ExternalAPITimeout: viper.GetInt32(externalAPITimeoutKey),
		RequesterKey:       viper.GetString(requesterKey),
	}
	s := server.GetAdmissionValidationServer(&nsac, TLSCert, TLSKey, listenAddr)
	logrus.Println("Webhook listening on ", listenAddr)
	err = s.ListenAndServeTLS("", "")
	if err != nil {
		logrus.Errorln("Failed to start ListenAndServeTLS:", err)
		os.Exit(1)
	}
}
