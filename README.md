# quay-operator

[![Build Status](https://travis-ci.org/theodor2311/quay-operator.svg?branch=master)](https://travis-ci.org/theodor2311/quay-operator) [![Docker Repository on Quay](https://quay.io/repository/redhat-cop/quay-operator/status "Docker Repository on Quay")](https://quay.io/repository/redhat-cop/quay-operator)

Operator to manage the lifecycle of [Quay](https://www.openshift.com/products/quay).

A dirty fork version from "redhat-cop/quay-operator" to create a Quay+Clair instance with shared postgreSQL for testing only. For full feature support please refer to https://github.com/redhat-cop/quay-operator.

## First, create the project
```
oc new-project quay-enterprise
```

## Second, prepare the pull secret
```
- Create the config secret. Refer https://access.redhat.com/solutions/3533201

#Method 1
docker login -u="<QUAY.IO_LOGIN>" -p="<QUAY.IO_PASSWORD>" quay.io
oc create secret generic redhat-pull-secret --from-file=".dockerconfigjson=$HOME/.docker/config.json" --type='kubernetes.io/dockerconfigjson' --namespace=quay-enterprise

#Method 2
Directly download the "redhat-quay-pull-secret" secret and import to the project

```
## Third, run this scripts
```bash
cd
git clone https://github.com/theodor2311/quay-operator.git
cd quay-operator
oc create -f deploy/crds/redhatcop_v1alpha1_quayecosystem_crd.yaml
oc create -f deploy/service_account.yaml
oc create -f deploy/cluster_role.yaml
oc create -f deploy/cluster_role_binding.yaml
oc create -f deploy/role.yaml
oc create -f deploy/role_binding.yaml
oc create -f deploy/operator.yaml
oc create -f deploy/crds/redhatcop_v1alpha1_quayecosystem_cr.yaml

```

## Cleanup
```bash
oc delete -f deploy/crds/redhatcop_v1alpha1_quayecosystem_crd.yaml
oc delete -f deploy/service_account.yaml
oc delete -f deploy/cluster_role.yaml
oc delete -f deploy/cluster_role_binding.yaml
oc delete -f deploy/role.yaml
oc delete -f deploy/role_binding.yaml
oc delete -f deploy/operator.yaml
oc delete project quay-enterprise
```
