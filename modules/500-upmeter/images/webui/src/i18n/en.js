export const langPack = {
  group: {
    "control-plane": "Cluster is available for deploy. Self-healing is working.",
    "synthetic": "Availability of applications running in cluster",
  },
  probe: {
    "control-plane": {
      "access": "API-server is available",
      "basic-functionality": "ConfigMap can be created and deleted",
      "control-plane-manager": "Deployment can be created and deleted",
      "namespace": "Namespace can be created and deleted",
      "scheduler": "Pod can be created and scheduled on matching Node"
    },
    "synthetic": {
      "access": "Application can response with 200 OK",
      "dns": "Application is discoverable via Service and response via resolved IPs",
      "neighbor": "Application's Pods can communicate",
      "neighbor-via-service": "Application's Pods can communicate via Service with ClusterIP"
    }
  }
}
