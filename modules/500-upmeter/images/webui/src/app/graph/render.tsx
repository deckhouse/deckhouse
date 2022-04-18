import React from "react";
import ReactDOM from "react-dom";
import * as d3 from "../libs/d3";

// Components
import { Tooltip } from "@grafana/ui";
import { Icon } from "../components/Icon";
import { GroupProbeTooltip } from "../components/GroupProbeTooltip";
import { PieBoundingRect } from "../components/PieBoundingRect";

// Services
import { getGroupSpec, getProbeSpec } from "../i18n/en";
import { calculateTopTicks } from "./topticks";
import { getEventsSrv } from "../services/EventsSrv";
import { getTimeRangeSrv } from "../services/TimeRangeSrv";
import { availabilityPercent } from "../util/humanSeconds";
import { Episode, LegacySettings, StatusRange } from "app/services/DatasetSrv";
import { Dataset } from "app/services/Dataset";

// Groups layout from https://bl.ocks.org/Andrew-Reid/960819e98873bbaf035bbf6bd2774b40
// Pie from https://observablehq.com/@d3/donut-chart
// Tooltip https://bl.ocks.org/d3noob/a22c42db65eb00d4e369
// Text alignment https://bl.ocks.org/emmasaunders/0016ee0a2cab25a643ee9bd4855d3464
// Format numbers https://github.com/d3/d3-format

// Settings
const pieBoxWidth = 60;
// const pieSpace = 15;
// const pieInnerRadius = 0.67;
// const legendWidth = 250;
// const topTicksHeight = 30;
// const leftPadding = 30;
// const rightPadding = 30;

// let width = leftPadding + legendWidth + (12 * (pieBoxWidth + pieSpace) + pieSpace) + rightPadding;
// let height = topTicksHeight + 5 * (pieBoxWidth + pieSpace) + pieSpace;

//let root = d3.select("#graph")
//  .attr("width", width)
//  .attr("height", height)
//  .attr("viewBox", [0, 0, width, height]);

export function renderGraphTable(dataset: Dataset, settings: LegacySettings) {
	const root = d3.select("#graph");

	// Always recreate everything
	root.selectAll("*").remove();

	root
		.append("div")
		.attr("class", "top-ticks")
		.append("div")
		.attr("class", "top-tick-left")
		.append("div")
		.attr("class", "top-tick-left-spacer");

	updateTicks(dataset, settings);

	const graphTable = root.append("div").attr("class", "graph-table");

	const groups = graphTable.selectAll(".graph-row").data(settings.groupProbes);

	// each group is a table row, so translate "g" element to proper y position
	const groupsEnter = groups
		.enter()
		.append("div")
		.attr("data-group-id", (d) => d.group)
		.attr("data-probe-id", (d) => d.probe)
		.attr("class", "graph-row");

	// Labels for group and probes.
	groupsEnter.each(function (probeData) {
		// `group` and `probe` are names
		// type is one of "group", "probe", "first-in-group", or "last-in-group"
		const { group, probe, type: typ } = probeData;

		// { expanded: boolean, probesLoaded: boolean }
		const groupState = settings.groupState[group];

		const rowEl = d3.select(this);
		const cellEl = rowEl.append("div").attr("class", "graph-cell graph-labels");

		if (probe === "__total__") {
			// total is always visible
			rowEl.classed("graph-row-visible", true);

			// add open/close icon
			// fas - fontawesome icon
			// fa-fw - fixed width https://fontawesome.com/how-to-use/on-the-web/styling/fixed-width-icons
			// fa-** - icon name
			// caret-right - closed icon for group
			// caret-down - opened icon for group
			cellEl.append("i").attr("class", "fas fa-fw fa-caret-right group-icon");

			// Add label
			const { title: groupTitle } = getGroupSpec(group);
			cellEl.append("span").text(groupTitle).attr("class", "group-label");

			cellEl.on("click", function (d) {
				// TODO add visibility indicator to request probes data without
				// additional clicks when change intervals.

				let probeRows = d3.selectAll(`#graph div[data-group-id=${group}].graph-row`);

				// invert expanded
				groupState.expanded = !groupState.expanded;
				getTimeRangeSrv().onExpandGroup(group, groupState.expanded);

				probeRows.each(function () {
					let rowEl = d3.select(this);
					let probeId = rowEl.attr("data-probe-id");
					if (probeId === "__total__") {
						return;
					}
					rowEl.classed("graph-row-hidden", !groupState.expanded);
					rowEl.classed("graph-row-visible", groupState.expanded);
				});

				// toggle icon
				let iconEl = cellEl.select(".group-icon svg[data-fa-i2svg]");
				iconEl.classed("fa-caret-right", !groupState.expanded);
				iconEl.classed("fa-caret-down", groupState.expanded);

				// trigger event to re-render graph
				if (!groupState.probesLoaded) {
					getEventsSrv().fireEvent("UpdateGroupProbes", { group, settings });
				}
			});
		} else {
			rowEl.classed("graph-row-hidden", true);
			rowEl.classed(`row-${typ}`, true);
			rowEl.classed(`row-probe`, true);

			// Add label
			const { title: probeTitle } = getProbeSpec(group, probe);
			cellEl.append("span").text(probeTitle).attr("class", "probe-label");
		}

		let infoEl = cellEl.append("div").attr("class", "group-probe-info");

		ReactDOM.render(
			<Tooltip content={<GroupProbeTooltip groupName={group} probeName={probe} />} placement="right-start">
				<Icon name="fa-info-circle" className="group-probe-info" />
			</Tooltip>,
			infoEl.node(),
		);
	});

	// Each row has empty cell to define initial height for empty rows
	groupsEnter
		.append("div")
		//.text("Data for group '" + group + "'")
		.attr("class", "graph-cell cell-data")
		.append("svg")
		.attr("width", pieBoxWidth)
		.attr("height", pieBoxWidth);
}

export function updateTicks(dataset: Dataset, settings: LegacySettings) {
	let root = d3.select("#graph div.top-ticks");
	// Always recreate top ticks
	root.selectAll("div.top-tick").remove();

	const topTicks = calculateTopTicks(dataset, settings);

	topTicks.forEach((tick) =>
		root //
			.append("div")
			.attr("data-timeslot", tick.ts)
			.attr("class", "top-tick")
			.append("span")
			.text(tick.text),
	);

	// 'Total' label
	root.append("div").attr("class", "top-tick total-tick").append("span").text("Total");
}

export function renderGroupData(_: Dataset, settings: any, group: string, data: StatusRange) {
	const rowEl = d3.select(`#graph div[data-group-id=${group}][data-probe-id="__total__"]`);
	rowEl.selectAll(".cell-data").remove();
	const groupEpisodes = data.statuses && data.statuses[group]["__total__"];
	if (!groupEpisodes) {
		console.log("Bad group data", data);
	}

	for (const episode of groupEpisodes) {
		const cell = rowEl
			.append("div")
			//.text("Data for group '" + group + "'")
			.attr("class", "graph-cell cell-data");

		const viewBox = [0, 0, pieBoxWidth, pieBoxWidth].join(" ");

		const svg = cell //
			.append("svg")
			.attr("width", pieBoxWidth)
			.attr("height", pieBoxWidth)
			.attr("viewBox", viewBox);

		drawOnePie(svg, settings, episode, "group");
	}

	// add empty boxes into probe rows to prevent stripe background on expand
	const rows = d3.selectAll(`#graph div[data-group-id=${group}].graph-row`);
	rows.each(function (item: any) {
		if (item.probe === "__total__") {
			return;
		}

		let rowEl = d3.select(this);
		rowEl.selectAll(".cell-data").remove();
		for (let i = 0; i < groupEpisodes.length; i++) {
			rowEl
				.append("div")
				.attr("class", "graph-cell cell-data")
				.append("svg")
				.attr("width", pieBoxWidth)
				.attr("height", pieBoxWidth);
		}
	});
}

export function renderGroupProbesData(settings: LegacySettings, group: string, data: StatusRange) {
	const root = d3.select("#graph");

	const statuses = data.statuses;
	for (const group in statuses) {
		if (!statuses.hasOwnProperty(group)) {
			continue;
		}

		const unseenGroupProbes = new Set(
			settings.groupProbes
				.filter(({ group: g }) => g === group)
				.filter(({ probe }) => probe !== "__total__")
				.map(({ probe }) => probe),
		);

		const probes = statuses[group];
		const getRowElement = (group: string, probe: string) => root.select(`div[data-group-id=${group}][data-probe-id=${probe}]`);
		for (const probe in probes) {
			if (!probes.hasOwnProperty(probe)) {
				continue;
			}

			// Track absent probes to hide empty rows
			unseenGroupProbes.delete(probe)

			// Render pies
			const episodes = probes[probe]
			const cellCount = probes[probe].length;
			const rowEl = getRowElement(group, probe)
			rowEl.selectAll(".cell-data").remove();
			episodes.forEach(function (episode, i) {
				const cell = rowEl.append("div").attr("class", "graph-cell cell-data");

				if (i === 0) {
					cell.classed("first-in-row", true);
				}
				if (i === cellCount - 1) {
					cell.classed("last-in-row", true);
				}

				const viewBox = [0, 0, pieBoxWidth, pieBoxWidth].join(" ");

				const svg = cell //
					.append("svg")
					.attr("width", pieBoxWidth)
					.attr("height", pieBoxWidth)
					.attr("viewBox", viewBox);

				drawOnePie(svg, settings, episode, "probe");
			});
		}

		for (const probe of unseenGroupProbes) {
			getRowElement(group, probe).remove();
		}

	}
}

// pie for each "g"
const pie = d3
	.pie()
	.padAngle(0)
	.sort(null)
	.value((x) => x.valueOf());

const arcs = {
	group: (function () {
		const radius = pieBoxWidth / 2;
		return d3
			.arc()
			.innerRadius(0)
			.outerRadius(radius - 1);
	})(),
	probe: (function () {
		const radius = (pieBoxWidth * 0.8) / 2;
		return d3
			.arc()
			.innerRadius(0)
			.outerRadius(radius - 1);
	})(),
};

function toPieData(d: Episode): Array<{ name: string; valueOf(): number }> {
	const fields = ["up", "down", "muted", "unknown", "nodata"];
	const listedTimers: Array<{ name: string; valueOf(): number }> = [];

	for (const [field, value] of Object.entries(d)) {
		if (!fields.includes(field)) {
			continue;
		}
		listedTimers.push({
			name: field,
			valueOf: () => +value,
		});
	}

	return listedTimers;
}

function drawOnePie(root: any, settings: LegacySettings, episode: Episode, pieType: "group" | "probe") {
	const halfWidth = pieBoxWidth / 2;

	const pieRoot = root
		.append("g")
		.attr("class", "statusPie")
		.attr("height", pieBoxWidth)
		.attr("width", pieBoxWidth)
		.attr("transform", () => `translate(${halfWidth},${halfWidth})`);

	pieRoot
		.selectAll("path")
		.data(pie(toPieData(episode)))
		.join("path")
		.attr("class", (d: { data: { name: string } }) => "pie-seg-" + d.data.name)
		.attr("d", arcs[pieType])
		.append("title")
		.text((d: { data: { name: string; valueOf(): number } }) => `${d.data.name}: ${d.data.valueOf().toLocaleString()}`);

	// Add text with availability percents
	pieRoot
		.append("text")
		.text(availabilityPercent(+episode.up, +episode.down, +episode.muted, 2))
		.attr("class", `pie-text-${pieType}`);

	// Add a transparent rectangle to use
	// as a bounding box for click events and for tooltip hover events.
	const boundingRectRoot = pieRoot.append("g");

	const onClick = () => getTimeRangeSrv().drillDownStep(+episode.ts);

	ReactDOM.render(<PieBoundingRect size={pieBoxWidth} episode={episode} onClick={onClick} />, boundingRectRoot.node());
}
