package provisioning

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	ossecurityv1 "github.com/openshift/api/security/v1"
	"github.com/theodor2311/quay-operator/pkg/controller/quayecosystem/constants"
	"github.com/theodor2311/quay-operator/pkg/controller/quayecosystem/logging"
	"github.com/theodor2311/quay-operator/pkg/controller/quayecosystem/resources"
	"github.com/theodor2311/quay-operator/pkg/controller/quayecosystem/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/theodor2311/quay-operator/pkg/k8sutils"

	appsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	"github.com/redhat-cop/operator-utils/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ReconcileQuayEcosystemConfiguration defines values required for Quay configuration
type ReconcileQuayEcosystemConfiguration struct {
	reconcilerBase    util.ReconcilerBase
	k8sclient         kubernetes.Interface
	quayConfiguration *resources.QuayConfiguration
}

// New creates the structure for the Quay configuration
func New(reconcilerBase util.ReconcilerBase, k8sclient kubernetes.Interface,
	quayConfiguration *resources.QuayConfiguration) *ReconcileQuayEcosystemConfiguration {
	return &ReconcileQuayEcosystemConfiguration{
		reconcilerBase:    reconcilerBase,
		k8sclient:         k8sclient,
		quayConfiguration: quayConfiguration,
	}
}

// CoreResourceDeployment takes care of base configuration
func (r *ReconcileQuayEcosystemConfiguration) CoreResourceDeployment(metaObject metav1.ObjectMeta) (*reconcile.Result, error) {

	if err := r.createQuayConfigSecret(metaObject); err != nil {
		return nil, err
	}

	if err := r.createClairConfigSecret(metaObject); err != nil {
		return nil, err
	}

	if err := r.createSecurityScannerKeySecret(metaObject); err != nil {
		return nil, err
	}

	if err := r.createClairTrustCASecret(metaObject); err != nil {
		return nil, err
	}

	if err := r.createRBAC(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create RBAC")
		return nil, err
	}

	if err := r.createServiceAccounts(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Service Accounts")
		return nil, err
	}

	if err := r.configureAnyUIDSCCs(metaObject); err != nil {
		logging.Log.Error(err, "Failed to configure SCCs")
		return nil, err
	}

	// Redis
	if utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Redis.Hostname) {
		if err := r.createRedisService(metaObject); err != nil {
			logging.Log.Error(err, "Failed to create Redis service")
			return nil, err
		}

		redisDeploymentResult, err := r.redisDeployment(metaObject)
		if err != nil {
			logging.Log.Error(err, "Failed to create Redis deployment")
			return redisDeploymentResult, err
		}

	}

	// Database (PostgreSQL/MySQL)
	if utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Quay.Database.Server) {

		createDatabaseResult, err := r.createQuayDatabase(metaObject)

		if err != nil {
			logging.Log.Error(err, "Failed to create Quay database")
			return nil, err
		}

		if createDatabaseResult != nil {
			return createDatabaseResult, nil
		}

		err = r.configurePostgreSQL(metaObject)

		if err != nil {
			logging.Log.Error(err, "Failed to Setup Postgresql")
			return nil, err
		}

	}

	// Quay Resources
	if err := r.createQuayService(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Quay service")
		return nil, err
	}

	if err := r.createQuayConfigService(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Quay Config service")
		return nil, err
	}

	if err := r.createClairService(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Clair service")
		return nil, err
	}

	if err := r.createQuayRoute(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Quay route")
		return nil, err
	}

	if err := r.createQuayConfigRoute(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Quay Config route")
		return nil, err
	}

	if err := r.createClairRoute(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Clair route")
		return nil, err
	}

	if !utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage) {

		if err := r.quayRegistryStorage(metaObject); err != nil {
			logging.Log.Error(err, "Failed to create registry storage")
			return nil, err
		}

	}

	return nil, nil
}

// DeployClair takes care of the deployment
func (r *ReconcileQuayEcosystemConfiguration) DeployClair(metaObject metav1.ObjectMeta) (*reconcile.Result, error) {

	if err := r.clairDeployment(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Clair deployment")
		return nil, err
	}

	time.Sleep(time.Duration(2) * time.Second)

	// Verify Deployment
	deploymentName := resources.GetClairResourcesName(r.quayConfiguration.QuayEcosystem)

	return r.verifyDeployment(deploymentName, r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace)

}

// DeployQuayConfiguration takes care of the deployment of the quay configuration
func (r *ReconcileQuayEcosystemConfiguration) DeployQuayConfiguration(metaObject metav1.ObjectMeta) (*reconcile.Result, error) {

	if err := r.quayConfigDeployment(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Quay Config deployment")
		return nil, err
	}

	time.Sleep(time.Duration(2) * time.Second)

	// Verify Deployment
	deploymentName := resources.GetQuayConfigResourcesName(r.quayConfiguration.QuayEcosystem)

	return r.verifyDeployment(deploymentName, r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace)

}

// DeployQuay takes care of base configuration
func (r *ReconcileQuayEcosystemConfiguration) DeployQuay(metaObject metav1.ObjectMeta) (*reconcile.Result, error) {

	if err := r.quayDeployment(metaObject); err != nil {
		logging.Log.Error(err, "Failed to create Quay deployment")
		return nil, err
	}

	if !r.quayConfiguration.QuayEcosystem.Spec.Quay.SkipSetup {

		time.Sleep(time.Duration(2) * time.Second)

		// Verify Deployment
		deploymentName := resources.GetQuayResourcesName(r.quayConfiguration.QuayEcosystem)

		return r.verifyDeployment(deploymentName, r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace)

	} else {
		logging.Log.Info("Skipping Quay Deployment verification as setup marked as skipped")
		return &reconcile.Result{}, nil
	}

}

// DeployQuay takes care of base configuration
func (r *ReconcileQuayEcosystemConfiguration) RemoveQuayConfigResources(metaObject metav1.ObjectMeta) (*reconcile.Result, error) {

	quayName := resources.GetQuayConfigResourcesName(r.quayConfiguration.QuayEcosystem)

	err := r.k8sclient.AppsV1().Deployments(r.quayConfiguration.QuayEcosystem.Namespace).Delete(quayName, &metav1.DeleteOptions{})

	if err != nil && !apierrors.IsNotFound(err) {
		logging.Log.Error(err, "Error Deleting Quay Config Deployment", "Namespace", r.quayConfiguration.QuayEcosystem.Namespace, "Name", quayName)
		return nil, err
	}

	// OpenShift Route
	route := &routev1.Route{}
	err = r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: quayName, Namespace: r.quayConfiguration.QuayEcosystem.Namespace}, route)

	if err != nil && !apierrors.IsNotFound(err) {
		logging.Log.Error(err, "Error Finding Quay Config Route", "Namespace", r.quayConfiguration.QuayEcosystem.Namespace, "Name", quayName)
		return nil, err
	}

	err = r.reconcilerBase.GetClient().Delete(context.TODO(), route)

	if err != nil && !apierrors.IsNotFound(err) {
		logging.Log.Error(err, "Failed to Delete Quay Config Route", "Namespace", r.quayConfiguration.QuayEcosystem.Namespace, "Name", quayName)
		return nil, err
	}

	err = r.k8sclient.CoreV1().Services(r.quayConfiguration.QuayEcosystem.Namespace).Delete(quayName, &metav1.DeleteOptions{})

	if err != nil && !apierrors.IsNotFound(err) {
		logging.Log.Error(err, "Error Deleting Quay Config Service", "Namespace", r.quayConfiguration.QuayEcosystem.Namespace, "Name", quayName)
		return nil, err
	}

	return nil, nil
}

func (r *ReconcileQuayEcosystemConfiguration) createQuayDatabase(meta metav1.ObjectMeta) (*reconcile.Result, error) {

	// Update Metadata
	meta = resources.UpdateMetaWithName(meta, resources.GetQuayDatabaseName(r.quayConfiguration.QuayEcosystem))
	resources.BuildQuayDatabaseResourceLabels(meta.Labels)

	var databaseResources []metav1.Object

	if !r.quayConfiguration.ValidProvidedQuayDatabaseSecret {
		quayDatabaseSecret := resources.GetSecretDefinitionFromCredentialsMap(resources.GetQuayDatabaseName(r.quayConfiguration.QuayEcosystem), meta, constants.DefaultQuayDatabaseCredentials)
		databaseResources = append(databaseResources, quayDatabaseSecret)

		r.quayConfiguration.QuayDatabase.Username = constants.DefaultQuayDatabaseCredentials[constants.DatabaseCredentialsUsernameKey]
		r.quayConfiguration.QuayDatabase.Password = constants.DefaultQuayDatabaseCredentials[constants.DatabaseCredentialsPasswordKey]
		r.quayConfiguration.QuayDatabase.Database = constants.DefaultQuayDatabaseCredentials[constants.DatabaseCredentialsDatabaseKey]

	}

	// Create PVC
	if !utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Quay.Database.VolumeSize) {
		databasePvc := resources.GetDatabasePVCDefinition(meta, r.quayConfiguration.QuayEcosystem.Spec.Quay.Database.VolumeSize)
		databaseResources = append(databaseResources, databasePvc)
	}

	service := resources.GetDatabaseServiceResourceDefinition(meta, constants.PostgreSQLPort)
	databaseResources = append(databaseResources, service)

	deployment := resources.GetDatabaseDeploymentDefinition(meta, r.quayConfiguration)
	databaseResources = append(databaseResources, deployment)

	for _, resource := range databaseResources {
		err := r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, resource)
		if err != nil {
			logging.Log.Error(err, "Error applying Quay database Resource")
			return nil, err
		}
	}

	// Verify Deployment
	deploymentName := meta.Name

	time.Sleep(time.Duration(2) * time.Second)

	return r.verifyDeployment(deploymentName, r.quayConfiguration.QuayEcosystem.Namespace)

}

func (r *ReconcileQuayEcosystemConfiguration) configurePostgreSQL(meta metav1.ObjectMeta) error {

	postgresqlPods := &corev1.PodList{}
	opts := &client.ListOptions{}
	opts.SetLabelSelector(fmt.Sprintf("%s=%s", constants.LabelCompoentKey, constants.LabelComponentQuayDatabaseValue))
	opts.InNamespace(r.quayConfiguration.QuayEcosystem.Namespace)

	err := r.reconcilerBase.GetClient().List(context.TODO(), opts, postgresqlPods)

	if err != nil {
		return err
	}

	postgresqlPodsItems := postgresqlPods.Items
	var podName string

	if len(postgresqlPodsItems) == 0 {
		return fmt.Errorf("Failed to locate any active PostgreSQL Pod")
	}

	podName = postgresqlPodsItems[0].Name

	//TODO enhance command to support Postgresql10
	success, stdout, stderr := k8sutils.ExecIntoPod(r.k8sclient, podName, fmt.Sprintf("echo \"SELECT * FROM pg_available_extensions\" | /opt/rh/rh-postgresql96/root/usr/bin/psql -d %s", r.quayConfiguration.QuayDatabase.Database), "", r.quayConfiguration.QuayEcosystem.Namespace)

	if !success {
		return fmt.Errorf("Failed to Exec into Postgresql Pod: %s", stderr)
	}

	if strings.Contains(stdout, "pg_trim") {
		return nil
	}

	success, stdout, stderr = k8sutils.ExecIntoPod(r.k8sclient, podName, fmt.Sprintf("echo \"CREATE EXTENSION pg_trgm\" | /opt/rh/rh-postgresql96/root/usr/bin/psql -d %s", r.quayConfiguration.QuayDatabase.Database), "", r.quayConfiguration.QuayEcosystem.Namespace)

	if !success {
		return fmt.Errorf("Failed to add pg_trim extension: %s", stderr)
	}

	//Create Clair Database
	//TODO Convert Clair Database to Parameter
	success, stdout, stderr = k8sutils.ExecIntoPod(r.k8sclient, podName, fmt.Sprintf("echo \"create database clair\" | /opt/rh/rh-postgresql96/root/usr/bin/psql -d %s", r.quayConfiguration.QuayDatabase.Database), "", r.quayConfiguration.QuayEcosystem.Namespace)

	if !success {
		return fmt.Errorf("Failed to create database clair: %s", stderr)
	}

	return nil
}

func (r *ReconcileQuayEcosystemConfiguration) createQuayConfigSecret(meta metav1.ObjectMeta) error {

	configSecretName := resources.GetConfigMapSecretName(r.quayConfiguration.QuayEcosystem)

	meta.Name = configSecretName

	found := &corev1.Secret{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: configSecretName, Namespace: r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace}, found)

	if err != nil && apierrors.IsNotFound(err) {

		return r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, resources.GetSecretDefinition(meta))

	} else if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (r *ReconcileQuayEcosystemConfiguration) createClairConfigSecret(meta metav1.ObjectMeta) error {

	configSecretName := resources.GetClairConfigSecretName(r.quayConfiguration.QuayEcosystem)

	meta.Name = configSecretName

	found := &corev1.Secret{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: configSecretName, Namespace: r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace}, found)

	if err != nil && apierrors.IsNotFound(err) {

		return r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, resources.GetSecretDefinition(meta))

	} else if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (r *ReconcileQuayEcosystemConfiguration) createClairTrustCASecret(meta metav1.ObjectMeta) error {

	configSecretName := resources.GetClairTrustCASecretName(r.quayConfiguration.QuayEcosystem)

	meta.Name = configSecretName

	found := &corev1.Secret{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: configSecretName, Namespace: r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace}, found)

	if err != nil && apierrors.IsNotFound(err) {

		return r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, resources.GetSecretDefinition(meta))

	} else if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (r *ReconcileQuayEcosystemConfiguration) createSecurityScannerKeySecret(meta metav1.ObjectMeta) error {

	configSecretName := resources.GetSecurityScannerKeySecretName(r.quayConfiguration.QuayEcosystem)

	meta.Name = configSecretName

	found := &corev1.Secret{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: configSecretName, Namespace: r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace}, found)

	if err != nil && apierrors.IsNotFound(err) {

		return r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, resources.GetSecretDefinition(meta))

	} else if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (r *ReconcileQuayEcosystemConfiguration) configureAnyUIDSCCs(meta metav1.ObjectMeta) error {

	// Configure Quay Service Account for AnyUID SCC
	err := r.configureAnyUIDSCC(constants.QuayServiceAccount, meta)

	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createServiceAccounts(meta metav1.ObjectMeta) error {
	// Create Redis Service Account
	if utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Redis.Hostname) {
		err := r.createServiceAccount(constants.RedisServiceAccount, meta)

		if err != nil {
			return err
		}
	}

	// Create Quay Service Account
	err := r.createServiceAccount(constants.QuayServiceAccount, meta)

	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createServiceAccount(serviceAccountName string, meta metav1.ObjectMeta) error {

	meta.Name = serviceAccountName

	found := &corev1.ServiceAccount{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: serviceAccountName, Namespace: r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace}, found)

	if err != nil && apierrors.IsNotFound(err) {

		return r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, resources.GetServiceAccountDefinition(meta))

	} else if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createRBAC(meta metav1.ObjectMeta) error {

	role := resources.GetRoleDefinition(meta, r.quayConfiguration.QuayEcosystem)

	err := r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, role)
	if err != nil {
		return err
	}

	roleBinding := resources.GetRoleBindingDefinition(meta, r.quayConfiguration.QuayEcosystem)

	err = r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, roleBinding)
	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createQuayService(meta metav1.ObjectMeta) error {

	service := resources.GetQuayServiceDefinition(meta, r.quayConfiguration.QuayEcosystem)

	err := r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, service)
	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createQuayConfigService(meta metav1.ObjectMeta) error {

	service := resources.GetQuayConfigServiceDefinition(meta, r.quayConfiguration.QuayEcosystem)

	err := r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, service)
	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createClairService(meta metav1.ObjectMeta) error {

	service := resources.GetClairServiceDefinition(meta, r.quayConfiguration.QuayEcosystem)

	err := r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, service)
	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createQuayRoute(meta metav1.ObjectMeta) error {

	meta.Name = resources.GetQuayResourcesName(r.quayConfiguration.QuayEcosystem)

	route := resources.GetQuayRouteDefinition(meta, r.quayConfiguration.QuayEcosystem)

	err := r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, route)

	if err != nil {
		return err
	}

	time.Sleep(time.Duration(2) * time.Second)

	createdRoute := &routev1.Route{}
	err = r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: meta.Name, Namespace: r.quayConfiguration.QuayEcosystem.Namespace}, createdRoute)

	if err != nil {
		return err
	}

	if utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Quay.RouteHost) {
		r.quayConfiguration.QuayHostname = createdRoute.Spec.Host
	} else {
		r.quayConfiguration.QuayHostname = r.quayConfiguration.QuayEcosystem.Spec.Quay.RouteHost
	}

	r.quayConfiguration.QuayEcosystem.Status.Hostname = r.quayConfiguration.QuayHostname

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createQuayConfigRoute(meta metav1.ObjectMeta) error {

	route := resources.GetQuayConfigRouteDefinition(meta, r.quayConfiguration.QuayEcosystem)

	err := r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, route)

	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createClairRoute(meta metav1.ObjectMeta) error {

	meta.Name = resources.GetClairResourcesName(r.quayConfiguration.QuayEcosystem)

	route := resources.GetClairRouteDefinition(meta, r.quayConfiguration.QuayEcosystem)

	err := r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, route)

	if err != nil {
		return err
	}

	time.Sleep(time.Duration(2) * time.Second)

	createdRoute := &routev1.Route{}
	err = r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: meta.Name, Namespace: r.quayConfiguration.QuayEcosystem.Namespace}, createdRoute)

	if err != nil {
		return err
	}

	r.quayConfiguration.ClairHostname = createdRoute.Spec.Host

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) configureAnyUIDSCC(serviceAccountName string, meta metav1.ObjectMeta) error {

	sccUser := "system:serviceaccount:" + meta.Namespace + ":" + serviceAccountName

	anyUIDSCC := &ossecurityv1.SecurityContextConstraints{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: constants.AnyUIDSCC, Namespace: ""}, anyUIDSCC)

	if err != nil {
		logging.Log.Error(err, "Error occurred retrieving SCC")
		return err
	}

	sccUserFound := false
	for _, user := range anyUIDSCC.Users {
		if user == sccUser {

			sccUserFound = true
			break
		}
	}

	if !sccUserFound {
		anyUIDSCC.Users = append(anyUIDSCC.Users, sccUser)
		err = r.reconcilerBase.CreateOrUpdateResource(nil, r.quayConfiguration.QuayEcosystem.Namespace, anyUIDSCC)
		if err != nil {
			return err
		}
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) quayRegistryStorage(meta metav1.ObjectMeta) error {

	for _, registryBackend := range r.quayConfiguration.QuayEcosystem.Spec.Quay.RegistryBackends {

		if !utils.IsZeroOfUnderlyingType(registryBackend.RegistryBackendSource.Local) {
			registryVolumeName := resources.GetRegistryStorageVolumeName(r.quayConfiguration.QuayEcosystem, registryBackend.Name)

			meta.Name = registryVolumeName

			registryStoragePVC := resources.GetQuayPVCRegistryStorageDefinition(meta, r.quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.PersistentVolumeAccessModes, r.quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.PersistentVolumeSize, &r.quayConfiguration.QuayEcosystem.Spec.Quay.RegistryStorage.PersistentVolumeStorageClassName)

			err := r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, registryStoragePVC)

			if err != nil {
				return err
			}

		}

	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) removeQuayRegistryStorage(meta metav1.ObjectMeta) error {

	registryPVC := resources.GetQuayRegistryStorageName(r.quayConfiguration.QuayEcosystem)

	err := r.k8sclient.CoreV1().PersistentVolumeClaims(r.quayConfiguration.QuayEcosystem.Namespace).Delete(registryPVC, &metav1.DeleteOptions{})

	if err != nil && !apierrors.IsNotFound(err) {
		logging.Log.Error(err, "Error Deleting Quay Registry PVC", "Namespace", r.quayConfiguration.QuayEcosystem.Namespace, "Name", registryPVC)
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) ManageQuayEcosystemCertificates(meta metav1.ObjectMeta) (*reconcile.Result, error) {

	configSecretName := resources.GetConfigMapSecretName(r.quayConfiguration.QuayEcosystem)

	meta.Name = configSecretName

	appConfigSecret := &corev1.Secret{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: configSecretName, Namespace: r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace}, appConfigSecret)

	if err != nil {

		if apierrors.IsNotFound(err) {
			// Config Secret Not Found. Requeue object
			return &reconcile.Result{}, nil
		}
		return nil, err
	}

	if !isQuayCertificatesConfigured(appConfigSecret) {

		if utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Quay.SslCertificatesSecretName) {
			certBytes, privKeyBytes, err := cert.GenerateSelfSignedCertKey(constants.QuayEnterprise, []net.IP{}, []string{r.quayConfiguration.QuayHostname})
			if err != nil {
				logging.Log.Error(err, "Error creating public/private key")
				return nil, err
			}

			r.quayConfiguration.QuaySslCertificate = certBytes
			r.quayConfiguration.QuaySslPrivateKey = privKeyBytes

		}
	} else {
		if utils.IsZeroOfUnderlyingType(r.quayConfiguration.QuayEcosystem.Spec.Quay.SslCertificatesSecretName) {
			r.quayConfiguration.QuaySslPrivateKey = appConfigSecret.Data[constants.QuayAppConfigSSLPrivateKeySecretKey]
			r.quayConfiguration.QuaySslCertificate = appConfigSecret.Data[constants.QuayAppConfigSSLCertificateSecretKey]
		}

	}

	if appConfigSecret.Data == nil {
		appConfigSecret.Data = map[string][]byte{}
	}

	appConfigSecret.Data[constants.QuayAppConfigSSLPrivateKeySecretKey] = r.quayConfiguration.QuaySslPrivateKey
	appConfigSecret.Data[constants.QuayAppConfigSSLCertificateSecretKey] = r.quayConfiguration.QuaySslCertificate

	err = r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, appConfigSecret)

	if err != nil {
		logging.Log.Error(err, "Error Updating app secret with certificates")
		return nil, err
	}

	return nil, nil
}

func (r *ReconcileQuayEcosystemConfiguration) ManageClairConfig(meta metav1.ObjectMeta) (*reconcile.Result, error) {

	clairConfigSecretName := resources.GetClairConfigSecretName(r.quayConfiguration.QuayEcosystem)

	meta.Name = clairConfigSecretName

	clairConfigSecret := &corev1.Secret{}

	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: clairConfigSecretName, Namespace: r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace}, clairConfigSecret)

	if err != nil {

		if apierrors.IsNotFound(err) {
			// Config Secret Not Found. Requeue object
			return &reconcile.Result{}, nil
		}
		return nil, err
	}

	if clairConfigSecret.Data == nil {
		clairConfigSecret.Data = map[string][]byte{}
	}

	//TODO update paginationkey
	clairConfig := fmt.Sprintf(
		`clair:
  database:
    type: pgsql
    options:
      source: postgresql://%s:%s@%s:5432/clair?sslmode=disable
      cachesize: 16384
  api:
    healthport: 6061
    port: 6062
    timeout: 900s
    paginationkey: "XxoPtCUzrUv4JV5dS+yQ+MdW7yLEJnRMwigVY/bpgtQ="
  updater:
    interval: 6h
    notifier:
      attempts: 3
      renotifyinterval: 1h
      http:
        endpoint: https://%s/secscan/notify
        proxy: http://localhost:6063
jwtproxy:
  signer_proxy:
    enabled: true
    listen_addr: :6063
    ca_key_file: /certificates/mitm.key # Generated internally, do not change.
    ca_crt_file: /certificates/mitm.crt # Generated internally, do not change.
    signer:
      issuer: security_scanner
      expiration_time: 5m
      max_skew: 1m
      nonce_length: 32
      private_key:
        type: preshared
        options:
          key_id: %s
          private_key_path: /clair/config/security_scanner.pem
  verifier_proxies:
  - enabled: true
    listen_addr: :6060
    verifier:
      audience: http://%s
      upstream: http://localhost:6062
      key_server:
        type: keyregistry
        options:
          registry: https://%s/keys/`, r.quayConfiguration.QuayDatabase.Username, r.quayConfiguration.QuayDatabase.Password, r.quayConfiguration.QuayDatabase.Server, r.quayConfiguration.QuayHostname, r.quayConfiguration.SecurityScannerKeyKid, r.quayConfiguration.ClairHostname, r.quayConfiguration.QuayHostname)

	clairConfigSecret.Data[constants.ClairConfigKey] = []byte(clairConfig)

	err = r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, clairConfigSecret)

	if err != nil {
		logging.Log.Error(err, "Error Updating clair config secret")
		return nil, err
	}
	return nil, nil
}

func (r *ReconcileQuayEcosystemConfiguration) ManageClairTrustCA(meta metav1.ObjectMeta) (*reconcile.Result, error) {

	trustCASecretName := resources.GetClairTrustCASecretName(r.quayConfiguration.QuayEcosystem)

	meta.Name = trustCASecretName

	trustCASecret := &corev1.Secret{}

	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: trustCASecretName, Namespace: r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace}, trustCASecret)

	if err != nil {

		if apierrors.IsNotFound(err) {
			// Config Secret Not Found. Requeue object
			return &reconcile.Result{}, nil
		}
		return nil, err
	}

	if trustCASecret.Data == nil {
		trustCASecret.Data = map[string][]byte{}
	}

	trustCASecret.Data[constants.ClairTrustCASecretKey] = r.quayConfiguration.QuaySslCertificate

	err = r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, trustCASecret)

	if err != nil {
		logging.Log.Error(err, "Error Updating clair trust CA secret with certificates")
		return nil, err
	}
	return nil, nil
}

func (r *ReconcileQuayEcosystemConfiguration) ManageSecurityScannerKey(meta metav1.ObjectMeta) (*reconcile.Result, error) {

	securityScannerKeySecretName := resources.GetSecurityScannerKeySecretName(r.quayConfiguration.QuayEcosystem)

	meta.Name = securityScannerKeySecretName

	securityScannerKeySecret := &corev1.Secret{}

	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: securityScannerKeySecretName, Namespace: r.quayConfiguration.QuayEcosystem.ObjectMeta.Namespace}, securityScannerKeySecret)

	if err != nil {

		if apierrors.IsNotFound(err) {
			// Config Secret Not Found. Requeue object
			return &reconcile.Result{}, nil
		}
		return nil, err
	}

	if securityScannerKeySecret.Data == nil {
		securityScannerKeySecret.Data = map[string][]byte{}
	}

	securityScannerKeySecret.Data[constants.SecurityScannerKeySecretKey] = []byte(r.quayConfiguration.SecurityScannerKeyPrivateKey)

	err = r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, securityScannerKeySecret)

	if err != nil {
		logging.Log.Error(err, "Error Updating security scanner key secret")
		return nil, err
	}
	return nil, nil
}

func (r *ReconcileQuayEcosystemConfiguration) quayDeployment(meta metav1.ObjectMeta) error {

	quayDeployment := resources.GetQuayDeploymentDefinition(meta, r.quayConfiguration)

	err := r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, quayDeployment)

	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) quayConfigDeployment(meta metav1.ObjectMeta) error {

	if !r.quayConfiguration.ValidProvidedQuayConfigPasswordSecret {
		quayConfigSecret := resources.GetSecretDefinitionFromCredentialsMap(resources.GetQuayConfigResourcesName(r.quayConfiguration.QuayEcosystem), meta, constants.DefaultQuayConfigCredentials)

		err := r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, quayConfigSecret)

		if err != nil {
			return err
		}

	}

	quayDeployment := resources.GetQuayConfigDeploymentDefinition(meta, r.quayConfiguration)

	err := r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, quayDeployment)

	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) clairDeployment(meta metav1.ObjectMeta) error {

	clairDeployment := resources.GetClairDeploymentDefinition(meta, r.quayConfiguration)

	err := r.reconcilerBase.CreateOrUpdateResource(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, clairDeployment)

	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) createRedisService(meta metav1.ObjectMeta) error {

	service := resources.GetRedisServiceDefinition(meta, r.quayConfiguration.QuayEcosystem)

	err := r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, service)

	if err != nil {
		return err
	}

	return nil

}

func (r *ReconcileQuayEcosystemConfiguration) redisDeployment(meta metav1.ObjectMeta) (*reconcile.Result, error) {

	redisDeployment := resources.GetRedisDeploymentDefinition(meta, r.quayConfiguration)

	err := r.reconcilerBase.CreateResourceIfNotExists(r.quayConfiguration.QuayEcosystem, r.quayConfiguration.QuayEcosystem.Namespace, redisDeployment)
	if err != nil {
		return nil, err
	}

	time.Sleep(time.Duration(2) * time.Second)

	// Verify Deployment
	redisDeploymentName := resources.GetRedisResourcesName(r.quayConfiguration.QuayEcosystem)
	return r.verifyDeployment(redisDeploymentName, r.quayConfiguration.QuayEcosystem.Namespace)
}

// Verify Deployment
func (r *ReconcileQuayEcosystemConfiguration) verifyDeployment(deploymentName string, deploymentNamespace string) (*reconcile.Result, error) {

	deployment := &appsv1.Deployment{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Name: deploymentName, Namespace: deploymentNamespace}, deployment)

	if err != nil {
		return nil, err
	}

	if deployment.Status.AvailableReplicas != 1 {
		scaled := k8sutils.GetDeploymentStatus(r.k8sclient, deploymentNamespace, deploymentName)

		if !scaled {
			return &reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, nil
		}

	}

	return nil, nil

}

func isQuayCertificatesConfigured(secret *corev1.Secret) bool {

	if !utils.IsZeroOfUnderlyingType(secret) {
		if _, found := secret.Data[constants.QuayAppConfigSSLCertificateSecretKey]; !found {
			return false
		}

		if len(secret.Data[constants.QuayAppConfigSSLCertificateSecretKey]) == 0 {
			return false
		}

		if _, found := secret.Data[constants.QuayAppConfigSSLPrivateKeySecretKey]; !found {
			return false
		}

		if len(secret.Data[constants.QuayAppConfigSSLPrivateKeySecretKey]) == 0 {
			return false
		}

		return true

	}
	return false
}
