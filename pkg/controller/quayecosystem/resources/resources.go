package resources

import (
	"fmt"

	redhatcopv1alpha1 "github.com/theodor2311/quay-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/theodor2311/quay-operator/pkg/controller/quayecosystem/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewResourceObjectMeta builds ObjectMeta for all Kubernetes resources created by operator
func NewResourceObjectMeta(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      GetGenericResourcesName(quayEcosystem),
		Namespace: quayEcosystem.ObjectMeta.Namespace,
		Labels:    BuildResourceLabels(quayEcosystem),
	}
}

// GetGenericResourcesName returns name of Kubernetes resource name
func GetGenericResourcesName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return quayEcosystem.ObjectMeta.Name
}

// BuildResourceLabels returns labels for all Kubernetes resources created by operator
func BuildResourceLabels(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) map[string]string {
	return map[string]string{
		constants.LabelAppKey:    constants.LabelAppValue,
		constants.LabelQuayCRKey: quayEcosystem.Name,
	}
}

// BuildQuayResourceLabels builds labels for the Quay app resources
func BuildQuayResourceLabels(resourceMap map[string]string) map[string]string {
	resourceMap[constants.LabelCompoentKey] = constants.LabelComponentAppValue
	return resourceMap
}

// BuildClairResourceLabels builds labels for the Clair app resources
func BuildClairResourceLabels(resourceMap map[string]string) map[string]string {
	resourceMap[constants.LabelCompoentKey] = constants.LabelComponentClairValue
	return resourceMap
}

// BuildQuayConfigResourceLabels builds labels for the Quay config resources
func BuildQuayConfigResourceLabels(resourceMap map[string]string) map[string]string {
	resourceMap[constants.LabelCompoentKey] = constants.LabelComponentConfigValue
	return resourceMap
}

// BuildQuayDatabaseResourceLabels builds labels for the Quay app resources
func BuildQuayDatabaseResourceLabels(resourceMap map[string]string) map[string]string {
	resourceMap[constants.LabelCompoentKey] = constants.LabelComponentQuayDatabaseValue
	return resourceMap
}

// BuildRedisResourceLabels builds labels for the Redis app resources
func BuildRedisResourceLabels(resourceMap map[string]string) map[string]string {
	resourceMap[constants.LabelCompoentKey] = constants.LabelComponentRedisValue
	return resourceMap
}

// GetQuayResourcesName returns name of Kubernetes resource name
func GetQuayResourcesName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-quay", GetGenericResourcesName(quayEcosystem))
}

// GetClairResourcesName returns name of Kubernetes resource name
func GetClairResourcesName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-clair", GetGenericResourcesName(quayEcosystem))
}

// GetQuayConfigResourcesName returns name of Kubernetes resource name
func GetQuayConfigResourcesName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-quay-config", GetGenericResourcesName(quayEcosystem))
}

// GetRedisResourcesName returns name of Kubernetes resource name
func GetRedisResourcesName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-redis", GetGenericResourcesName(quayEcosystem))
}

// GetConfigMapSecretName returns the name of the Quay config secret
func GetConfigMapSecretName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	//configSecretName := fmt.Sprintf("%s-config-secret", GetGenericResourcesName(quayEcosystem))
	return "quay-enterprise-config-secret"
	//return configSecretName
}

// GetClairConfigSecretName returns the name of the Clair config secret
func GetClairConfigSecretName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return "clair-config-secret"
}

// GetSecurityScannerKeySecretName returns the name of the security scanner key secret
func GetSecurityScannerKeySecretName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return "security-scanner-key-secret"
}

// GetClairTrustCASecretName returns the name of the Clair trust CA secret
func GetClairTrustCASecretName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return "clair-trust-ca-secret"
}

// GetQuayExtraCertsSecretName returns the name of the Quay extra certs secret
func GetQuayExtraCertsSecretName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return "quay-enterprise-cert-secret"
}

// GetQuayDatabaseName returns the name of the Quay database
func GetQuayDatabaseName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-quay-%s", GetGenericResourcesName(quayEcosystem), constants.PostgresqlName)
}

// GetClairDatabaseName returns the name of the Quay database
func GetClairDatabaseName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-clair-%s", GetGenericResourcesName(quayEcosystem), constants.PostgresqlName)
}

// GetQuayRegistryStorageName returns the name of the Quay registry storage
func GetQuayRegistryStorageName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem) string {
	return fmt.Sprintf("%s-registry", GetGenericResourcesName(quayEcosystem))
}

// GetRegistryStorageVolumeName returns the name that should be applied to the volume for the storage backend
func GetRegistryStorageVolumeName(quayEcosystem *redhatcopv1alpha1.QuayEcosystem, registryBackendName string) string {
	return fmt.Sprintf("%s-%s", GetGenericResourcesName(quayEcosystem), registryBackendName)
}

// UpdateMetaWithName updates the name of the resource
func UpdateMetaWithName(meta metav1.ObjectMeta, name string) metav1.ObjectMeta {
	meta.Name = name
	return meta
}
