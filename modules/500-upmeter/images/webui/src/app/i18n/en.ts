const langPack = {
  group: {
    "control-plane": `<p><b>Cluster control-plane is available. Self-healing is working.</b></p>
<p>Group result is a combination of probe results with priority of the worst results.</p>
<ul class="tooltip-list">
<li><i class="fa fa-fw fa-square tooltip-up"></i><b>Up</b>: all probes are Up.</li>
<li><i class="fa fa-fw fa-square tooltip-down"></i><b>Down</b>: at least one probe is Down.</li>
<li><i class="fa fa-fw fa-square tooltip-unknown"></i><b>Unknown</b>: uncertain result, see probe results for clues.</li>
<li><i class="fa fa-fw fa-square tooltip-nodata"></i><b>Nodata</b>: all agents were stopped.</li>
</ul>`,
    "synthetic": `<p><b>Availability of sample application running in cluster.</b></p>
<p>Group result is a combination of probe results with priority of the worst results.</p>
<ul class="tooltip-list">
<li><i class="fa fa-fw fa-square tooltip-up"></i><b>Up</b>: all probes are Up.</li>
<li><i class="fa fa-fw fa-square tooltip-down"></i><b>Down</b>: at least one probe is Down.</li>
<li><i class="fa fa-fw fa-square tooltip-unknown"></i><b>Unknown</b>: uncertain result, see probe results for clues.</li>
<li><i class="fa fa-fw fa-square tooltip-nodata"></i><b>Nodata</b>: all agents were stopped.</li>
</ul>`,
  },
  probe: {
    "control-plane": {
      "access": `<p><b>API-server is available</b></p>
<p>Probe request /version endpoint from API server.</p>
<ul class="tooltip-list">
<li><i class="fa fa-fw fa-square tooltip-up"></i><b>Up</b>: <code>/version</code> returns 200 OK.</li>
<li><i class="fa fa-fw fa-square tooltip-down"></i><b>Down</b>: <code>/version</code> returns other codes or there is no connection to API server.</li>
<li><i class="fa fa-fw fa-square tooltip-unknown"></i><b>Unknown</b>: connection to API server is slow.</li>
<li><i class="fa fa-fw fa-square tooltip-nodata"></i><b>Nodata</b>: when agents were stopped.</li>
</ul>
        `,
      "basic-functionality": `<p><b>Kubernetes resources can be created and deleted</b></p>
<p>Every 5s a ConfigMap is created and deleted.</p>
<ul class="tooltip-list">
<li><i class="fa fa-fw fa-square tooltip-up"></i><b>Up</b>: <code>ConfigMap</code> successfully created and deleted.</li>
<li><i class="fa fa-fw fa-square tooltip-down"></i><b>Down</b>: error during create or delete operations.</li>
<li><i class="fa fa-fw fa-square tooltip-unknown"></i><b>Unknown</b>: API server is not available or probe execution is skipped because previous probe was not yet finished.</li>
<li><i class="fa fa-fw fa-square tooltip-nodata"></i><b>Nodata</b>: when agents were stopped.</li>
</ul>`,
      "control-plane-manager": `<p><b>kube-controller-manager is working</b></p>
<p>Every 1 minute a Deployment is created and deleted.</p>
<ul class="tooltip-list">
<li><i class="fa fa-fw fa-square tooltip-up"></i><b>Up</b>: <code>Deployment</code> successfully created and <code>Pod</code> become <code>Pending</code>. Deployment successfully deleted.</li>
<li><i class="fa fa-fw fa-square tooltip-down"></i><b>Down</b>: created <code>Pod</code> is not become <code>Pending</code> after 10 seconds.</li>
<li><i class="fa fa-fw fa-square tooltip-unknown"></i><b>Unknown</b>: error during create or delete operations, API server is not available or probe execution is skipped because previous probe was not yet finished.</li>
<li><i class="fa fa-fw fa-square tooltip-nodata"></i><b>Nodata</b>: when agents were stopped.</li>
</ul>`,
      "namespace": `<p><b>Namespace can be created and deleted</b></p>
<p>Every 1 minute a Namespace is created and deleted.</p>
<ul class="tooltip-list">
<li><i class="fa fa-fw fa-square tooltip-up"></i><b>Up</b>: <code>Namespace</code> successfully created and deleted.</li>
<li><i class="fa fa-fw fa-square tooltip-down"></i><b>Down</b>: created <code>Namespace</code> is not deleted after 60 seconds.</li>
<li><i class="fa fa-fw fa-square tooltip-unknown"></i><b>Unknown</b>:  error during create or delete operations, API server is not available or operations are slow.</li>
<li><i class="fa fa-fw fa-square tooltip-nodata"></i><b>Nodata</b>: when agents were stopped.</li>
</ul>`,
      "scheduler": `<p><b>kube-scheduler is working: a Pod can be created and scheduled on matching Node</b></p>
<p>Every 1 minute a Pod is created and deleted.</p>
<ul class="tooltip-list">
<li><i class="fa fa-fw fa-square tooltip-up"></i><b>Up</b>: <code>Pod</code> successfully created and scheduled to the <code>Node</code>.</li>
<li><i class="fa fa-fw fa-square tooltip-down"></i><b>Down</b>: created <code>Pod</code> is not scheduled within 20 seconds.</li>
<li><i class="fa fa-fw fa-square tooltip-unknown"></i><b>Unknown</b>:  error during create or delete operations, API server is not available or probe execution is skipped because previous probe was not yet finished.</li>
<li><i class="fa fa-fw fa-square tooltip-nodata"></i><b>Nodata</b>: when agents were stopped.</li>
</ul>`
    },
    synthetic: {
      "access": `<p><b>Sample application is up and response with 200 OK</b></p>
<p>Every 5 seconds resolve sample application IPs and request <code>/</code> endpoint until first success.</p>
<ul class="tooltip-list">
<li><i class="fa fa-fw fa-square tooltip-up"></i><b>Up</b>: resolve is successful and all IPs are responded with 200OK.</li>
<li><i class="fa fa-fw fa-square tooltip-down"></i><b>Down</b>: some IP responded with other code.</li>
<li><i class="fa fa-fw fa-square tooltip-unknown"></i><b>Unknown</b>:  error during resolve or application is responded slow.</li>
<li><i class="fa fa-fw fa-square tooltip-nodata"></i><b>Nodata</b>: when agents were stopped.</li>
</ul>`,
      "dns": `<p><b>Sample application is discoverable via Service and responded via resolved IPs</b></p>
<p>Every 5 seconds resolve sample application IPs and request <code>/dns</code> endpoint until first success.</p>
<ul class="tooltip-list">
<li><i class="fa fa-fw fa-square tooltip-up"></i><b>Up</b>: resolve is successful and all IPs are responded with 200OK.</li>
<li><i class="fa fa-fw fa-square tooltip-down"></i><b>Down</b>: some IP responded with other code.</li>
<li><i class="fa fa-fw fa-square tooltip-unknown"></i><b>Unknown</b>:  error during resolve or application is responded slow.</li>
<li><i class="fa fa-fw fa-square tooltip-nodata"></i><b>Nodata</b>: when agents were stopped.</li>
</ul>`,
      "neighbor": `<p><b>Sample application's Pods can communicate</b></p>
<p>Every 5 seconds resolve sample application IPs and request <code>/neighbor</code> endpoint until first success.</p>
<ul class="tooltip-list">
<li><i class="fa fa-fw fa-square tooltip-up"></i><b>Up</b>: resolve is successful and all IPs are responded with 200OK.</li>
<li><i class="fa fa-fw fa-square tooltip-down"></i><b>Down</b>: some IP resonded with other code.</li>
<li><i class="fa fa-fw fa-square tooltip-unknown"></i><b>Unknown</b>:  error during resolve or application is responded slow.</li>
<li><i class="fa fa-fw fa-square tooltip-nodata"></i><b>Nodata</b>: when agents were stopped.</li>
</ul>`,
      "neighbor-via-service": `<p><b>Sample application's Pods can communicate via Service with ClusterIP</b></p>
<p>Every 5 seconds resolve sample application IPs and request <code>/neighbor-via-service</code> endpoint until first success.</p>
<ul class="tooltip-list">
<li><i class="fa fa-fw fa-square tooltip-up"></i><b>Up</b>: resolve is successful and all IPs are responded with 200OK.</li>
<li><i class="fa fa-fw fa-square tooltip-down"></i><b>Down</b>: some IP resonded with other code.</li>
<li><i class="fa fa-fw fa-square tooltip-unknown"></i><b>Unknown</b>:  error during resolve or application is responded slow.</li>
<li><i class="fa fa-fw fa-square tooltip-nodata"></i><b>Nodata</b>: when agents were stopped.</li>
</ul>`
    }
  },
  mute: {
    items:{
      "Acd": {
        id: "Acd",
        label: "Accident",
        tooltip: `Mute <code>Down</code> seconds caused by <code>Accident</code>: a downtime caused by unforeseen cluster crash.`
      },
      "Mnt": {
        id: "Mnt",
        label: "Maintenance",
        tooltip: `Mute <code>Down</code> seconds caused by <code>Maintenance</code>: a downtime caused by scheduled cluster maintenance jobs.`
      },
      "InfAcd": {
        id: "InfAcd",
        label: "Infrastructure Accident",
        tooltip: `Mute <code>Down</code> seconds caused by <code>Infrastructure Accident</code>: a downtime caused by infrastructure crashes and reported by provider.`
      },
      "InfMnt": {
        id: "InfMnt",
        label: "Infrastructure Maintenance",
        tooltip: `Mute <code>Down</code> seconds caused by <code>Infrastructure Maintenance</code>: a downtime caused by scheduled maintenance jobs done by an infrastructure provider.`
      }
    },
    order: ["Acd", "Mnt", "InfAcd", "InfMnt"],
    menu: {
      label: "Mute",
      tooltip: `Choose types of Downtime objects in cluster that will be used to mute Down seconds.`,
    }
  },

}

export default langPack;
