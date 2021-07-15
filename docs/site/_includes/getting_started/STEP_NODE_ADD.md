Most modules **cannot operate correctly** in the cluster with just one master node since the node selection [strategy](/en/documentation/v1/#advanced-scheduling) prevents hosting the resources of those modules on master nodes.

You must add one (or more) non-master nodes to the cluster so that modules of the Deckhouse Platform can operate correctly.

{% if page.platform_type == 'cloud' %}
Note that Deckhouse automatically adds nodes to the cluster according to the NodeGroup parameters.

Thus, you have to [create a NodeGroup](/en/documentation/v1/modules/040-node-manager/usage.html#an-example-of-the-nodegroup-configuration) to add a node to the cluster.
{%- else %}
Create a NodeGroup and run a dedicated script to add one or more static nodes (dedicated servers, virtual machines, etc.) to the cluster.

[This manual](/en/documentation/v1/modules/040-node-manager/faq.html#how-do-i-automatically-add-a-static-node-to-a-cluster) will guide you through the process of adding a node to the cluster.
{%- endif %}
