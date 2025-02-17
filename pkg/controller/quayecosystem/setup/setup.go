package setup

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/theodor2311/quay-operator/pkg/controller/quayecosystem/constants"

	"github.com/redhat-cop/operator-utils/pkg/util"
	"github.com/theodor2311/quay-operator/pkg/client"
	"github.com/theodor2311/quay-operator/pkg/controller/quayecosystem/logging"
	"github.com/theodor2311/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/theodor2311/quay-operator/pkg/controller/quayecosystem/utils"
	"k8s.io/client-go/kubernetes"

	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Registry Represents the status returned from the Quay Registry
type RegistryStatus string

var (
	RegistryStatusConfigDB RegistryStatus = "config-db"
	RegistryStatusSetupDB  RegistryStatus = "setup-db"
	RegistryStatusConfig   RegistryStatus = "config"
)

type QuaySetupManager struct {
	reconcilerBase util.ReconcilerBase
	k8sclient      kubernetes.Interface
}

type QuaySetupInstance struct {
	quayConfiguration resources.QuayConfiguration
	setupClient       client.QuayClient
}

func NewQuaySetupManager(reconcilerBase util.ReconcilerBase, k8sclient kubernetes.Interface) *QuaySetupManager {
	return &QuaySetupManager{reconcilerBase: reconcilerBase, k8sclient: k8sclient}
}

func (*QuaySetupManager) PrepareForSetup(kclient kclient.Client, quayConfiguration *resources.QuayConfiguration) error {

	quayConfigHost := quayConfiguration.QuayEcosystem.Spec.Quay.ConfigRouteHost

	if utils.IsZeroOfUnderlyingType(quayConfigHost) {
		quayConfigHost = resources.GetQuayConfigResourcesName(quayConfiguration.QuayEcosystem)
	}

	quayConfiguration.QuayConfigHostname = quayConfigHost

	/*
		quayRouteHost := quayConfiguration.QuayEcosystem.Spec.Quay.RouteHost


			if utils.IsZeroOfUnderlyingType(quayRouteHost) {

				quayRoute := &routev1.Route{}
				err := kclient.Get(context.TODO(), types.NamespacedName{Name: resources.GetQuayResourcesName(quayConfiguration.QuayEcosystem), Namespace: quayConfiguration.QuayEcosystem.Namespace}, quayRoute)

				if err != nil {
					logging.Log.Error(err, "Error Finding Quay Route", "Namespace", quayConfiguration.QuayEcosystem.Namespace, "Name", quayConfiguration.QuayEcosystem.Name)
				}

				quayRouteHost = quayRoute.Spec.Host
			}

			quayConfiguration.QuayHostname = quayRouteHost
	*/

	postgresqlHost := quayConfiguration.QuayEcosystem.Spec.Quay.Database.Server

	if utils.IsZeroOfUnderlyingType(postgresqlHost) {
		postgresqlHost = resources.GetQuayDatabaseName(quayConfiguration.QuayEcosystem)
	}

	quayConfiguration.QuayDatabase.Server = postgresqlHost

	redisHost := quayConfiguration.QuayEcosystem.Spec.Redis.Hostname

	if utils.IsZeroOfUnderlyingType(redisHost) {
		redisHost = resources.GetRedisResourcesName(quayConfiguration.QuayEcosystem)
	}

	quayConfiguration.RedisHostname = redisHost

	if utils.IsZeroOfUnderlyingType(quayConfiguration.QuayEcosystem.Spec.Redis.Port) {
		quayConfiguration.RedisPort = quayConfiguration.QuayEcosystem.Spec.Redis.Port
	}

	return nil
}

func (*QuaySetupManager) NewQuaySetupInstance(quayConfiguration *resources.QuayConfiguration) (*QuaySetupInstance, error) {

	t := http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := http.Client{
		Transport: &t,
	}

	quayConfigURL := fmt.Sprintf("https://%s", quayConfiguration.QuayConfigHostname)

	setupClient := client.NewClient(&httpClient, quayConfigURL, quayConfiguration.QuayConfigUsername, quayConfiguration.QuayConfigPassword)

	quaySetupInstance := QuaySetupInstance{
		quayConfiguration: *quayConfiguration,
		setupClient:       *setupClient,
	}

	return &quaySetupInstance, nil
}

// SetupQuay performs the initialization and initial configuration of the Quay server
func (qm *QuaySetupManager) SetupQuay(quaySetupInstance *QuaySetupInstance) error {

	_, _, err := quaySetupInstance.setupClient.GetRegistryStatus()

	if err != nil {
		logging.Log.Error(err, "Failed to obtain initial registry status")
		return err
	}

	_, _, err = quaySetupInstance.setupClient.InitializationConfiguration()

	if err != nil {
		logging.Log.Error(err, "Failed to Initialize")
		return err
	}

	quayConfig := client.QuayConfig{
		Config: map[string]interface{}{},
	}

	quayConfig.Config["DB_URI"] = fmt.Sprintf("postgresql://%s:%s@%s/%s", quaySetupInstance.quayConfiguration.QuayDatabase.Username, quaySetupInstance.quayConfiguration.QuayDatabase.Password, quaySetupInstance.quayConfiguration.QuayDatabase.Server, quaySetupInstance.quayConfiguration.QuayDatabase.Database)

	err = qm.validateComponent(quaySetupInstance, quayConfig, client.DatabaseValidation)

	if err != nil {
		return err
	}

	redisConfiguration := map[string]interface{}{
		"host": quaySetupInstance.quayConfiguration.RedisHostname,
	}

	if !utils.IsZeroOfUnderlyingType(quaySetupInstance.quayConfiguration.RedisPort) {
		redisConfiguration["port"] = quaySetupInstance.quayConfiguration.RedisPort
	}

	quayConfig.Config["BUILDLOGS_REDIS"] = redisConfiguration
	quayConfig.Config["USER_EVENTS_REDIS"] = redisConfiguration
	quayConfig.Config["SERVER_HOSTNAME"] = quaySetupInstance.quayConfiguration.QuayHostname

	_, _, err = quaySetupInstance.setupClient.UpdateQuayConfiguration(quayConfig)

	if err != nil {
		logging.Log.Error(err, "Failed to update quay configuration")
		return fmt.Errorf("Failed to update quay configuration: %s", err.Error())
	}

	_, _, err = quaySetupInstance.setupClient.SetupDatabase()

	if err != nil {
		logging.Log.Error(err, "Failed to setup database")
		return fmt.Errorf("Failed to setup database: %s", err.Error())
	}

	_, _, err = quaySetupInstance.setupClient.CreateSuperuser(client.QuayCreateSuperuserRequest{
		Username:        quaySetupInstance.quayConfiguration.QuaySuperuserUsername,
		Email:           quaySetupInstance.quayConfiguration.QuaySuperuserEmail,
		Password:        quaySetupInstance.quayConfiguration.QuaySuperuserPassword,
		ConfirmPassword: quaySetupInstance.quayConfiguration.QuaySuperuserPassword,
	})

	if err != nil {
		logging.Log.Error(err, "Failed to create superuser")
		return fmt.Errorf("Failed to create superuser: %s", err.Error())
	}

	_, quayConfig, err = quaySetupInstance.setupClient.GetQuayConfiguration()

	if err != nil {
		logging.Log.Error(err, "Failed to get Quay Configuration")
		return fmt.Errorf("Failed to get Quay Configuration: %s", err.Error())
	}

	// var quayKeys client.QuayKeys
	// _, _, err = quaySetupInstance.setupClient.GetQuayKeys()

	// if err != nil {
	// 	logging.Log.Error(err, "Failed to get security scanner key")
	// 	return fmt.Errorf("Failed to get security scanner key: %s", err.Error())
	// }

	//TODO Change to the correct clair endpoint
	quayConfig.Config["SECURITY_SCANNER_ENDPOINT"] = fmt.Sprintf("http://%s", quaySetupInstance.quayConfiguration.ClairHostname)
	quayConfig.Config["SECURITY_SCANNER_ISSUER_NAME"] = "security_scanner"
	quayConfig.Config["FEATURE_SECURITY_SCANNER"] = true

	// Setup Storage
	distributedStorageConfig := map[string][]interface{}{}

	for _, registryBackend := range quaySetupInstance.quayConfiguration.QuayEcosystem.Spec.Quay.RegistryBackends {

		var quayRegistry []interface{}

		if !utils.IsZeroOfUnderlyingType(registryBackend.RegistryBackendSource.Local) {
			quayRegistry = append(quayRegistry, constants.RegistryStorageTypeLocalStorageName)
			quayRegistry = append(quayRegistry, registryBackend.RegistryBackendSource.Local)
		}

		distributedStorageConfig[registryBackend.Name] = quayRegistry

	}

	quayConfig.Config["DISTRIBUTED_STORAGE_CONFIG"] = distributedStorageConfig

	// Add Certificates
	_, _, err = quaySetupInstance.setupClient.UploadFileResource(constants.QuayAppConfigSSLPrivateKeySecretKey, quaySetupInstance.quayConfiguration.QuaySslPrivateKey)

	if err != nil {
		logging.Log.Error(err, "Failed to upload SSL certificates")
		return fmt.Errorf("Failed to upload SSL certificates: %s", err.Error())
	}

	_, _, err = quaySetupInstance.setupClient.UploadFileResource(constants.QuayAppConfigSSLCertificateSecretKey, quaySetupInstance.quayConfiguration.QuaySslCertificate)

	if err != nil {
		return err
	}

	// Validate multiple components
	for _, validationComponent := range []client.QuayValidationType{client.RedisValidation, client.RegistryValidation, client.TimeMachineValidation, client.AccessValidation, client.SslValidation} {
		err = qm.validateComponent(quaySetupInstance, quayConfig, validationComponent)

		if err != nil {
			logging.Log.Error(err, "Failed to Validate Component")
			return fmt.Errorf("Failed to Validate Component: %s", err.Error())
		}
	}

	quayConfig.Config["PREFERRED_URL_SCHEME"] = "https"

	quayConfig.Config["SETUP_COMPLETE"] = true
	_, quayConfig, err = quaySetupInstance.setupClient.UpdateQuayConfiguration(quayConfig)

	if err != nil {
		logging.Log.Error(err, "Failed to update Quay Configuration")
		return fmt.Errorf("Failed to update Quay Configuration: %s", err.Error())
	}

	_, _, err = quaySetupInstance.setupClient.CompleteSetup()

	if err != nil {
		logging.Log.Error(err, "Failed to complete Quay Configuration setup")
		return fmt.Errorf("Failed to complete Quay Configuration setup: %s", err.Error())
	}

	return nil

}

func (*QuaySetupManager) SetupSecurityScannerKey(quaySetupInstance *QuaySetupInstance, quayConfiguration *resources.QuayConfiguration) error {

	//TODO Convert to parameter
	var securityScannerKey client.SecurityScannerKey
	_, securityScannerKey, err := quaySetupInstance.setupClient.CreateSecurityScannerKey(client.QuayCreateSecurityScannerKeyRequest{
		Name:       "security_scanner Service Key",
		Service:    "security_scanner",
		Expiration: nil,
		Notes:      "Created during setup for service `security_scanner`",
	})

	if err != nil {
		logging.Log.Error(err, "Failed to create security scanner key")
		return fmt.Errorf("Failed to create security scanner key: %s", err.Error())
	}

	quayConfiguration.SecurityScannerKeyKid = securityScannerKey.Kid
	quayConfiguration.SecurityScannerKeyPrivateKey = securityScannerKey.PrivateKey

	return nil
}

func (*QuaySetupManager) validateComponent(quaySetupInstance *QuaySetupInstance, quayConfig client.QuayConfig, validationType client.QuayValidationType) error {

	_, validateResponse, err := quaySetupInstance.setupClient.ValidateComponent(quayConfig, validationType)

	if err != nil {
		return err
	}

	if !validateResponse.Status {
		return fmt.Errorf("%s Validation Failed: %s", validationType, validateResponse.Reason)
	}

	return nil
}
