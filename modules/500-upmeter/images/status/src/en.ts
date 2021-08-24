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
	"scaling": {
		name: "Cluster scaling",
		description: "Availability of cluster scaling controllers and controller managers.",
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
