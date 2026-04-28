<ol>
  <li>
<p>On the home page, click the "Install Deckhouse Kubernetes Platform" button.</p>
{% offtopic title="Where is this button located..." %}
<img src="/images/gs/installer/install-button.png" alt="Install Deckhouse Kubernetes Platform">
{% endofftopic %}
  </li>
  <li>
<p>Select the basic installation parameters:</p>
<ul>
  <li>Platform edition. Community Edition is selected by default; you can choose the required edition in the "Add source" drop-down list in the center.<br>
  If you choose an edition other than Community Edition, enter a name (any value) and the license key you received in the window that opens.
  {% offtopic title="What the license key input window looks like..." %}
<img src="/images/gs/installer/enter-license-key.png" alt="Basic installation parameters">
  {% endofftopic %}
  </li>
  <li><a href="../documentation/v1/reference/release-channels.html">Deckhouse Kubernetes Platform update channel</a>. Stable is selected by default.</li>
  <li>Kubernetes version. By default, the "Auto" mode is selected, which chooses the <a href="../documentation/v1/reference/supported_versions.html#kubernetes">currently recommended version</a>.</li>
</ul>
{% offtopic title="What this screen looks like..." %}
<img src="/images/gs/installer/main-parameters.png" alt="Basic installation parameters">
{% endofftopic %}
<p>Then click the "Infrastructure" button to proceed to the next step.</p>
  </li>
</ol>
