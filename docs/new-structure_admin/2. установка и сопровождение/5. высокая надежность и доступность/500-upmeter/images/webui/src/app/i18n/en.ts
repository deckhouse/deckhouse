interface tooltipData {
  title: string
  description: string
  reasonUp: string
  reasonDown: string
  reasonUnknown: string
  reasonNodata: string
}

/**
 * tooltip wraps the markup
 *
 * Note the punctuation in status slots.
 */
function tooltip(data: tooltipData): string {
  const {title, description, reasonUp, reasonDown, reasonUnknown, reasonNodata} = data
  return `
<p><b>${title}</b></p>
<p>${description}</p>
<ul class="tooltip-list">
    <li><i class="fa fa-fw fa-square tooltip-up"></i><b>Up</b>: ${reasonUp}</li>
    <li><i class="fa fa-fw fa-square tooltip-down"></i><b>Down</b>: ${reasonDown}</li>
    <li><i class="fa fa-fw fa-square tooltip-unknown"></i><b>Unknown</b>: ${reasonUnknown}</li>
    <li><i class="fa fa-fw fa-square tooltip-nodata"></i><b>Nodata</b>: ${reasonNodata}</li>
</ul>
`
}

const REASON_AGENTS_STOPPED = "agents were stopped"
const REASON_APISERVER_UNAVAILABLE = "kube-apiserver is not available"
const SYNTHETIC_REASONS = {
  reasonUp: "resolve is successful and all IPs are responded with 200 OK",
  reasonDown: "some IP responded with other code",
  reasonUnknown: "error during resolve or application is responded slow",
  reasonNodata: REASON_AGENTS_STOPPED,
}
const GROUP_DEFAULT_TOOLTIP = {
  reasonUp: "all probes are up",
  reasonDown: "at least one probe is down",
  reasonUnknown: "uncertain result, see probe results for clues",
  reasonNodata: REASON_AGENTS_STOPPED,
}

interface LangPack {
  group: { [name: string]: tooltipData }
  probe: { [name: string]: { [name: string]: tooltipData } }
  mute: Mute
}

interface MuteItem {
  id: string
  label: string
  tooltip: string
}

interface MuteMenu {
  label: string
  tooltip: string
}

interface Mute {
  items: {
    Acd: MuteItem
    Mnt: MuteItem
    InfAcd: MuteItem
    InfMnt: MuteItem
  }
  order: string[]
  menu: MuteMenu
}

const langPack: LangPack = {
  group: {
    "control-plane": {
      ...GROUP_DEFAULT_TOOLTIP,
      title: "Kubernetes Control Plane",
      description: `
            <p>Control Plane is available. Self-healing is working.</p>
            <p>Group result is a combination of probe results with the priority of the worst results.</p>
            `,
    },
    synthetic: {
      ...GROUP_DEFAULT_TOOLTIP,
      title: "Synthetic",
      description: `
            <p>Checks the availability of sample application running in the cluster.</p>
            <p>Group result is a combination of probe results with the priority of the worst results.</p>
            `,
    },
    "monitoring-and-autoscaling": {
      ...GROUP_DEFAULT_TOOLTIP,
      title: "Monitoring and Autoscaling",
      description: `
            <p>Checks the availability of monitoring and autoscaling applications in the cluster.</p>
            <p>Group result is a combination of probe results with the priority of the worst results.</p>
            `,
    },
    extensions: {
      ...GROUP_DEFAULT_TOOLTIP,
      title: "Extensions",
      description: `
            <p>Checks the availability of various extensions apps.</p>
            <p>Group result is a combination of probe results with the priority of the worst results.</p>
            `,
    },
    "load-balancing": {
      ...GROUP_DEFAULT_TOOLTIP,
      title: "Load balancing",
      description: `
            <p>Checks the availability of load balancer configuration controller.</p>`,
    },
    deckhouse: {
      ...GROUP_DEFAULT_TOOLTIP,
      title: "Deckhouse",
      description: `
            <p>Checks the availability of deckhouse and working hook.</p>
            <p>Group result is a combination of probe results with the priority of the worst results.</p>
            `,
    },
    nginx: {
      ...GROUP_DEFAULT_TOOLTIP,
      title: "Nginx",
      description: `
            <p>Checks the availability of Ingress Nginx Controller.</p>
            <p>Group result is a combination of probe results with the priority of the worst results.</p>
            `,
    },
    nodegroups: {
      ...GROUP_DEFAULT_TOOLTIP,
      title: "Node Groups",
      description: `
            <p>Checks the availability of nodes as specified in NodeGroups.</p>
            <p>Group result is a combination of probe results with the priority of the worst results.</p>
            `,
    },
  },
  probe: {
    "control-plane": {
      apiserver: {
        title: "API server",
        description: "The probe requests <code>/version</code> endpoint from kube-apiserver",
        reasonUp: "<code>/version</code> returns 200 OK.",
        reasonDown: "<code>/version</code> returns other codes or there is no connection to kube-apiserver.",
        reasonUnknown: "connection to kube-apiserver is slow.",
        reasonNodata: REASON_AGENTS_STOPPED,
      },
      "basic-functionality": {
        title: "Basic Functionality",
        description: "Every 5s a ConfigMap is created and deleted.",
        reasonUp: "configMap successfully created and deleted",
        reasonDown: "error occurred during the creation or deletion of the ConfigMap",
        reasonUnknown:
          "kube-apiserver is not available or probe execution is skipped because previous probe was not yet finished",
        reasonNodata: REASON_AGENTS_STOPPED,
      },
      "cert-manager": {
        title: "Cert Manager",
        description: "Every 1 minute a certificate is created and deleted to check the cert-manager handling of it",
        reasonUp:
          "certificate secret was created, and after the certificate was deleted, the secret disappeared",
        reasonDown: "certificate secret was not created in 5 seconds, or was no deleted in 20 seconds",
        reasonUnknown:
          "error occurred during creation or deletion, or kube-apiserver is not available",
        reasonNodata: REASON_AGENTS_STOPPED,
      },
      "controller-manager": {
        title: "Controller Manager",
        description: "Every 1 minute a Deployment is created and deleted",
        reasonUp:
          "deployment successfully created, and its Pod became <code>Pending</code>. Afterwards, the deployment successfully deleted",
        reasonDown: "created Pod has not become <code>Pending</code> after 10 seconds",
        reasonUnknown:
          "error occurred during creation or deletion, or kube-apiserver is not available, or probe execution is skipped because previous probe was not yet finished",
        reasonNodata: REASON_AGENTS_STOPPED,
      },
      namespace: {
        title: "Namespace",
        description: "Every 1 minute a Namespace is created and deleted",
        reasonUp: "namespace successfully created and deleted",
        reasonDown: "created Namespace is not deleted after 1 minute",
        reasonUnknown:
          "error during create or delete operations, kube-apiserver is not available or operations are slow",
        reasonNodata: REASON_AGENTS_STOPPED,
      },
      scheduler: {
        title: "Scheduler",
        description: "Every 1 minute a Pod is created, scheduled to a matching node and deleted",
        reasonUp: "pod successfully created and scheduled to a node",
        reasonDown: "created Pod is not scheduled within 20 seconds",
        reasonUnknown:
          "error during create or delete operations, kube-apiserver is not available or probe execution is skipped because previous probe was not yet finished",
        reasonNodata: REASON_AGENTS_STOPPED,
      },
    },
    "monitoring-and-autoscaling": {
      prometheus: {
        title: "Prometheus",
        description: "Prometheus is a monitoring system.",
        reasonUp: "at least one Pod is in <code>Ready</code> state, and the Prometheus API responds correctly",
        reasonDown: "either no ready Pods present, or Prometheus responds with invalid data",
        reasonUnknown: REASON_APISERVER_UNAVAILABLE,
        reasonNodata: REASON_AGENTS_STOPPED,
      },
      trickster: {
        title: "Trickster",
        description: "Trickster is an HTTP reverse proxy/cache for Prometheus",
        reasonUp: "at least one Pod is in <code>Ready</code> state, and the Trickster API responds correctly",
        reasonDown: "either no ready Pods present, or Trickster responds with invalid data",
        reasonUnknown: REASON_APISERVER_UNAVAILABLE,
        reasonNodata: REASON_AGENTS_STOPPED,
      },
      "prometheus-metrics-adapter": {
        title: "Prometheus Metrics Adapter",
        description: "Prometheus Metrics Adapter provides metrics for the cluster (nodes, containers, etc.)",
        reasonUp: "at least one Pod is in <code>Ready</code> state and the custom metrics CR contains non-zero value",
        reasonDown: "either no ready Pods present, or adapter metrics response is invalid",
        reasonUnknown: REASON_APISERVER_UNAVAILABLE,
        reasonNodata: REASON_AGENTS_STOPPED,
      },
      "key-metrics-present": {
        title: "Key Metrics Presence",
        description: "Key cluster metrics are metrics from the Kubernetes, nodes and containers.",
        reasonUp: "metrics from all key components are present in Prometheus and are non-zero",
        reasonDown: "at least one of metrics is absent or has zero value",
        reasonUnknown: "<i>(unexpected)</i>",
        reasonNodata: REASON_AGENTS_STOPPED,
      },
      "metrics-sources": {
        title: "Metrics Sources",
        description: "Key metric sources are from nodes and Kubernetes.",
        reasonUp:
          "all pods of node-exporter, and at least one Pod of kube-state-metrics is in <code>Ready</code> state",
        reasonDown:
          "at least one Pod of node-exporter, or all pods of kube-state-metrics are not in <code>Ready</code> state",
        reasonUnknown: REASON_APISERVER_UNAVAILABLE,
        reasonNodata: REASON_AGENTS_STOPPED,
      },
      "vertical-pod-autoscaler": {
        title: "Vertical Pod Autoscaler",
        description: "The Vertical Pod Autoscaler automatically changes resources acquired by pods (CPU, memory)",
        reasonUp: "at least one Pod of a VPA component is in <code>Ready</code> state",
        reasonDown: "a Pod of at least one VPA component is not in <code>Ready</code> state",
        reasonUnknown: REASON_APISERVER_UNAVAILABLE,
        reasonNodata: REASON_AGENTS_STOPPED,
      },
      "horizontal-pod-autoscaler": {
        title: "Horizontal Pod Autoscaler",
        description: "The Horizontal Pod Autoscaler automatically changes the number of replicas",
        reasonUp: "both probes of Prometheus Metrics Adapter and Controller Manager are successful",
        reasonDown: "at least one of probes of Prometheus Metrics Adapter or Controller Manager failed",
        reasonUnknown:
          "at least one of probes of Prometheus Metrics Adapter or Controller Manager has unknown result but none of them failed",
        reasonNodata: REASON_AGENTS_STOPPED,
      },
    },
    synthetic: {
      access: {
        ...SYNTHETIC_REASONS,
        title: "Access",
        description:
          "Every 5 seconds resolve sample application IPs and request <code>/</code> endpoint until first success",
      },
      dns: {
        ...SYNTHETIC_REASONS,
        title: "DNS",
        description:
          "Every 5 seconds resolve sample application IPs and request <code>/dns</code> endpoint until first success",
      },
      neighbor: {
        ...SYNTHETIC_REASONS,
        title: "Neighbor",
        description:
          "Every 5 seconds resolve sample application IPs and request <code>/neighbor</code> endpoint until first success",
      },
      "neighbor-via-service": {
        ...SYNTHETIC_REASONS,
        title: "Neighbor via Service",
        description:
          "Every 5 seconds resolve sample application IPs and request <code>/neighbor-via-service</code> endpoint until first success",
      },
    },
    extensions: {
      "cluster-scaling": {
        title: "Cluster Scaling",
        description:
          "Cluster scaling is provided by Machine Controller Manager (MCM), Cloud Contoller Manager (CCM), and bashible apiserver",
        reasonUp: "at least one Pod of each of MCM, CCM, and bashible apiserver is in <code>Ready</code> state",
        reasonDown: "there are no pods of any of MCM, CCM, or bashible apiserver in <code>Ready</code> state",
        reasonUnknown:
          "error occurred during pods fetching, or kube-apiserver is not available, or probe execution is skipped because previous probe was not yet finished",
        reasonNodata: REASON_AGENTS_STOPPED,
      },
      "cluster-autoscaler": {
        title: "Cluster Autoscaler",
        description: "Cluster Autoscaler automatically adds and removes cluster nodes",
        reasonUp: "at least one Pod of cluster-autoscaler is in <code>Ready</code> state",
        reasonDown: "there are no pods of cluster-autoscaler in <code>Ready</code> state",
        reasonUnknown:
          "error occurred during pods fetching, or kube-apiserver is not available, or probe execution is skipped because previous probe was not yet finished",
        reasonNodata: REASON_AGENTS_STOPPED,
      },
      grafana: {
        title: "Grafana",
        description: "Grafana shows metrics dashboards",
        reasonUp: "at least one Pod of Grafana is in <code>Ready</code> state",
        reasonDown: "there are no pods of Grafana in <code>Ready</code> state",
        reasonUnknown:
          "error occurred during pods fetching, or kube-apiserver is not available, or probe execution is skipped because previous probe was not yet finished",
        reasonNodata: REASON_AGENTS_STOPPED,
      },
      openvpn: {
        title: "OpenVPN",
        description: "OpenVPN provides access to cluster networks of pods and services",
        reasonUp: "at least one Pod of OpenVPN is in <code>Ready</code> state",
        reasonDown: "there are no pods of OpenVPN in <code>Ready</code> state",
        reasonUnknown:
          "error occurred during pods fetching, or kube-apiserver is not available, or probe execution is skipped because previous probe was not yet finished",
        reasonNodata: REASON_AGENTS_STOPPED,
      },
      "prometheus-longterm": {
        title: "Longterm Prometheus",
        description: "Prometheus-longterm stores sparse longterm metrics for retrospective observations.",
        reasonUp: "at least one Pod is in <code>Ready</code> state, and the Prometheus API responds correctly",
        reasonDown: "either no ready Pods present, or Prometheus responds with invalid data",
        reasonUnknown:
          "error occurred during pods fetching, or kube-apiserver is not available, or probe execution is skipped because previous probe was not yet finished",
        reasonNodata: REASON_AGENTS_STOPPED,
      },
      dashboard: {
        title: "Kubernetes Dashboard",
        description: "General-purpose web UI for the cluster",
        reasonUp: "at least one Pod of Dashboard is in <code>Ready</code> state",
        reasonDown: "there are no pods of Dashboard in <code>Ready</code> state",
        reasonUnknown:
          "error occurred during pods fetching, or kube-apiserver is not available, or probe execution is skipped because previous probe was not yet finished",
        reasonNodata: REASON_AGENTS_STOPPED,
      },
      dex: {
        title: "Dex",
        description: "OpenID Connect provider",
        reasonUp: "at least one Pod of Dex is in <code>Ready</code> state and its API responds correctly",
        reasonDown: "either no ready Pods present, or Dex responds with invalid data",
        reasonUnknown:
          "error occurred during pods fetching, or kube-apiserver is not available, or probe execution is skipped because previous probe was not yet finished",
        reasonNodata: REASON_AGENTS_STOPPED,
      },
    },
    "load-balancing": {
      "load-balancer-configuration": {
        title: "Load Balancer Configuration",
        description: "Load balancer configuration is provided by Cloud Controller Manager (CCM)",
        reasonUp: "at least one Pod of CCM is in <code>Ready</code> state",
        reasonDown: "there are no pods of CCM in <code>Ready</code> state",
        reasonUnknown:
          "error occurred during pods fetching, or kube-apiserver is not available, or probe execution is skipped because previous probe was not yet finished",
        reasonNodata: REASON_AGENTS_STOPPED,
      },
      metallb: {
        title: "MetalLB",
        description: "Metal Load Balancer provides traffic load balancing feature independently from a cloud provider",
        reasonUp: "at least one Pod of each of controller and speaker is in <code>Ready</code> state",
        reasonDown: "there are no pods of any of controller or speaker in <code>Ready</code> state",
        reasonUnknown:
          "error occurred during pods fetching, or kube-apiserver is not available, or probe execution is skipped because previous probe was not yet finished",
        reasonNodata: REASON_AGENTS_STOPPED,
      },
    },
    deckhouse: {
      "cluster-configuration": {
        title: "Cluster Configuration",
        description: "Cluster configurations is contolled by Deckhouse",
        reasonUp:
          "Deckhouse pod is <code>Ready</code> state in 20 minutes, and test hook reacts on custom resourse changes",
        reasonDown:
          "Deckhouse pod is not in <code>Ready</code> state for more than 20 minutes, or hook does not react to custom resource",
        reasonUnknown:
          "error occurred during pod fetching, or pod is terminating, or kube-apiserver is not available, or probe execution is skipped because previous probe was not yet finished",
        reasonNodata: REASON_AGENTS_STOPPED,
      },
    },
  },
  mute: {
    items: {
      Acd: {
        id: "Acd",
        label: "Accident",
        tooltip:
          "Mute <code>Down</code> seconds caused by <code>Accident</code>: a downtime caused by unforeseen cluster crash.",
      },
      Mnt: {
        id: "Mnt",
        label: "Maintenance",
        tooltip:
          "Mute <code>Down</code> seconds caused by <code>Maintenance</code>: a downtime caused by scheduled cluster maintenance jobs.",
      },
      InfAcd: {
        id: "InfAcd",
        label: "Infrastructure Accident",
        tooltip:
          "Mute <code>Down</code> seconds caused by <code>Infrastructure Accident</code>: a downtime caused by infrastructure crashes and reported by provider.",
      },
      InfMnt: {
        id: "InfMnt",
        label: "Infrastructure Maintenance",
        tooltip:
          "Mute <code>Down</code> seconds caused by <code>Infrastructure Maintenance</code>: a downtime caused by scheduled maintenance jobs done by an infrastructure provider.",
      },
    },
    order: ["Acd", "Mnt", "InfAcd", "InfMnt"],
    menu: {
      label: "Mute",
      tooltip: "Choose types of Downtime objects in cluster that will be used to mute Down seconds.",
    },
  },
}

// Tooltips replicate the same structure as langPack but with rendered HTML for groups and probes
function renderTooltips() {
  const groupTooltips: { [group: string]: string } = {}

  for (const [name, groupSpec] of Object.entries(langPack.group)) {
    groupTooltips[name] = tooltip(groupSpec)
  }

  const probeTooltips: { [group: string]: { [probe: string]: string } } = {}

  for (const [groupName, probesByName] of Object.entries(langPack.probe)) {
    const groupedTooltips: { [probe: string]: string } = {}

    for (const [probeName, probeSpec] of Object.entries(probesByName)) {
      groupedTooltips[probeName] = tooltip(probeSpec)
    }

    probeTooltips[groupName] = groupedTooltips
  }

  return {
    group: groupTooltips,
    probe: probeTooltips,
    mute: langPack.mute,
  }
}

function fallbackTooltip(input: Partial<tooltipData>): tooltipData {
  const fallbackTooltipData = {
    title: "...",
    description: "...",
    reasonUp: "...",
    reasonDown: "...",
    reasonUnknown: "...",
    reasonNodata: "...",
  }

  return {
    ...fallbackTooltipData,
    ...input,
  }
}

export function getGroupSpec(group: string): tooltipData {
  const spec = langPack.group[group]
  if (spec) {
    return spec
  }

  return fallbackTooltip({title: `Group ${group}`})
}

export function getProbeSpec(group: string, probe: string): tooltipData {
  const probeSpecs = langPack.probe[group]
  if (!probeSpecs) {
    // for dynamic probes, we cannot control probe names in advance, so just show what we got.
    return fallbackTooltip({title: `${probe}`})
  }

  const spec = probeSpecs[probe]
  if (!spec) {
    // we have the group, so the title of probe is the probe key
    return fallbackTooltip({title: probe})
  }

  return spec
}

const tooltips = renderTooltips()
export default tooltips
