interface IGroupData {
	name: string;
	description: string;
}

const defaultGroupData: IGroupData = {
	name: "TODO",
	description: "TODO",
};

const known: { [name: string]: IGroupData } = {
	"control-plane": {
		name: "Control plane",
		description: "The availability of Kubernetes control-plane",
	},
	synthetic: {
		name: "Synthetic",
		description: "The availability of sample application, and network connectivity",
	},
	"monitoring-and-autoscaling": {
		name: "Monitoring and Autoscaling",
		description: "The availability of monitoring and autoscaling applications in the cluster",
	},
	extensions: {
		name: "Extensions",
		description: "The availability of extensions apps",
	},
	"load-balancing": {
		name: "Load Balancing",
		description: "The availability of traffic load balancing and its configuration controllers",
	},
	deckhouse: {
		name: "Deckhouse",
		description: "The availability of deckhouse and working hook",
	},
	nodegroups: {
		name: "Node Groups",
		description: "The availability of CloudEphemeral nodes",
	},
	nginx: {
		name: "Nginx",
		description: "The availability of Nginx Ingress Controllers",
	},
};

export function getGroupData(name: string): IGroupData {
	const data = known[name];

	if (!data) {
		return {
			...defaultGroupData,
			name,
		};
	}

	return data;
}
