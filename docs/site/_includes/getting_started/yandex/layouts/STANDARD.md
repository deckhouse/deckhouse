![resources](/images/gs/cloud-provider-yandex/layout-standard.png)
<!--- Source: https://docs.google.com/drawings/d/1WI8tu-QZYcz3DvYBNlZG4s5OKQ9JKyna7ESHjnjuCVQ/edit --->
{% alert level="danger" %}
Under this placement strategy, virtual machines access the Internet using a NAT gateway service in Yandex Cloud with a public (and single) source IP.
{% endalert %}

{% alert level="warning" %}
Because nodes are created without public IP addresses in this layout, the master node must be accessible over SSH from the machine where `dhctl` is running: either directly over a private network or through a bastion host.

If the master node is not directly accessible, run the installation with the `--ssh-bastion-host`, `--ssh-bastion-user`, and, if needed, `--ssh-bastion-port` parameters.
{% endalert %}
