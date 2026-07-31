<ol>
  <li>
<p>On the home page, click the "Install Deckhouse Kubernetes Platform" button.</p>
<div style="width: 70%; margin: 16px auto;">
<img src="/images/gs/installer/install-button.png" alt="Install Deckhouse Kubernetes Platform" style="width: 100%;">
</div>
  </li>
  <li>
<p>Select the basic installation parameters:</p>
<ul>
  <li>Platform edition. Community Edition is selected by default; you can choose the required edition in the "Add source" drop-down list in the center.<br>
  If you choose an edition other than Community Edition, enter a name (any value) and the license key you received in the window that opens.
<div style="width: 70%; margin: 16px auto;">
<img src="/images/gs/installer/enter-license-key.png" alt="Basic installation parameters" style="width: 100%;">
</div>
  </li>
  <li><a href="../../../documentation/v1/reference/release-channels.html">Deckhouse Kubernetes Platform update channel</a>. Stable is selected by default.</li>
  <li>Kubernetes version. By default, the "Auto" mode is selected, which chooses the <a href="../../../documentation/v1/reference/supported_versions.html#kubernetes">currently recommended version</a>.</li>
</ul>
<div style="width: 70%; margin: 16px auto;">
<img src="/images/gs/installer/main-parameters.png" alt="Basic installation parameters" style="width: 100%;">
</div>
<p>Then click the "Infrastructure" button to proceed to the next step.</p>
  </li>
  <li>
    <p>In the "Add infrastructure" drop-down list at the top of the page, select the required cloud platform.<br>
    In this list, available cloud providers depend on the selected DKP edition.</p>
<div style="width: 70%; margin: 16px auto;">
<img src="/images/gs/installer/select-cloud-provider.png" alt="What this drop-down list looks like" style="width: 100%;">
</div>
  </li>
  <li>
    <p>In the panel that opens on the right, enter the required cloud provider connection parameters.</p>
<div style="width: 70%; margin: 16px auto;">
{%- if page.platform_code == 'yandex' %}
<img src="/images/gs/installer/ya-cloud-example.png" alt="Yandex Cloud settings panel" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'dvp-provider' %}
<img src="/images/gs/installer/dvp-cloud-example.png" alt="DVP settings panel" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'openstack' %}
<img src="/images/gs/installer/openstack-cloud-example.png" alt="OpenStack settings panel" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'openstack_selectel' %}
<img src="/images/gs/installer/selectel-cloud-example.png" alt="Selectel settings panel" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'openstack_vk' %}
<img src="/images/gs/installer/vk-cloud-example.png" alt="VK Cloud settings panel" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'vsphere' %}
<img src="/images/gs/installer/vsphere-cloud-example.png" alt="VMware vSphere settings panel" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'vcd' %}
<img src="/images/gs/installer/vcd-cloud-example.png" alt="VMware Cloud Director settings panel" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'zvirt' %}
<img src="/images/gs/installer/zvirt-cloud-example.png" alt="zVirt settings panel" style="width: 100%;">
{%- endif %}
</div>
    <p>Click the "Save" button.</p>
  </li>
  <li>The created infrastructure appears in the list on the main screen. Select the required item (if multiple options were created) and click the "Cluster parameters" button.
<div style="width: 70%; margin: 16px auto;">
{%- if page.platform_code == 'yandex' %}
<img src="/images/gs/installer/ya-cloud-infrastructure.png" alt="What the infrastructure selection panel looks like" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'dvp-provider' %}
<img src="/images/gs/installer/dvp-cloud-infrastructure.png" alt="What the infrastructure selection panel looks like" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'openstack' %}
<img src="/images/gs/installer/openstack-cloud-infrastructure.png" alt="What the infrastructure selection panel looks like" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'openstack_selectel' %}
<img src="/images/gs/installer/selectel-cloud-infrastructure.png" alt="What the infrastructure selection panel looks like" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'openstack_vk' %}
<img src="/images/gs/installer/vk-cloud-infrastructure.png" alt="What the infrastructure selection panel looks like" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'vsphere' %}
<img src="/images/gs/installer/vsphere-cloud-infrastructure.png" alt="What the infrastructure selection panel looks like" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'vcd' %}
<img src="/images/gs/installer/vcd-cloud-infrastructure.png" alt="What the infrastructure selection panel looks like" style="width: 100%;">
{%- endif %}
{%- if page.platform_code == 'zvirt' %}
<img src="/images/gs/installer/zvirt-cloud-infrastructure.png" alt="What the infrastructure selection panel looks like" style="width: 100%;">
{%- endif %}
</div>
  </li>
  <li>Set the cluster name and specify the parameters of the requested machines for future cluster nodes.<br>
  Advanced future cluster settings, such as placement scheme or subnet configuration, are available after clicking the "Additional settings" button.
<div style="width: 70%; margin: 16px auto;">
<img src="/images/gs/installer/cloud-extended.png" alt="What the additional cluster settings panel looks like..." style="width: 100%;">
</div>
  The "Advanced configuration" button on the left side of the screen lets you view and download generated YAML configuration files. This may be required to run <a href="../../../documentation/v1/installing/">dhctl</a> manually using these files.
<div style="width: 70%; margin: 16px auto;">
<img src="/images/gs/installer/cloud-mega-setup.png" alt="What the advanced configuration panel looks like..." style="width: 100%;">
</div>
  Click the "Install" button.
<div style="width: 70%; margin: 16px auto;">
<img src="/images/gs/installer/cloud-settings.png" alt="What the node settings panel looks like..." style="width: 100%;">
</div>
  </li>
</ol>
