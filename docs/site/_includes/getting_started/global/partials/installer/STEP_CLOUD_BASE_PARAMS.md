<ol>
  <li>
    <p>In the "Add infrastructure" drop-down list at the top of the page, select the required cloud platform.<br>
    In this list, available cloud providers depend on the selected DKP edition.</p>
<img src="/images/gs/installer/select-cloud-provider.png" alt="What this drop-down list looks like">
  </li>
  <li>
    <p>In the panel that opens on the right, enter the required cloud provider connection parameters.</p>
<img src="/images/gs/installer/ya-cloud-example.png" alt="Yandex Cloud settings panel">
    <p>Click the "Save" button.</p>
  </li>
  <li>The created infrastructure appears in the list on the main screen. Select the required item (if multiple options were created) and click the "Cluster parameters" button.
<img src="/images/gs/installer/cloud-infrastructure.png" alt="What the infrastructure selection panel looks like">
  </li>
  <li>Set the cluster name and specify the parameters of the requested machines for future cluster nodes.<br>
  Advanced future cluster settings, such as placement scheme or subnet configuration, are available after clicking the "Additional settings" button.
<img src="/images/gs/installer/cloud-extended.png" alt="What the additional cluster settings panel looks like...">
  The "Advanced configuration" button on the left side of the screen lets you view and download generated YAML configuration files. This may be required to run <a href="../documentation/v1/installing/">dhctl</a> manually using these files.
<img src="/images/gs/installer/cloud-mega-setup.png" alt="What the advanced configuration panel looks like...">
  Click the "Install" button.
<img src="/images/gs/installer/cloud-settings.png" alt="What the node settings panel looks like...">
  </li>
</ol>
