## Installation process

You will need:
1. **Personal computer**.
 
   The computer from which the installation will be performed. It is only needed to run the Deckhouse installer, and will not be part of the cluster.
   
   Requirements:
   - HTTPS-access to the container registry `registry.deckhouse.io` (it is also possible to use a [third-party registry](/en/documentation/v1/deckhouse-faq.html#how-do-i-configure-deckhouse-to-use-a-third-party-registry));
   - SSH key access to the node, the **master node** of the future cluster.  
2. **Master-node**.
 
   A server {% if page.platform_type == 'baremetal' or page.platform_type == 'existing' %}(physical server or virtual machine){% else %}(virtual machine){% endif %}, the master node of the future cluster. During the installation, the Deckhouse installer running on the **personal computer** (section 1) will connect to that host via SSH, install necessary packages, configure control plane Kubernetes and deploy Deckhouse. Installation **from a master node** is currently **not supported**.

   Requirements: 
   - at least 4 CPU cores;
   - at least 8 GB of RAM;
   - at least 40 GB of disk space for the cluster and etcd data;
   - OS: Ubuntu Linux 16.04/18.04/20.04 LTS or CentOS 7. 
   - HTTPS-access to the container registry `registry.deckhouse.io` (it is also possible to use a [third-party registry](/en/documentation/v1/deckhouse-faq.html#how-do-i-configure-deckhouse-to-use-a-third-party-registry));
   - SSH key access from the **personal computer** (section 1). 

3. Additional nodes (not required).   
{% if page.platform_type != 'baremetal' %}
   Depending on the purpose of the cluster and selected node layout in the next steps, additional nodes will be automatically ordered from the selected cloud provider.
{%- else %}
   Depending on the purpose of the cluster, you may need additional nodes, for example, dedicated nodes for monitoring, a load balancer, etc.    
{%- endif %}

The presentation below is an overview of the actions that will be required to install Deckhouse Platform. While it's totally fine to skip it, we recommend that you watch it to get a better understanding during the following steps.

Please also note, it's just a brief, rough overview. The actual actions and commands to be executed will be given during next steps.

<iframe src="{{ include.presentation }}" frameborder="0" width="{{ include.width }}" height="{{ include.height }}" allowfullscreen="true" mozallowfullscreen="true" webkitallowfullscreen="true"></iframe>

<p class="text text_alt" style="color: #2A5EFF">
  <img src="/images/icons/arrow-up.svg" alt="" style="width: 25px;margin-left: 59px;position: relative;top: -2px;">
  Control presentation
</p>

To start the process of installation of your Kubernetes cluster, please click the "Next" button below.
