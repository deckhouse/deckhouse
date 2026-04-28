<ol>
  <li>
    <p>In the "Add infrastructure" drop-down list at the top of the page, select the required cloud platform.<br>
    In this list, available cloud providers depend on the selected DKP edition.</p>
    {% offtopic title="What this drop-down list looks like..." %}
<img src="/images/gs/installer/select-cloud-provider.png" alt="What this drop-down list looks like">
    {% endofftopic %}
  </li>
  <li>
    <p>In the panel that opens on the right, enter the required cloud provider connection parameters.</p>
    {% offtopic title="What this panel looks like (Yandex Cloud example)..." %}
<img src="/images/gs/installer/ya-cloud-example.png" alt="Yandex Cloud settings panel">
    {% endofftopic %}
    <p>Click the "Save" button.</p>
  </li>
  <li>The created infrastructure appears in the list on the main screen. Select the required item (if multiple options were created) and click the "Cluster parameters" button.
  {% offtopic title="What selecting created infrastructure looks like..." %}
<img src="/images/gs/installer/cloud-infrastructure.png" alt="What the infrastructure selection panel looks like">
  {% endofftopic %}
  </li>
  <li>Set the cluster name and specify the parameters of the requested machines for future cluster nodes.<br>
  Advanced future cluster settings, such as placement scheme or subnet configuration, are available after clicking the "Additional settings" button.
  {% offtopic title="What the additional cluster settings panel looks like..." %}
<img src="/images/gs/installer/cloud-extended.png" alt="What the additional cluster settings panel looks like...">
  {% endofftopic %}
  {% offtopic title="What advanced configuration is for..." %}
  The "Advanced configuration" button on the left side of the screen lets you view and download generated YAML configuration files. This may be required to run <a href="../documentation/v1/installing/">dhctl</a> manually using these files.
<img src="/images/gs/installer/cloud-mega-setup.png" alt="What the advanced configuration panel looks like...">
  {% endofftopic %}
  Click the "Install" button.
  {% offtopic title="What the node settings panel looks like..." %}
<img src="/images/gs/installer/cloud-settings.png" alt="What the node settings panel looks like...">
  {% endofftopic %}
  </li>
</ol>
