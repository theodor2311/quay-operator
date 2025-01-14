package resources

import (
	redhatcopv1alpha1 "github.com/theodor2311/quay-operator/pkg/apis/redhatcop/v1alpha1"
)

// QuayConfiguration is a wrapper object around a QuayEcosystem that provides the full set of configurable options
type QuayConfiguration struct {
	QuayEcosystem *redhatcopv1alpha1.QuayEcosystem

	// OpenShift Environment
	IsOpenShift bool

	// Superuser
	QuaySuperuserUsername            string
	QuaySuperuserPassword            string
	QuaySuperuserEmail               string
	ValidProvidedQuaySuperuserSecret bool

	// Database
	ValidProvidedQuayDatabaseSecret bool
	QuayDatabase                    DatabaseConfig
	ProvisionQuayDatabase           bool

	// Redis
	RedisHostname string
	RedisPort     *int32
	RedisPassword string

	// Security Scanner
	// SecurityScannerKeyName			string
	// SecurityScannerKeyService		string
	// SecurityScannerKeyExpiration		interface{}
	// SecurityScannerKeyNotes			string
	SecurityScannerKeyKid        string
	SecurityScannerKeyPrivateKey string

	// Quay
	QuayHostname                          string
	QuayConfigHostname                    string
	QuayConfigUsername                    string
	QuayConfigPassword                    string
	QuayConfigPasswordSecret              string
	ValidProvidedQuayConfigPasswordSecret bool
	QuayImage                             string
	QuayReplicas                          *int32
	DeployQuayConfiguration               bool
	QuaySslCertificate                    []byte
	QuaySslPrivateKey                     []byte

	//Clair
	ClairHostname string
}

// DatabaseConfig is an internal structure representing a database
type DatabaseConfig struct {
	Name                string
	Server              string
	Image               string
	Database            string
	Username            string
	Password            string
	RootPassword        string
	CPU                 string
	Memory              string
	VolumeSize          string
	CredentialsName     string
	ValidProvidedSecret bool
	UserProvided        bool
}
