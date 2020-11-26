import React from 'react';
import ReactDOM from 'react-dom';
import * as d3 from '../libs/d3';


// Components
import {Tooltip} from "@grafana/ui"
import {Icon} from "../components/Icon";
import {GroupProbeTooltip} from "../components/GroupProbeTooltip";
import {PieBoundingRect} from "../components/PieBoundingRect";

// Services
import {langPack} from "../i18n/en";
import {topTicks, calculateTopTicks} from './topticks';
import {getEventsSrv} from "../services/EventsSrv";
import {getTimeRangeSrv} from "../services/TimeRangeSrv";
import {availabilityPercent} from "../util/humanSeconds";

// Groups layout from https://bl.ocks.org/Andrew-Reid/960819e98873bbaf035bbf6bd2774b40
// Pie from https://observablehq.com/@d3/donut-chart
// Tooltip https://bl.ocks.org/d3noob/a22c42db65eb00d4e369
// Text alignment https://bl.ocks.org/emmasaunders/0016ee0a2cab25a643ee9bd4855d3464
// Format numbers https://github.com/d3/d3-format

// Settings
const pieBoxWidth = 60,
  pieSpace = 15,
  pieInnerRadius = 0.67,
  legendWidth = 250,
  topTicksHeight = 30,
  leftPadding = 30,
  rightPadding = 30
;

let width = leftPadding + legendWidth + (12*(pieBoxWidth + pieSpace) + pieSpace) + rightPadding;
let height = topTicksHeight + 5 * (pieBoxWidth + pieSpace) + pieSpace;

//let root = d3.select("#graph")
//  .attr("width", width)
//  .attr("height", height)
//  .attr("viewBox", [0, 0, width, height]);

export function renderGraphTable(dataset, settings) {
  let root = d3.select('#graph');

  // Always recreate everything
  root.selectAll("*").remove();

  root.append("div")
    .attr("class", "top-ticks")
    .append("div")
    .attr("class", "top-tick-left")
    .append("div")
    .attr("class", "top-tick-left-spacer")

  updateTicks(dataset, settings);

  let graphTable = root.append("div")
    .attr("class", "graph-table")

  let groups = graphTable.selectAll(".graph-row")
    .data(settings.groupProbes);

  // each group is a table row, so translate "g" element to proper y position
  let groupsEnter = groups.enter()
    .append("div")
    .attr("data-group-id", d => d.group)
    .attr("data-probe-id", d => d.probe)
    .attr("class","graph-row")

  // Labels for group and probes.
  groupsEnter.each(function(item){
    let rowEl = d3.select(this);
    let cellEl = rowEl
      .append("div")
      .attr("class", "graph-cell graph-labels");

    if (item.probe === "__total__") {
      // total is always visible
      rowEl.classed("graph-row-visible", true);

      // add open/close icon
      // fas - fontawesome icon
      // fa-fw - fixed width https://fontawesome.com/how-to-use/on-the-web/styling/fixed-width-icons
      // fa-** - icon name
      // caret-right - closed icon for group
      // caret-down - opened icon for group
      cellEl.append("i")
        .attr("class", "fas fa-fw fa-caret-right group-icon")

      // Add label
      cellEl.append("span")
        .text(item.group)
        .attr("class", "group-label")

      cellEl.on('click', function (d) {
        // TODO add visibility indicator to request probes data without additional clicks when change intervals.

        let probeRows = d3.selectAll(`#graph div[data-group-id=${item.group}].graph-row`);

        // invert expanded
        let expanded = !settings.groupState[item.group].expanded;
        settings.groupState[item.group].expanded = expanded;
        getTimeRangeSrv().onExpandGroup(item.group, expanded);

        probeRows.each(function(){
          let rowEl = d3.select(this)
          let probeId = rowEl.attr('data-probe-id');
          if (probeId === "__total__") {
            return
          }
          rowEl.classed("graph-row-hidden", !rowEl.classed("graph-row-hidden"));
          rowEl.classed("graph-row-visible", !rowEl.classed("graph-row-visible"));
        })

        // toggle icon
        let iconEl = cellEl.select(".group-icon svg[data-fa-i2svg]")
        iconEl.classed("fa-caret-down", settings.groupState[item.group].expanded);
        iconEl.classed("fa-caret-right", !settings.groupState[item.group].expanded);

        // trigger event to re-render graph
        if (!settings.groupState[item.group]["probe-data-loaded"]){
          getEventsSrv().fireEvent("UpdateGroupProbes", {group: item.group})
        }
      });

    } else {
      rowEl.classed("graph-row-hidden", true);
      rowEl.classed(`row-${item.type}`, true);
      rowEl.classed(`row-probe`, true);

      // Add label
      cellEl.append("span")
        .text(item.probe)
        .attr("class", "probe-label")
    }

    let infoEl = cellEl.append("div")
      .attr("class", "group-probe-info")

    ReactDOM.render(
      <Tooltip content={<GroupProbeTooltip groupName={item.group} probeName={item.probe}/>} placement="right-start">
        <Icon name="fa-info-circle" className="group-probe-info" />
      </Tooltip>
        ,
      infoEl.node()
    )
  });


  // Each row has empty cell to define initial height for empty rows
  groupsEnter.append("div")
      //.text("Data for group '" + group + "'")
      .attr("class", "graph-cell cell-data")
      .append("svg")
      .attr("width", pieBoxWidth)
      .attr("height", pieBoxWidth)

}

export function updateTicks(dataset, settings) {
  let root = d3.select("#graph div.top-ticks")
  // Always recreate top ticks
  root.selectAll("div.top-tick").remove();

  calculateTopTicks(dataset, settings);

  topTicks.forEach(function(tick, i) {
    root.append("div")
      .attr("data-timeslot", tick.ts)
      .attr("class", "top-tick")
      .append("span")
      .text(tick.text);
  });

  // 'Total' label
  root.append("div")
    .attr("class", "top-tick total-tick")
    .append("span")
    .text("Total");
}

export function renderGroupData(dataset, settings, group, data) {
  let rowEl = d3.select(`#graph div[data-group-id=${group}][data-probe-id="__total__"]`);
  rowEl.selectAll(".cell-data").remove();
  if (!data["statuses"] || !data["statuses"][group]["__total__"]) {
    console.log("Bad group data", data);
  }
  data["statuses"][group]["__total__"].forEach(function(item, i) {
    let cell = rowEl.append("div")
      //.text("Data for group '" + group + "'")
      .attr("class", "graph-cell cell-data");

    let svg = cell.append("svg")
        .attr("width", pieBoxWidth)
        .attr("height", pieBoxWidth)
        .attr("viewBox", [0, 0, pieBoxWidth, pieBoxWidth]);

    drawOnePie(svg, settings, item, "group")
  })

  let piesCount = data["statuses"][group]["__total__"].length;

  // add empty boxes into probe rows to prevent stripe background on expand
  let rows = d3.selectAll(`#graph div[data-group-id=${group}].graph-row`);
  rows.each(function(item){
    if (item.probe === "__total__") {
      return
    }
    let rowEl = d3.select(this);
    rowEl.selectAll(".cell-data").remove();
    for (let i = 0; i<piesCount; i ++) {
      rowEl.append("div")
        .attr("class", "graph-cell cell-data")
        .append("svg")
        .attr("width", pieBoxWidth)
        .attr("height", pieBoxWidth)
    }

  })
}

export function renderGroupProbesData(settings, group, data) {
  let root = d3.select("#graph");

  let statuses = data["statuses"]
  for (const group in statuses) {
    if (!statuses.hasOwnProperty(group)) {
      continue
    }
    let probes = statuses[group];
    for (const probe in probes) {
      if (!probes.hasOwnProperty(probe)) {
        continue
      }
      let rowEl = root.select(`div[data-group-id=${group}][data-probe-id=${probe}]`);
      rowEl.selectAll(".cell-data").remove();

      let cellCount = probes[probe].length;
      probes[probe].forEach(function(item, i) {
        let cell = rowEl.append("div")
          .attr("class", "graph-cell cell-data");

        if (i === 0 ) {
          cell.classed("first-in-row", true);
        }
        if (i === cellCount-1 ) {
          cell.classed("last-in-row", true);
        }

        let svg = cell.append("svg")
          .attr("width", pieBoxWidth)
          .attr("height", pieBoxWidth)
          .attr("viewBox", [0, 0, pieBoxWidth, pieBoxWidth]);

        drawOnePie(svg, settings, item, "probe")
      })
    }
  }
}

// pie for each "g"
const pie = d3.pie()
  .padAngle(0)
  .sort(null)
  .value(d => d.value);

const arcs = {
  "group": function(){
    const radius = pieBoxWidth / 2;
    return d3.arc().innerRadius(0).outerRadius(radius - 1);
  }(),
  "probe": function(){
    const radius = pieBoxWidth*0.8 / 2;
    return d3.arc().innerRadius(0).outerRadius(radius - 1);
  }(),
};

const toPieData = function(d) {
  return ["up", "down", "muted", "unknown", "nodata"].map(n => {
    return {"name": n, "value": d[n]};
  });
};

const drawOnePie = function(root, settings, data, pieType) {
  let pieRoot = root.append("g")
    .attr("class","statusPie")
    .attr("height", pieBoxWidth)
    .attr("width", pieBoxWidth)
    .attr("transform",function(d,i) {
      return `translate(${pieBoxWidth/2},${pieBoxWidth/2})`;
    })

  pieRoot.selectAll("path")
    .data(pie(toPieData(data)))
    .join("path")
    .attr("class", d => "pie-seg-"+d.data.name)
    .attr("d", arcs[pieType])
    .append("title")
    .text(d => `${d.data.name}: ${d.data.value.toLocaleString()}`);

  // Add text with availability percents
  pieRoot.append("text")
    .text(availabilityPercent(+data.up, +data.down, +data.muted, 2))
    .attr("class", `pie-text-${pieType}`);

  // Add a transparent rectangle to use
  // as a bounding box for click events and for tooltip hover events.
  let boundingRectRoot = pieRoot.append("g")

  ReactDOM.render(
    <PieBoundingRect
      width={pieBoxWidth}
      onClick={() => {
        getTimeRangeSrv().drillDownStep(+data.ts)
      }}
      data={data} />,
    boundingRectRoot.node()
  )
}
