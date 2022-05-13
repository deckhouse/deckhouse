interface IGroupData {
	name: string
	description: string
}

const defaultGroupData: IGroupData = {
	name: "TODO",
	description: "TODO",
};

const known: { [name: string]: IGroupData } = {
	"control-plane": {
		name: "Control plane",
		description: "Cluster control-plane is available. Self-healing is working.",
	},
	"synthetic": {
		name: "Synthetic",
		description: "Availability of sample application running in cluster.",
	},
	"monitoring-and-autoscaling": {
		name: "Monitoring and Autoscaling",
		description: "Availability of monitoring and autoscaling applications in the cluster.",
	},
	"extensions": {
		name: "Extensions",
		description: "Availability of extensions apps.",
	},
	"load-balancing": {
		name: "Load Balancing",
		description: "Availability of traffic load balancing and its configuration controllers.",
	},
	"deckhouse": {
		name: "Deckhouse",
		description: "The availability of deckhouse and working hook.",
	}
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
