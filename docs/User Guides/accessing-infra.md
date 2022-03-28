# Accessing Infra

### 1. Install Infra CLI

<details>
  <summary><strong>macOS</strong></summary>

```bash
brew install infrahq/tap/infra
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
sudo echo 'deb [trusted=yes] https://apt.fury.io/infrahq/ /' >/etc/apt/sources.list.d/infrahq.list
sudo apt update
sudo apt install infra
```

```bash
# Fedora & Red Hat Enterprise Linux
sudo dnf config-manager --add-repo https://yum.fury.io/infrahq/
sudo dnf install infra
```

</details>

### 2. Login to your Infra host

```
infra login HOST
```

[block:callout]
{
"type": "info",
"body": "Ask your Infra administrator for the hostname that you should use to login.",
"title": "Don't know your Infra host?"
}
[/block]

### 3. See what you can access

```
infra list
```

### 4. Switch to the cluster context you want

```
infra use DESTINATION
```

### 5. Use your preferred tool to run commands

```
# for example, you can run kubectl commands directly after the infra context is set
kubectl [command]
```