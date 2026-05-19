<div>
  <ol>
<li>Start Docker on your computer</li>
<li>
  <p>Run the installer</p>
  <div class="tabs">
<button id="tab-button-mac" class="tab-button active" onclick="openTab(event, 'tab-mac'); openMacTab(event, 'tab-mac-content-container'); openMacDefaultTab();">macOS</button>
<button id="tab-button-linux" class="tab-button" onclick="openTab(event, 'tab-linux'); openLinuxTab(event, 'tab-linux-content-container'); openLinuxDefaultTab();">Linux</button>
<button id="tab-button-windows" class="tab-button" onclick="openTab(event, 'tab-windows')">Windows</button>
  </div>
  <div id="tab-mac" class="tab-content active">
<div class="tabs">
  <button id="tab-buttons-mac-container" class="tab-button active" onclick="openMacTab(event, 'tab-mac-content-container')">Container</button>
  <button id="tab-buttons-mac-script" class="tab-button" onclick="openMacTab(event, 'tab-mac-content-trdl')">Using trdl</button>
  <button id="tab-buttons-mac-file" class="tab-button" onclick="openMacTab(event, 'tab-mac-content-file')">File</button>
</div>
<div id="tab-mac-content-container" class="tab-content active">
{%- include getting_started/global/partials/installer/installer_rosetta_alert_ru.html %}
  <p>Run the command:</p>
  <p><b>If the installer container cannot access the network while VPN is enabled, follow this <a href="/products/kubernetes-platform/documentation/v1/faq.html#что-делать-если-при-включенном-vpn-контейнер-с-установщиком-не-м">instruction</a>.</b></p>
{% capture command %}
```bash
docker run --rm --pull always -v $HOME/.d8installer:$HOME/.d8installer -v /var/run/docker.sock:/var/run/docker.sock -p 127.0.0.1:8080:8080 registry.deckhouse.ru/deckhouse/installer:latest -r $HOME/.d8installer
```
{% endcapture %}
{{ command | markdownify }}
</div>
<div id="tab-mac-content-trdl" class="tab-content">
{%- include getting_started/global/partials/installer/installer_rosetta_alert_ru.html %}
  <p>Starting from version 0.5.0, the installer can be installed on your machine using <a href="https://ru.trdl.dev/">trdl</a>.</p>
  <ol>
<li>Install the <a href="https://ru.trdl.dev/quickstart.html#%D1%83%D1%81%D1%82%D0%B0%D0%BD%D0%BE%D0%B2%D0%BA%D0%B0-%D0%BA%D0%BB%D0%B8%D0%B5%D0%BD%D1%82%D0%B0">trdl client</a>.</li>
<li><p>Add the trdl repository:</p>
{% capture command %}
```bash
URL=https://deckhouse.ru/downloads/deckhouse-installer-trdl
ROOT_VERSION=1
ROOT_SHA512=62e4b351bd06ee962dca92c0650ecbd2bceca9a78c125836fa62186b046f07257015929c853eb8a6241d90d59b2995bb028389cdb30bfa9c0991b10ddc2c57bc
REPO=d8-installer
trdl add $REPO $URL $ROOT_VERSION $ROOT_SHA512
```
{% endcapture %}
{{ command | markdownify }}
</li>
<li>
  <p>Install the latest installer release from the early-access channel and verify that it works:</p>
{% capture command %}
```bash
. $(trdl use -d d8-installer 1 ea) && d8install version
```
{% endcapture %}
{{ command | markdownify }}
<p>If you do not want to run <code>. $(trdl use -d d8-installer 1 ea)</code> before each installer usage, add the line <code>source $(trdl use -d d8-installer 1 ea)</code> to your shell RC file.</p>
</li>
  </ol>
</div>
<div id="tab-mac-content-file" class="tab-content">
{%- include getting_started/global/partials/installer/installer_rosetta_alert_ru.html %}
  <p>Download the installer:
<a href="/downloads/installer/latest/darwin-arm64/d8install" class="download-btn">darwin-arm64</a>
<a href="/downloads/installer/latest/darwin-amd64/d8install" class="download-btn">darwin-amd64</a>
  </p>
  <p>Run it with the commands below:</p>
{% capture command %}
```bash
chmod +x d8install
xattr -c d8install
./d8install -b
```
{% endcapture %}
{{ command | markdownify }}
</div>
  </div>
  <div id="tab-linux" class="tab-content">
<div class="tabs">
  <button id="tab-buttons-linux-container" class="tab-button active" onclick="openLinuxTab(event, 'tab-linux-content-container')">Container</button>
  <button id="tab-buttons-linux-script" class="tab-button" onclick="openLinuxTab(event, 'tab-linux-content-trdl')">Using trdl</button>
  <button id="tab-buttons-linux-file" class="tab-button" onclick="openLinuxTab(event, 'tab-linux-content-file')">File</button>
</div>
<div id="tab-linux-content-container" class="tab-content active">
  <p>Run the command:</p>
  <p><b>If the installer container cannot access the network while VPN is enabled, follow this <a href="/products/kubernetes-platform/documentation/v1/faq.html#что-делать-если-при-включенном-vpn-контейнер-с-установщиком-не-м">instruction</a>.</b></p>
{% capture command %}
```bash
docker run --rm --pull always -v $HOME/.d8installer:$HOME/.d8installer -v /var/run/docker.sock:/var/run/docker.sock -p 127.0.0.1:8080:8080 registry.deckhouse.ru/deckhouse/installer:latest -r $HOME/.d8installer
```
{% endcapture %}
{{ command | markdownify }}
</div>
<div id="tab-linux-content-trdl" class="tab-content">
  <p>Starting from version 0.5.0, the installer can be installed on your machine using <a href="https://ru.trdl.dev/">trdl</a>.</p>
  <ol>
<li>Install the <a href="https://ru.trdl.dev/quickstart.html#%D1%83%D1%81%D1%82%D0%B0%D0%BD%D0%BE%D0%B2%D0%BA%D0%B0-%D0%BA%D0%BB%D0%B8%D0%B5%D0%BD%D1%82%D0%B0">trdl client</a>.</li>
<li><p>Add the trdl repository:</p>
{% capture command %}
```bash
URL=https://deckhouse.ru/downloads/deckhouse-installer-trdl
ROOT_VERSION=1
ROOT_SHA512=62e4b351bd06ee962dca92c0650ecbd2bceca9a78c125836fa62186b046f07257015929c853eb8a6241d90d59b2995bb028389cdb30bfa9c0991b10ddc2c57bc
REPO=d8-installer
trdl add $REPO $URL $ROOT_VERSION $ROOT_SHA512
```
{% endcapture %}
{{ command | markdownify }}
</li>
<li>
  <p>Install the latest installer release from the early-access channel and verify that it works:</p>
{% capture command %}
```bash
. $(trdl use -d d8-installer 1 ea) && d8install version
```
{% endcapture %}
{{ command | markdownify }}
<p>If you do not want to run <code>. $(trdl use -d d8-installer 1 ea)</code> before each installer usage, add the line <code>source $(trdl use -d d8-installer 1 ea)</code> to your shell RC file.</p>
</li>
  </ol>
</div>
<div id="tab-linux-content-file" class="tab-content">
  <p>Download the installer: <a href="/downloads/installer/latest/linux-amd64/d8install" class="download-btn">amd64</a></p>
  <p>Run it with the following commands:</p>
{% capture command %}
```bash
chmod +x d8install
./d8install -b
```
{% endcapture %}
{{ command | markdownify }}
</div>
  </div>
  <div id="tab-windows" class="tab-content">
{% alert level="info" %}
Before starting the container, make sure [Docker Desktop](https://docs.docker.com/desktop/setup/install/windows-install/) is installed and [WSL2 is enabled](https://learn.microsoft.com/ru-ru/windows/wsl/install#install-wsl-command).
{% endalert %}
<p>Run the command if you are using Command Prompt:</p>
{% capture command %}
```bash
docker run --rm --pull always -v /mnt/host/c/Users/%USERNAME%/.d8installer:/mnt/host/c/Users/%USERNAME%/.d8installer -v /var/run/docker.sock:/var/run/docker.sock -p 127.0.0.1:8080:8080 registry.deckhouse.ru/deckhouse/installer:latest -r /mnt/host/c/Users/%USERNAME%/.d8installer
```
{% endcapture %}
{{ command | markdownify }}
<p>If you are using PowerShell:</p>
{% capture command %}
```bash
docker run --rm --pull always -v /mnt/host/c/Users/$env:USERNAME/.d8installer:/mnt/host/c/Users/$env:USERNAME/.d8installer -v /var/run/docker.sock:/var/run/docker.sock -p 127.0.0.1:8080:8080 registry.deckhouse.ru/deckhouse/installer:latest -r /mnt/host/c/Users/$env:USERNAME/.d8installer
```
{% endcapture %}
{{ command | markdownify }}
  </div>
</li>
<li>Open <a href="http://localhost:8080">http://localhost:8080</a></li>
</ol>
</div>

<script>
  function openTab(evt, tabName) {
  document.getElementById("tab-mac").classList.remove("active");
  document.getElementById("tab-linux").classList.remove("active");
  document.getElementById("tab-windows").classList.remove("active");

  document.getElementById("tab-button-mac").classList.remove("active");
  document.getElementById("tab-button-linux").classList.remove("active");
  document.getElementById("tab-button-windows").classList.remove("active");

  document.getElementById(tabName).classList.add("active");
  evt.currentTarget.classList.add("active");
  }

  function openMacTab(evt, subTabName) {
  document.getElementById("tab-mac-content-container").classList.remove("active");
  document.getElementById("tab-mac-content-trdl").classList.remove("active");
  document.getElementById("tab-mac-content-file").classList.remove("active");

  document.getElementById("tab-buttons-mac-container").classList.remove("active");
  document.getElementById("tab-buttons-mac-script").classList.remove("active");
  document.getElementById("tab-buttons-mac-file").classList.remove("active");

  document.getElementById(subTabName).classList.add("active");
  evt.currentTarget.classList.add("active");
  }

  function openLinuxTab(evt, subTabName) {
  document.getElementById("tab-linux-content-container").classList.remove("active");
  document.getElementById("tab-linux-content-trdl").classList.remove("active");
  document.getElementById("tab-linux-content-file").classList.remove("active");

  document.getElementById("tab-buttons-linux-container").classList.remove("active");
  document.getElementById("tab-buttons-linux-script").classList.remove("active");
  document.getElementById("tab-buttons-linux-file").classList.remove("active");

  document.getElementById(subTabName).classList.add("active");
  evt.currentTarget.classList.add("active");
  }

  function openInstallerTypeTab(evt, subTabName) {
  document.getElementById("tab-bare-metal-content").classList.remove("active");
  document.getElementById("tab-cloud-content").classList.remove("active");

  document.getElementById("tab-button-bare-metal").classList.remove("active");
  document.getElementById("tab-button-cloud").classList.remove("active");

  document.getElementById(subTabName).classList.add("active");
  evt.currentTarget.classList.add("active");
  }

  function openLinuxDefaultTab() {
  document.getElementById("tab-buttons-linux-container").classList.add("active");
  }
  function openMacDefaultTab() {
  document.getElementById("tab-buttons-mac-container").classList.add("active");
  }
</script>
