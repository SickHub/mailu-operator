# mailu-operator

The purpose of this project is to define Email Domains, Users and Aliases used in Mailu via CRs.

The Mailu-Operator uses the Mailu API to create/update/delete Domains, Users and Aliases, it therefore needs the API 
endpoint and token which can be set through command line or the environment variables `MAILU_SERVER` and `MAILU_TOKEN`.

**Important note**: A user can still make changes in the Mailu frontend which are not synced back to the CRDs.
Also, some changes may be intended to be done "on-the-fly" in the Mailu frontend, for example setting auto reply or changing the password.

## Description

This operator adds three custom resources: `Domain`, `User` and `Alias` and each resource represents an object in Mailu API.
For details refer also to your Mailu API documentation: https://mailu.io/master/api.html

Domain fields and defaults (see [sample](config/samples/operator_v1alpha1_domain.yaml))
- Name (required)
- Comment
- MaxUsers = 0
- MaxAliases = 0
- MaxQuotaBytes = 0
- SignupEnabled = false
- Alternatives

User fields and defaults (see [sample](config/samples/operator_v1alpha1_user.yaml))
- Name (required)
- Domain (required)
- AllowSpoofing = false
- ChangePwNextLogin = true
- Comment
- DisplayedName
- Enabled = false
- EnableImap = true
- EnablePop = true
- ForwardDestination
- ForwardEnabled = false
- ForwardKeep = false
- GlobalAdmin = false
- Password (hash, excluded from updates)
- PasswordSecret (takes precedence over `RawPassword`, secret name in the current namespace)
- PasswordKey (key within the `PasswordSecret` which contains the password)
- QuotaBytes = 0
- QuotaBytesUsed (excluded from updates)
- RawPassword (excluded from updates; **optional**: if not set, a random password will be generated)
- ReplyBody (TODO: excluded from updates)
- ReplyEnabled = false (TODO: excluded from updates)
- ReplyEnddate (TODO: excluded from updates)
- ReplyStartdate (TODO: excluded from updates)
- ReplySubject (TODO: excluded from updates)
- SpamEnabled = false
- SpamMarkAsRead = false
- SpamThreshold

Alias fields and defaults (see [sample](config/samples/operator_v1alpha1_alias.yaml))
- Name (required)
- Domain (required)
- Comment
- Destination
- Wildcard = false

### Simplified flow

Using `Domain` as an example resource
```mermaid
flowchart LR
  CRD(Domain resource)
  Controller
  MailuAPI(Mailu API)
  
  User -- 1. create/update/delete Domain resource --> CRD
  
  Controller -- 2. watch Domain resources --> CRD
  Controller -- 3. get/create/update/delete Domain --> MailuAPI
  Controller -- 4. update resource status --> CRD
```

### How to use the resources

#### Domain

Domains defines the domain names known to the mail system.

At the current state, this project does not touch DNS records in any form, nor does it trigger generation of DKIM keys.
It might be interesting to automate DNS records in the future with `external-dns`: https://github.com/Mailu/Mailu/issues/547#issuecomment-1722539650

#### User

Basically any email address that should be able to receive or send emails on its address must be a user. The domain used must be configured.
Even if you only forward emails to an external address hosted elsewhere, you need to create a user (with `forwardDestination` set).

#### Alias

Aliases only work with domains and email addresses know to the system, i.e. you cannot define an alias to forward emails to an external address. 
For that, you need to create a user.

Aliases are used to route emails for multiple email addresses to a user (email) known to the system.

## Getting Started

### Prerequisites

- go version v1.21.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.
- A running installation of [Mailu](https://github.com/Mailu/Mailu) with API enabled.


## Try it out

As this project is brand new and in alpha stage, here are the current steps to try it out:

1. get and apply the `install.yaml` which contains the CRDs and the deployment of the operator.
2. edit the deployment to configure `MAILU_API` and `MAILU_TOKEN` environment variables.
3. create `Domain`, `User` and `Alias` resources that will be applied to your Mailu instance.

```shell
LATEST=https://raw.githubusercontent.com/SickHub/mailu-operator/main/dist/install.yaml
RELEASE=https://raw.githubusercontent.com/SickHub/mailu-operator/v0.0.2/dist/install.yaml
NAMESPACE=mailu
kubectl apply -n $NAMESPACE -f $RELEASE

# edit the deployment and set your Mailu API url and token
kubectl -n $NAMESPACE edit deployment mailu-operator-controller-manager

# now you can add Domain, User and Alias resources
kubectl apply -n $NAMESPACE -f config/samples/operator_v1alpha1_domain.yaml
kubectl apply -n $NAMESPACE -f config/samples/operator_v1alpha1_user.yaml
kubectl apply -n $NAMESPACE -f config/samples/operator_v1alpha1_alias.yaml

# remove the operator again
kubectl delete -n $NAMESPACE -f $RELEASE
```

Build and install the operator from your fork:
```shell
REGISTRY=<your-registry>
NAMESPACE=mailu
# 1. build the image and push it to your own registry
make docker-buildx IMG=$REGISTRY/mailu-operator:dev

# 2. build the `install.yaml` used to deploy the operator (including CRDs, Roles and Deployment)
make build-installer IMG=$REGISTRY/mailu-operator:dev

# 3. apply the `install.yaml` to your k8s cluster
kubectl apply -n $NAMESPACE -f dist/install.yaml

# 4. edit the `MAILU_API` and `MAILU_TOKEN` environment variables
kubectl -n $NAMESPACE edit deployment mailu-operator-controller-manager
```

Uninstall the operator
```shell
kubectl delete -n $NAMESPACE -f dist/install.yaml
```

## Development: Build and deploy on your cluster

To setup the project, `operator-sdk` was used to generate the structure and custom resource objects:
```shell
operator-sdk init --plugins=go/v4 --domain mailu.io --repo github.com/sickhub/mailu-operator
operator-sdk create api --group operator --version v1alpha1 --kind Domain --resource --controller
operator-sdk create api --group operator --version v1alpha1 --kind User --resource --controller
operator-sdk create api --group operator --version v1alpha1 --kind Alias --resource --controller
```

**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/mailu-operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/mailu-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Releasing / Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the release tag:

```sh
VERSION=0.0.2 make build-installer
```

2. Create a Pull-Request with the changes
  - at least `dist/install.yaml` and `config/manager/kustomization.yaml` containing the latest tag

3. Merge the Pull-Request
  - the image will be built and pushed to docker hub
  - a release with be created with generated release notes


## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

