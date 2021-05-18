const groups = {
  "control-plane": {
    label: "Control plane",
    description: "Cluster control-plane is available. Self-healing is working.",
  },
  synthetic: {
    label: "Synthetic",
    description: "Availability of sample application running in cluster.",
  },
  "monitoring-and-autoscaling": {
    label: "Monitoring and Autoscaling",
    description: "Availability of monitoring and autoscaling applications in the cluster.",
  },
}

const defaultGroup = {
  label: "Unknown Probe Group",
  description: "Waiting to be filled in.",
}

export function group(name) {
  const data = groups[name]

  if (!data) {
    return {
      ...defaultGroup,
      label: `[${name}]`,
    }
  }

  return data
}
