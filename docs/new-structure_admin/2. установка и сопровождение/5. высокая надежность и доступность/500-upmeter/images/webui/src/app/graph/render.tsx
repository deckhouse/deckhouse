import React from "react"
import ReactDOM from "react-dom"
import * as d3 from "d3"

// Components
import { Tooltip } from "@grafana/ui"
import { Icon } from "../components/Icon"
import { GroupProbeTooltip } from "../components/GroupProbeTooltip"
import { PieBoundingRect } from "../components/PieBoundingRect"

// Services
import { getGroupSpec, getProbeSpec } from "../i18n/en"
import { calculateTopTicks } from "./topticks"
import { getEventsSrv } from "../services/EventsSrv"
import { getTimeRangeSrv } from "../services/TimeRangeSrv"
import { availabilityPercent } from "../util/humanSeconds"
import { Episode, LegacySettings, StatusRange } from "app/services/DatasetSrv"
import { Dataset } from "app/services/Dataset"

// Groups layout from https://bl.ocks.org/Andrew-Reid/960819e98873bbaf035bbf6bd2774b40
// Pie from https://observablehq.com/@d3/donut-chart
// Tooltip https://bl.ocks.org/d3noob/a22c42db65eb00d4e369
// Text alignment https://bl.ocks.org/emmasaunders/0016ee0a2cab25a643ee9bd4855d3464
// Format numbers https://github.com/d3/d3-format

// Settings
const pieBoxWidth = 60
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
  // Group and probe data
  const probesByGroup = (g: string) =>
    settings.groupProbes.filter(({ group, probe }) => group === g && probe !== "__total__")

  const groupedData = settings.groupProbes
    .filter(({ probe }) => probe === "__total__")
    .map((x) => ({ ...x, probes: probesByGroup(x.group) }))

  const root = d3.select("#graph")

  // Always recreate everything
  root.selectAll("*").remove()

  root
    .append("div")
    .attr("class", "top-ticks")
    .append("div")
    .attr("class", "top-tick-left")
    .append("div")
    .attr("class", "top-tick-left-spacer")

  updateTicks(dataset, settings)

  // The container for group charts row and probes container
  // table #graph
  // 	group container   div[data-group-id=${group}].group-container
  // 		group charts row   div[data-group-id=${group}][data-probe-id=__total__]
  // 		probes container   div[data-group-id=${group}].probes-container
  //			probes charts row   div[data-group-id=${group}][data-probe-id=${probe}]

  const graphTable = root.append("div").attr("class", "graph-table")

  // group container
  const groupContainers = graphTable
    .selectAll("div.group-container")
    .data(groupedData)
    .enter()
    .append("div")
    .attr("class", "group-container")
    .attr("data-group-id", (gp) => gp.group)

  // group charts rows
  const groupChartsRows = groupContainers
    .append("div")
    .attr("class", "graph-row")
    .attr("data-group-id", (gp) => gp.group)
    .attr("data-probe-id", "__total__")

  // probes containers
  /* const probeContainers = */ groupContainers
    .append("div")
    .attr("class", "probes-container")
    .attr("data-group-id", (gp) => gp.group)
    .style("display", "none")

  const selectProbeContainer = (group: string) => d3.select(`#graph div[data-group-id=${group}].probes-container`)

  // Group rows charts
  groupChartsRows.each(function ({ group }) {
    // Group row is always visible
    const groupRow = d3.select(this).classed("graph-row-visible", true)
    const groupLabel = groupRow.append("div").attr("class", "graph-cell graph-labels")

    // Expanding triangle arrow icon
    // 	fas - fontawesome icon
    // 	fa-fw - fixed width https://fontawesome.com/how-to-use/on-the-web/styling/fixed-width-icons
    // 	fa-** - icon name
    // 	caret-right - closed icon for group
    // 	caret-down - opened icon for group
    groupLabel.append("i").attr("class", "fas fa-fw fa-caret-right group-icon")

    // Add label
    const { title: groupTitle } = getGroupSpec(group)
    groupLabel.append("span").text(groupTitle).attr("class", "group-label")

    // Switch show/hide probe stats
    // { expanded: boolean, probesLoaded: boolean }
    const groupState = settings.groupState[group]
    groupLabel.on("click", () => {
      // TODO add visibility indicator to request probes data without
      // additional clicks when change intervals.

      // invert the expanded state
      groupState.expanded = !groupState.expanded
      getTimeRangeSrv().onExpandGroup(group, groupState.expanded)

      // show/hide probe container
      const display = groupState.expanded ? "block" : "none"
      selectProbeContainer(group).style("display", display)

      // toggle icon
      groupLabel
        .select(".group-icon svg[data-fa-i2svg]")
        .classed("fa-caret-right", !groupState.expanded)
        .classed("fa-caret-down", groupState.expanded)

      // trigger event to re-render graph
      if (!groupState.probesLoaded) {
        getEventsSrv().fireEvent("UpdateGroupProbes", { group, settings })
      }
    })

    // Group tooltip handle
    const infoEl = groupLabel.append("div").attr("class", "group-probe-info")
    ReactDOM.render(
      <Tooltip content={<GroupProbeTooltip groupName={group} probeName="__total__" />} placement="right-start">
        <Icon name="fa-info-circle" className="group-probe-info" />
      </Tooltip>,
      infoEl.node(),
    )

    /* const probeContainerEnter = */ selectProbeContainer(group)
      .selectAll("div")
      .data((d: { group: string }) => probesByGroup(d.group))
      .enter()
      .append("div")
      .attr("data-group-id", (gp) => gp.group)
      .attr("data-probe-id", (gp) => gp.probe)
      .attr("class", "graph-row")
      .classed("row-probe", true)
      .each(function ({ group, probe }) {
        // Probe name and tooltip handle
        const probeChartRow = d3.select(this) //.classed("row-probe", true)
        const probeLabelCell = probeChartRow.append("div").attr("class", "graph-cell graph-labels")

        const { title: probeTitle } = getProbeSpec(group, probe)

        probeLabelCell.append("span").text(probeTitle).attr("class", "probe-label")
        const infoEl = probeLabelCell.append("div").attr("class", "group-probe-info")

        ReactDOM.render(
          <Tooltip content={<GroupProbeTooltip groupName={group} probeName={probe} />} placement="right-start">
            <Icon name="fa-info-circle" className="group-probe-info" />
          </Tooltip>,
          infoEl.node(),
        )
      })
  })

  // Each row has empty cell to define initial height for empty rows
  groupChartsRows
    .append("div")
    //.text("Data for group '" + group + "'")
    .attr("class", "graph-cell cell-data")
    .append("svg")
    .attr("width", pieBoxWidth)
    .attr("height", pieBoxWidth)
}

export function updateTicks(dataset: Dataset, settings: LegacySettings) {
  let root = d3.select("#graph div.top-ticks")
  // Always recreate top ticks
  root.selectAll("div.top-tick").remove()

  const topTicks = calculateTopTicks(dataset, settings)

  topTicks.forEach((tick) =>
    root //
      .append("div")
      .attr("data-timeslot", tick.ts)
      .attr("class", "top-tick")
      .append("span")
      .text(tick.text),
  )

  // 'Total' label
  root.append("div").attr("class", "top-tick total-tick").append("span").text("Total")
}

export function renderGroupData(_: Dataset, settings: any, group: string, data: StatusRange) {
  const rowEl = d3.select(`#graph div[data-group-id=${group}][data-probe-id="__total__"]`)
  rowEl.selectAll(".cell-data").remove()
  const groupEpisodes = data.statuses && data.statuses[group]["__total__"]
  if (!groupEpisodes) {
    console.log("Bad group data", data)
  }

  for (const episode of groupEpisodes) {
    const cell = rowEl
      .append("div")
      //.text("Data for group '" + group + "'")
      .attr("class", "graph-cell cell-data")

    const viewBox = [0, 0, pieBoxWidth, pieBoxWidth].join(" ")

    const svg = cell //
      .append("svg")
      .attr("width", pieBoxWidth)
      .attr("height", pieBoxWidth)
      .attr("viewBox", viewBox)

    drawOnePie(svg, settings, episode, "group")
  }

  // add empty boxes into probe rows to prevent stripe background on expand
  const rows = d3.selectAll(`#graph div[data-group-id=${group}].graph-row`)
  rows.each(function (item: any) {
    if (item.probe === "__total__") {
      return
    }

    let rowEl = d3.select(this)
    rowEl.selectAll(".cell-data").remove()
    for (let i = 0; i < groupEpisodes.length; i++) {
      rowEl
        .append("div")
        .attr("class", "graph-cell cell-data")
        .append("svg")
        .attr("width", pieBoxWidth)
        .attr("height", pieBoxWidth)
    }
  })
}

export function renderGroupProbesData(settings: LegacySettings, group: string, data: StatusRange) {
  if (!data.statuses.hasOwnProperty(group)) {
    console.warn(`no group=${group} in statuses`)
    return
  }
  const probes = data.statuses[group]

  const expectedProbes = new Set<string>(
    settings.groupProbes
      .filter(({ group: g }) => g === group)
      .filter(({ probe }) => probe !== "__total__")
      .map(({ probe }) => probe),
  )

  const root = d3.select("#graph")
  const getRowElement = (probe: string) => root.select(`div[data-group-id=${group}][data-probe-id=${probe}]`)
  const getProbesContainer = () => root.select(`div[data-group-id=${group}].probes-container`)

  for (const probe in probes) {
    if (!probes.hasOwnProperty(probe)) {
      continue
    }

    if (!expectedProbes.has(probe)) {
      // add missing row, the probe might be from the past
      addProbeRow(getProbesContainer(), group, probe)
    }

    // Render pies
    const episodes = probes[probe]
    const rowEl = getRowElement(probe)
    rowEl.selectAll(".cell-data").remove()
    episodes.forEach(function (episode, i) {
      const cell = rowEl.append("div").attr("class", "graph-cell cell-data")

      const viewBox = [0, 0, pieBoxWidth, pieBoxWidth].join(" ")

      const svg = cell //
        .append("svg")
        .attr("width", pieBoxWidth)
        .attr("height", pieBoxWidth)
        .attr("viewBox", viewBox)

      drawOnePie(svg, settings, episode, "probe")
    })
  }
  // }
}

// pie for each "g"
const pie = d3
  .pie()
  .padAngle(0)
  .sort(null)
  .value((x) => x.valueOf())

const arcs = {
  group: (function () {
    const radius = pieBoxWidth / 2
    return d3
      .arc()
      .innerRadius(0)
      .outerRadius(radius - 1)
  })(),
  probe: (function () {
    const radius = (pieBoxWidth * 0.8) / 2
    return d3
      .arc()
      .innerRadius(0)
      .outerRadius(radius - 1)
  })(),
}

function toPieData(d: Episode): Array<{ name: string; valueOf(): number }> {
  const fields = ["up", "down", "muted", "unknown", "nodata"]
  const listedTimers: Array<{ name: string; valueOf(): number }> = []

  for (const [field, value] of Object.entries(d)) {
    if (!fields.includes(field)) {
      continue
    }
    listedTimers.push({
      name: field,
      valueOf: () => +value,
    })
  }

  return listedTimers
}

function drawOnePie(root: any, settings: LegacySettings, episode: Episode, pieType: "group" | "probe") {
  const halfWidth = pieBoxWidth / 2

  const pieRoot = root
    .append("g")
    .attr("class", "statusPie")
    .attr("height", pieBoxWidth)
    .attr("width", pieBoxWidth)
    .attr("transform", () => `translate(${halfWidth},${halfWidth})`)

  pieRoot
    .selectAll("path")
    .data(pie(toPieData(episode)))
    .join("path")
    .attr("class", (d: { data: { name: string } }) => "pie-seg-" + d.data.name)
    .attr("d", arcs[pieType])
    .append("title")
    .text((d: { data: { name: string; valueOf(): number } }) => `${d.data.name}: ${d.data.valueOf().toLocaleString()}`)

  // Add text with availability percents
  pieRoot
    .append("text")
    .text(availabilityPercent(+episode.up, +episode.down, +episode.muted, 2))
    .attr("class", `pie-text-${pieType}`)

  // Add a transparent rectangle to use
  // as a bounding box for click events and for tooltip hover events.
  const boundingRectRoot = pieRoot.append("g")

  const onClick = () => getTimeRangeSrv().drillDownStep(+episode.ts)

  ReactDOM.render(<PieBoundingRect size={pieBoxWidth} episode={episode} onClick={onClick} />, boundingRectRoot.node())
}

function addProbeRow(
  probeContainer: d3.Selection<d3.BaseType, unknown, HTMLElement, any>,
  group: string,
  probe: string,
) {
  const probeChartRow = probeContainer
    .append("div")
    .attr("data-group-id", group)
    .attr("data-probe-id", probe)
    .attr("class", "graph-row")
    .classed("row-probe", true)

  const probeLabelCell = probeChartRow.append("div").attr("class", "graph-cell graph-labels")

  const { title: probeTitle } = getProbeSpec(group, probe)

  probeLabelCell.append("span").text(probeTitle).attr("class", "probe-label")
  const infoEl = probeLabelCell.append("div").attr("class", "group-probe-info")

  ReactDOM.render(
    <Tooltip content={<GroupProbeTooltip groupName={group} probeName={probe} />} placement="right-start">
      <Icon name="fa-info-circle" className="group-probe-info" />
    </Tooltip>,
    infoEl.node(),
  )
}
