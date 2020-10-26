package main

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"namespace-admission-controller/server"
	"namespace-admission-controller/webhook"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	//buildv1client "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	"k8s.io/client-go/tools/clientcmd"
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
	clusterNameKey          = "cluster_name_key"
)

func getClustername(urlString string) (string, error) {
	u, err := url.Parse(urlString)
	clusterName := ""
	if err == nil {
		h1 := strings.Split(u.Hostname(), ".")
		if len(h1) > 1 {
			clusterName = h1[1]
			_, err := strconv.Atoi(clusterName)
			if err == nil {
				logrus.Warn("cluster name was not automatically detected.")
			}
		}
	}
	return clusterName, err
}

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

	// Instantiate loader for kubeconfig file.
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)

	// Determine the Namespace referenced by the current context in the
	// kubeconfig file.
	namespace, _, err := kubeconfig.Namespace()
	if err != nil {
		panic(err)
	}
	logrus.Println("namespace=", namespace)

	clusterName := viper.GetString(clusterNameKey)
	if len(clusterName) == 0 {
		// clientConfig.HOST gave IP address
		clientConfig, err := kubeconfig.ClientConfig()
		if err != nil {
			panic(err)
		}
		clusterName, _ = getClustername(clientConfig.Host)
		logrus.Println("clusterName=", clusterName)
		rawConfig, _ := kubeconfig.RawConfig()
		logrus.Println("rawConfig.CurrentContext =", rawConfig.CurrentContext)
	}
	// Get a rest.Config from the kubeconfig file.  This will be passed into all
	// the client objects we create.
	restconfig, err := kubeconfig.ClientConfig()
	if err != nil {
		panic(err)
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
		RestConfig:         *restconfig,
		ClusterName:        clusterName,
	}
	s := server.GetAdmissionValidationServer(&nsac, TLSCert, TLSKey, listenAddr)
	logrus.Println("Webhook starting to listen on ", listenAddr)
	err = s.ListenAndServeTLS("", "")
	if err != nil {
		logrus.Errorln("Failed to start ListenAndServeTLS:", err)
		os.Exit(1)
	}
}
