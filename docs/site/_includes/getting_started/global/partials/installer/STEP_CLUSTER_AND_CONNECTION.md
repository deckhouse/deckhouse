<ol>
<li>
  <p>By default, the installation option "Static cluster on existing servers" is selected. Leave this option unchanged.</p>
<div style="width: 70%; margin: 16px auto;">
<img src="/images/gs/installer/select-infrastructure.png" alt="What this drop-down list looks like" style="width: 100%;">
</div>
  <p>Go to the next screen by clicking the "Cluster parameters" button.</p>
</li>
<li>
  <p>Set the cluster name and specify the IP addresses of the machines for future cluster nodes.<br>
  You can remove extra nodes by clicking the red trash icon next to the IP address input field.<br>
  Advanced future cluster settings, such as proxy server configuration or subnet settings, are available after clicking the "Additional settings" button.</p>
<div style="width: 70%; margin: 16px auto;">
<img src="/images/gs/installer/extended-settings.png" alt="What the advanced cluster settings panel looks like..." style="width: 100%;">
</div>
  <p>Below, you can configure SSH connection settings for cluster nodes by selecting an existing key added earlier or creating a new one on the same screen.</p>
  <ul>
  <li>"Preconfigured username" — the username used for SSH login to machines for future cluster nodes.</li>
  <li>"Preconfigured user password" (for sudo) — the user's password, if set. It is used to escalate privileges via sudo. <i>Leave empty if sudo does not require a password.</i></li>
  <li>"SSH key for node access" — the key used to connect to machines. Here you can select a previously added key, generate a new one, or provide an existing key.
  The private key is stored in the `~/.ssh/SSH_PRIVATE_KEY_FILE` file. You can get it with the `cat ~/.ssh/SSH_PRIVATE_KEY_FILE` command. Example output (for an ED25519-encrypted key):
{% capture command %}
```bash
$ cat ~/.ssh/id_ed25519
-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
...
AAAEB3AcmUCQ9dd7fPhIYpQe1pBhZEanld6ZgJHmyD8XO460T3766IVjzrA+Vpgvu1l2IL
RTAeNZpi2e6dqGhsbK6cAAAAGHpoYmVydEB6aGJlcnQtMjB3bnMxeGowOQECAwQF
-----END OPENSSH PRIVATE KEY-----
```
{% endcapture %}
  {{ command | markdownify }}
  Copy the full output text into the form field, including the `-----BEGIN OPENSSH PRIVATE KEY-----` and `-----END OPENSSH PRIVATE KEY-----` lines.
  </li>
  <li>"SSH port" — the port used for SSH connection. <i>Leave the default value if the machine uses the standard port.</i></li>
  <li>"Use SSH bastion" — SSH bastion settings. If you do not use an intermediate server to access resources in a private network, keep this toggle disabled. If you do, enable it and provide settings in the opened section.
  </li>
  </ul>
<div style="width: 70%; margin: 16px auto;">
<img src="/images/gs/installer/ssh-settings.png" alt="SSH settings panel" style="width: 100%;">
</div>
  When installing on bare metal, you can configure Ingress controller settings and incoming traffic handling during installation.
  Enable the "Incoming traffic" checkbox. In the opened section, configure the Ingress controller to be created by selecting its operating mode and the node group where it will run.
<div style="width: 70%; margin: 16px auto;">
<img src="/images/gs/installer/set-up-ingress.png" alt="What the Ingress controller settings section looks like..." style="width: 100%;">
</div>
  You can also configure the domain name template for web interfaces of the future cluster. To do this, enable the "Access to module web interfaces" checkbox and specify the corresponding settings.
<div style="width: 70%; margin: 16px auto;">
<img src="/images/gs/installer/web-interfaces.png" alt="What the domain name settings look like..." style="width: 100%;">
</div>
  If you need to create a user for web interface login, enable the "Create user" checkbox and specify username and password (the password can be generated automatically).
  The "Advanced configuration" button on the left side of the screen lets you view and download generated YAML configuration files. This may be required to run <a href="../documentation/v1/installing/">dhctl</a> manually using these files.
<div style="width: 70%; margin: 16px auto;">
<img src="/images/gs/installer/mega-settings.png" alt="What the advanced configuration panel looks like..." style="width: 100%;">
</div>
</li>
</ol>
