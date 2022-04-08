## Quickstart

**Prerequisites:**
* Install [Helm](https://helm.sh/) (v3+)
* Install [Kubernetes](https://kubernetes.io/) (v1.14+)


### 1. Self-host Infra

```
helm repo add infrahq https://helm.infrahq.com/
helm repo update
helm install infra infrahq/infra
```

### 2. Install Infra CLI

<details>
  <summary><strong>macOS</strong></summary>

  ```bash
  brew install infrahq/tap/infra
  ```

  You may need to perform `brew link` if your symlinks are not working.
  ```bash
  brew link infrahq/tap/infra
  ```
</details>

<details>
  <summary><strong>Windows</strong></summary>

  ```powershell
  scoop bucket add infrahq https://github.com/infrahq/scoop.git
  scoop install infra
  ```

</details>

<details>
  <summary><strong>Linux</strong></summary>

  ```bash
  # Ubuntu & Debian
  echo 'deb [trusted=yes] https://apt.fury.io/infrahq/ /' | sudo tee /etc/apt/sources.list.d/infrahq.list
  sudo apt update
  sudo apt install infra
  ```
  ```bash
  # Fedora & Red Hat Enterprise Linux
  sudo dnf config-manager --add-repo https://yum.fury.io/infrahq/
  sudo dnf install infra
  ```
</details>

### 3. Login to Infra

```
infra login INFRA_URL --skip-tls-verify
```

Use the following command to find the Infra login URL. If you are not using a `LoadBalancer` service type, see the [Deploy Kubernetes guide](../operator-guides/deploy/kubernetes.md) for more information.

> Note: It may take a few minutes for the LoadBalancer endpoint to be assigned. You can watch the status of the service with:
> ```bash
> kubectl get service infra-server -w
> ```

```bash
kubectl get service infra-server -o jsonpath="{.status.loadBalancer.ingress[*]['ip', 'hostname']}"
```

This will output the admin access key which you can use to login in cases of emergency recovery. Please store this in a safe place as you will not see this again.

### 4. Connect your first Kubernetes cluster

In order to add connectors to Infra, you will need to set three pieces of information:

* `connector.config.name` is a name you give to identity this cluster. For the purposes of this Quickstart, the name will be `example-name`
* `connector.config.server` is the value in the previous step used to login to Infra
* `connector.config.accessKey` is the access key the connector will use to communicate with the server. You can use an existing access key or generate a new access key with `infra keys add KEY_NAME connector`

```bash
helm upgrade --install infra-connector infrahq/infra --set connector.config.server=INFRA_URL --set connector.config.accessKey=ACCESS_KEY --set connector.config.name=example-name --set connector.config.skipTLSVerify=true
```

### 5. Create the first local user

```
infra id add name@example.com
```

This creates a one-time password for the created user.

### 6. Grant Infra administrator privileges to the first user

```
infra grants add --user name@example.com --role admin infra
```

### 7. Grant Kubernetes cluster administrator privileges to the first user

```
infra grants add --user name@example.com --role cluster-admin kubernetes.example-name
```

<details>
  <summary><strong>
Supported Kubernetes cluster roles</strong></summary><br />

Infra supports any cluster roles within your Kubernetes environment, including custom ones. For simplicity, you can use cluster roles, and scope it to a particular namespace via Infra.

**Example applying a cluster role to a namespace:**
  ```
  infra grants add --user name@example.com --role edit kubernetes.example-name.namespace
  ```
**Default cluster roles within Kubernetes:**
- **cluster-admin** <br /><br />
  Allows super-user access to perform any action on any resource. When the 'cluster-admin' role is granted without specifying a namespace, it gives full control over every resource in the cluster and in all namespaces. When it is granted with a specified namespace, it gives full control over every resource in the namespace, including the namespace itself.<br /><br />
- **admin** <br /><br />
  Allows admin access, intended to be granted within a namespace.
The admin role allows read/write access to most resources in the specified namespace, including the ability to create roles and role bindings within the namespace. This role does not allow write access to resource quota or to the namespace itself.<br /><br />
- **edit** <br /><br />
  Allows read/write access to most objects in a namespace.
This role does not allow viewing or modifying roles or role bindings. However, this role allows accessing Secrets and running Pods as any ServiceAccount in the namespace, so it can be used to gain the API access levels of any ServiceAccount in the namespace.<br /><br />
- **view** <br /><br />
  Allows read-only access to see most objects in a namespace. It does not allow viewing roles or role bindings.
This role does not allow viewing Secrets, since reading the contents of Secrets enables access to ServiceAccount credentials in the namespace, which would allow API access as any ServiceAccount in the namespace (a form of privilege escalation).
</details>


### 8. Login to Infra with the newly created user

```
infra login
```

Select the Infra instance, and login with username/password.

### 9. Use your Kubernetes clusters

You can now access the connected Kubernetes clusters via your favorite tools directly. Infra in the background automatically synchronizes your Kubernetes configuration file (kubeconfig).

Alternatively, you can switch Kubernetes contexts by using the `infra use` command:

```
infra use kubernetes.example-name
```

<details>
  <summary><strong>Here are some other commands to get you started</strong></summary><br />

See the cluster(s) you have access to:
```
infra list
```
See the cluster(s) connected to Infra:
```
infra destinations list
```
See who has access to what via Infra:
```
infra grants list

Note: this requires the user to have the admin role within Infra.

An example to grant the permission:
infra grants add --user name@example.com --role admin infra
```
</details>

### 10. Share the cluster(s) with other developers

To share access with Infra, developers will need to install Infra CLI, and be provided the login URL. If using local users, please share the one-time password.