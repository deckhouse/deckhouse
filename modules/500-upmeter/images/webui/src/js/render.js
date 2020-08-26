import {topTicks} from "./topticks";
import {dataset} from "./dataset";
import {langPack} from '../i18n/en';
import {calculateTopTicks} from "./topticks";
import { jsonToHash, secondsToHumanReadable} from "./util";

import * as d3 from './d3';


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

let svg = d3.select("#graph")
  .attr("width", width)
  .attr("height", height)
  .attr("viewBox", [0, 0, width, height]);

export function renderGraph(settings) {
  // recalculate y coordinate and height for each row
  let rowY = topTicksHeight;
  dataset.forEach(function(item, i) {
    let rowHeight = pieBoxWidth + pieSpace;
    if (item["probes"]) {
      if (item["probes"].length === 0 || item["state"] !== "display") {
        rowHeight = 0;
      } else {
        rowHeight = item["probes"].length * (pieBoxWidth + pieSpace);
      }
    }
    dataset.get(i)["rowY"] = rowY;
    dataset.get(i)["rowHeight"] = rowHeight;
    rowY += rowHeight;
  });

  let svgHeight = rowY + pieSpace;
  svg.attr("height", svgHeight)
    .attr("viewBox", [0, 0, width, svgHeight]);


  // Recreate top ticks
  svg.select(".top-ticks").remove();

  // Time range ticks are drawn between pies, so shift Y coordinate to half of a pie.
  let topGraphTicksY = 10+legendWidth - (pieBoxWidth+pieSpace)/2 ;
  let ticksEl = svg.append("g")
    .attr("class", "top-ticks")
    .attr("transform", `translate(${topGraphTicksY}, 15)`);

  calculateTopTicks(dataset, settings);

  topTicks.forEach(function(tick, i) {
    ticksEl.append("text")
      .attr("x", i*(pieBoxWidth + pieSpace))
      .attr("y", topTicksHeight/2)
      .attr("text-anchor", "middle")
      .attr("data-timeslot", tick.ts)
      .style("font-size", "12px")
      .text(tick.text);

    ticksEl.append("line")
      .attr("x1", i*(pieBoxWidth + pieSpace))
      .attr("y1", topTicksHeight-5)
      .attr("x2", i*(pieBoxWidth + pieSpace))
      .attr("y2", rowY - pieSpace -5 )
      .attr("class", "tick-line")
  });

  // left/right/now controls
  //svg.append("g")


  //renderStepLabels();

  //renderGroupRows();

  // Bind groups of statuses
  // FIXME this setup doesn't delete group rows.
  let groups = svg.selectAll(".groupRow")
    .data(dataset.data, d => d.group);

  groups.exit().remove();

  // each group is a table row, so translate "g" element to proper y position
  let groupsEnter = groups.enter()
    .append("g")
    .attr("class","groupRow")
    .attr("transform",function(d,i) {
      return "translate(10,"+(d.rowY /*  i * pieBoxWidth + i * pieSpace */)+")";
    })
    .style("fill", function(d, i) { return d3.schemeCategory10[i%10]; });

  // Label with group name.
  // caret-right - closed
  // caret-down - opened
  // group name labels
  groupsEnter.each(function(e,d){
    if (e["statuses"]) {
      let groupLabelEl = d3.select(this)
        .append("g")
        .attr("data-group-id", d => d.group);

      // add open/close icon
      groupLabelEl.append("g")
        .attr("transform", `translate(0, ${((pieBoxWidth + pieSpace) / 2) - 8})`)
        .attr("width", 16) // These attributes are copied
        .attr("height", 16) // by fontawesome onto new svg element.
        .attr("class", "fas fa-caret-right group-icon")

      // Add label
      groupLabelEl.append("text")
        .text(function (d) {
          return d.label;
        })
        .attr("x", 20)
        .attr("y", d => (pieBoxWidth + pieSpace) / 2)

        .attr("class", "group-label")

      groupLabelEl.on('click', function (d) {
        let group = $(this).attr("data-group-id");
        let shouldRequestData = false;
        // find dataset item for probes and toggle state
        dataset.forEach(function (item, i) {
          if (item.group === group && item["probes"]) {
            let state = dataset.get(i)["state"];
            if (state === "startup") {
              shouldRequestData = true;
            }
            if (state !== "display") {
              dataset.get(i)["state"] = "display";
            } else {
              dataset.get(i)["state"] = "hidden";
            }
          }
        });
        // toggle icon
        let iconEl = groupLabelEl.select(".group-icon svg[data-fa-i2svg]")
        iconEl.classed("fa-caret-down", !iconEl.classed("fa-caret-down"));
        iconEl.classed("fa-caret-right", !iconEl.classed("fa-caret-right"));
        // trigger event to re-render graph
        // FIXME To heavy operation, need to rework one svg into multiple svgs and hide them instead of recalculate coordinates.
        $('body').trigger('updateGroupProbes', {
          "msg": "label clicked",
          "shouldRequestData": shouldRequestData,
          "group": group
        });
      });

      groupLabelEl.on("mouseover", function(e, d) {
        tooltip.transition()
          .duration(200)
          .style("opacity", .9);

        let tooltipContent = langPack.group[$(this).attr("data-group-id")];

        tooltip.html(tooltipContent)
          .style("left", (e.pageX + 10) + "px")
          .style("top", (e.pageY + 20) + "px");
      })
        .on("mouseout", function(d) {
          tooltip.transition()
            .duration(250)
            .style("opacity", 0);
        })

    }
  });


  groupsEnter.append("g")
    .attr("class",function(d) {
      if (d.statuses) {
        return "statuses";
      } else {
        return "probe-statuses";
      }
    })
    .attr("data-group-id", d => d.group)
  ;

  // merge entered parents with pre-existing:
  groups = groupsEnter.merge(groups);

  groups.each(function(e, d) {
    // probes can be hidden
    if (e["probes"]) {
      if (e["state"] === "hidden") {
        d3.select(this)
          .attr("transform", function (d, i) {
            return "translate(10," + (d.rowY) + ") scale(1,0)";
          })
      } else if (e["state"] === "display") {
        d3.select(this)
          .attr("transform", function (d, i) {
            return "translate(10," + (d.rowY) + ") scale(1,1)";
          })
      }
    }
    // group total
    if (e["statuses"]) {
      d3.select(this)
        .attr("transform", function (d, i) {
          return "translate(10," + (d.rowY) + ")";
        })
    }
  });


  // requires when exit is in use
  // groups.transition().attr("transform",function(d,i) { return "translate(10,"+(i*24+20)+")"; }).duration(t);
  // groups.transition()
  //       .attr("transform",function(d,i) { return "translate(10,"+(d.rowY)+")"; })
  //       //.attr("y", function(d) { return d.rowY })
  //       .duration(t);

  // Select a child group in each parent:
  // Draw pies for __total__
  let statuses = groups.select(".statuses");

  let statusPies = statuses.selectAll(".statusPie")
    .data(function(d) { return d.statuses || []; }, function(d) {
      let id = ""+d.ts+d.up+d.down+d.muted+d.unknown+d.nodata+d.downtimes;
      return id;
    });

  drawPie(statusPies, settings, "group");


  // Draw row for each probe, draw label and pies in each row. As for groups.
  let probeRows = groups.select(".probe-statuses")
    .selectAll(".probeRaw")
    .data(function(d) { return d.probes || []; }, function(d) { return d; });

  let probeRowsEnter = probeRows.enter()
    .append("g")
    .attr("class", "probeRow")
    .attr("transform",function(d,i) {
      return "translate(0,"+(i * pieBoxWidth + i * pieSpace)+")";
    })

  probeRowsEnter.append("text")
    .text(d => d.probe)
    .attr("class", "probe-label")
    .attr("x", "30") // small padding
    .attr("y", d => (pieBoxWidth+pieSpace) / 2)
    .on("mouseover", function(e, d) {
      tooltip.transition()
        .duration(200)
        .style("opacity", .9);

      let group_id = $(this).parent().parent().attr("data-group-id");

      let tooltipContent = langPack.probe[group_id][d.probe];

      tooltip.html(tooltipContent)
        .style("left", (e.pageX + 10) + "px")
        .style("top", (e.pageY + 20) + "px");
    })
    .on("mouseout", function(d) {
      tooltip.transition()
        .duration(250)
        .style("opacity", 0);
    });

  probeRowsEnter.append("g")
    .attr("class", "statuses");

  probeRows = probeRowsEnter.merge(probeRows);

  let probeStatuses = probeRows.select(".statuses");

  let probeStatusPies = probeStatuses.selectAll(".statusPie")
    .data(function(d) { return d.statuses || []; }, function(d) { return d; })

  drawPie(probeStatusPies, settings, "probe");
}


// pie for each "g"
const pie = d3.pie()
  .padAngle(0)
  .sort(null)
  .value(d => d.value);

const arcs = {
  "group": function(){
    const radius = pieBoxWidth / 2;
    return d3.arc().innerRadius(radius * pieInnerRadius).outerRadius(radius - 1);
  }(),
  "probe": function(){
    const radius = pieBoxWidth*0.9 / 2;
    return d3.arc().innerRadius(radius * pieInnerRadius * 1.05).outerRadius(radius - 1);
  }(),
};

// Define the div for the tooltip
const tooltip = d3.select("body").append("div")
  .attr("class", "tooltip")
  .style("opacity", 0);

const toPieData = function(d) {
  return ["up", "down", "muted", "unknown", "nodata"].map(n => {
    return {"name": n, "value": d[n]};
  });
};

const drawPie = function(selection, settings, pieType) {
  selection.exit().remove();

  let pieWidth = pieBoxWidth;
  if (pieType === "probe"){
    pieWidth = pieBoxWidth*0.85;
  }

  let statusEnter = selection.enter()
    .append("g")
    .attr("class","statusPie")
    .attr("height", pieBoxWidth)
    .attr("width", pieBoxWidth)
    .attr("transform",function(d,i) {
      return "translate("+(legendWidth + i * pieBoxWidth + i*pieSpace)+","+(pieBoxWidth/2+pieSpace/2)+")";
    })

  statusEnter.selectAll("path")
    .data(d => pie(toPieData(d)))
    .join("path")
    .attr("class", d => "pie-seg-"+d.data.name)
    .attr("d", arcs[pieType])
    .append("title")
    .text(d => `${d.data.name}: ${d.data.value.toLocaleString()}`);

  statusEnter.each(function(d) {
    if (d.nodata === settings.getStep()) {
      return
    }

    let el = d3.select(this);

    // Add text with availability percents
    el.append("text")
      .text(function(d){
        return availabilityPercent(+d.up, +d.down, +d.muted, 2);
      })
      .attr("class", `pie-text-${pieType}`);

    // add transparent rectangles to use
    // as a bounding box for on click events
    let boundingRect = el.append("rect")
      .attr("x", -pieBoxWidth/2)
      .attr("y", -pieBoxWidth/2)
      .attr("width", pieBoxWidth)
      .attr("height", pieBoxWidth)
      .style("fill", "rgba(0,0,0,0)")

    boundingRect
      .on("mouseover", function(e, d) {
        tooltip.style("display", "block");
        tooltip.transition()
          .duration(200)
          .style("opacity", .9);

        let tooltipContent = tooltipText(d)

        tooltip.html(tooltipContent)
          .style("left", (e.pageX + 15) + "px")
          .style("top", (e.pageY - 10) + "px");
      })
      .on("mouseout", function(d) {
        tooltip.transition()
          .duration(250)
          .style("opacity", 0);
      })

    if (settings.getStep() > 300) {
      boundingRect.style("cursor", "pointer");
      boundingRect.on("click", function (e, d) {
        if (settings.getStep() <= 300) {
          return;
        }
        // change graph to range of clicked pie (drill down)
        let step = Math.floor(settings.getStep() / settings.timeRange.count);
        let count = settings.timeRange.count;
        if (step < 300) {
          step = 300;
        }

        let newTimeRange = {
          from: d.ts,
          to: d.ts + settings.getStep(),
          //count: settings.timeRange.count,
          step: step
        }

        settings.setTimeRange(settings);

        if (history.pushState) {
          history.pushState(null, null, '#' + jsonToHash({
            from: newTimeRange.from,
            to: newTimeRange.to,
            step: newTimeRange.step
          }));
        } else {
          location.hash = newHash;
        }

        // TODO get rid of jquery
        //d3.selectAll('body').dispatch('updateGraph', {"msg": "onStartup"});
        $('body').trigger('updateGraph', {"msg": "onStartup"})

      })
    } else {
      boundingRect.style("cursor", "default");
    }
  });
}

const tooltipText = function(d) {
  let tmpl = `
    <p class="tooltip-head">${availabilityPercent(d.up, d.down, d.muted, 4)}</p>
    #UP_TEXT#
    #DOWN_TEXT#
    #MUTED_TEXT#
    #UNKNOWN_TEXT#
    #NODATA_TEXT#
    #DOWNTIMES_TEXT#
    `;

  if (d.up) {
    tmpl = tmpl.replaceAll(/#UP_TEXT#\n/gi,
      `<p>Up: ${secondsToHumanReadable(+d.up)}</p>`);
  }
  if (d.down) {
    tmpl = tmpl.replaceAll(/#DOWN_TEXT#\n/gi,
      `<p>Down: ${secondsToHumanReadable(+d.down)}</p>`);
  }
  if (d.muted) {
    tmpl = tmpl.replaceAll(/#MUTED_TEXT#\n/gi,
      `<p>Muted: ${secondsToHumanReadable(+d.muted)}</p>`);
  }
  if (d.unknown) {
    tmpl = tmpl.replaceAll(/#UNKNOWN_TEXT#\n/gi,
      `<p>Unknown: ${secondsToHumanReadable(+d.unknown)}</p>`);
  }
  if (d.nodata) {
    tmpl = tmpl.replaceAll(/#NODATA_TEXT#\n/gi,
      `<p>Nodata: ${secondsToHumanReadable(+d.nodata)}</p>`);
  }
  if (d.downtimes) {
    tmpl = tmpl.replaceAll(/#DOWNTIMES_TEXT#\n/gi,
      `<p>Downtimes: ${d.downtimes}</p>`);
  }

  return tmpl.replaceAll(/#.*?#\n/gi, '');
}

const availabilityPercent = function(up, down, muted, precision) {
  if ((+up+down+muted) === 0) {
    return ""
  }
  if (+up+muted === 0 && +down > 0) {
    return "0%"
  }
  if (+down === 0) {
    return "100%"
  }
  let pieValue = (+up+muted)/(+up+muted+down) ///settings.step;
  return d3.format(`.${precision}%`)(pieValue)
}
