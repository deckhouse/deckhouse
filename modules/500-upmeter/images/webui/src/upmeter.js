import('./css/main.css');
import('./css/upmeter-graph.css');

import './js/fontawesome';

import {renderGraph} from './js/render';
import {dataset} from './js/dataset';
import {jsonFromHash, jsonToHash, secondsToHumanReadable} from "./js/util";

import * as d3 from './js/d3';
import { DateTime } from 'luxon';
import $ from 'jquery';


document.addEventListener('DOMContentLoaded', function () {
  $('.btn-group-step button').on('click', toggleTimeRangeGroup)
  $('.btn-group-mutetype button').on('click', toggleCheckSquareIcon);
  $('button.btn-now').on('click', resetTimeRangeToNow);

  $('body').on('updateGraph', function(ev, d) {
    let onStartup = (d.msg === "onStartup");
    updateGraphSettings(onStartup);
    requestGroups();
  })
    // update graph on page load
    .trigger('updateGraph', {"msg":"onStartup"});
});

// 'Back' button
window.onpopstate = function(event) {
  $('body').trigger('updateGraph', {"msg":"onStartup"});
}

$('body').on('updateGroupProbes', function(ev, d) {
  if (d.shouldRequestData) {
    // request API for all probes in group.
    requestGroupProbesData(d.group, graphSettings, function(){
      renderGraph(graphSettings);
    });
  } else {
    renderGraph(graphSettings);
  }
});



const toggleCheckSquareIcon = function() {
  $(this)
    .toggleClass('active')
    .find('svg[data-fa-i2svg]')
    .toggleClass('fa-check-square')
    .toggleClass('fa-square');
  // get active buttons, send event
  $('body').trigger('updateGraph', {"msg":"mutetype"})
}

const toggleTimeRangeGroup = function() {
  $(this).parent().find("button")
    .removeClass("active");
  $(this).addClass("active");

  // Reset open/close icons for group labels
  $('svg#graph .group-icon svg[data-fa-i2svg]')
    .removeClass('fa-caret-down')
    .addClass('fa-caret-right');

  // send 'update' event
  $('body').trigger('updateGraph', {"msg":"timerange"})
}

const resetTimeRangeToNow = function() {
  // FIXME quick hack
  let to = Math.floor(DateTime.utc().toSeconds());
  let from = Math.floor(to - (graphSettings.timeRange.to - graphSettings.timeRange.from));

  let newTimeRange = {
    from: from,
    to: to,
    step: graphSettings.getStep()
  }

  graphSettings.setTimeRange(newTimeRange);

  if (history.pushState) {
    history.pushState(null, null, '#' + jsonToHash({
      from: newTimeRange.from,
      to: newTimeRange.to,
      step: newTimeRange.step
    }));
  } else {
    location.hash = newHash;
  }
  $('body').trigger('updateGraph', {"msg": "onStartup"})
}

// Singleton with graph settings
let graphSettings = (function(){
  function TimeRange() {
    this.from = 0;
    this.to = 0;
    this.step = 0;
    this.count = 0;
    this.topTickFormat = "";
    this.topTicks = [];
    //this.topTicksBetween = true;
    //this.realTopTicks = [];
  }
  function GraphSettings() {
    this.muteTypes = [];
    this.groups = [];
    this.groupProbes = [];
    this.timeRange = new TimeRange();
  }

  // GraphSettings.prototype.getRealTopTicks = function() {
  //   return this.timeRange.topTicks;
  // }
  // GraphSettings.prototype.getTopTicksBetween = function() {
  //   return this.timeRange.topTicksBetween;
  // }
  GraphSettings.prototype.getStep = function() {
    return this.timeRange.step;
  }
  GraphSettings.prototype.getFromToStepAsUri = function() {
    return `from=${encodeURIComponent(this.timeRange.from)}`+
      `&to=${encodeURIComponent(this.timeRange.to)}`+
      `&step=${encodeURIComponent(this.timeRange.step)}`
  }

  GraphSettings.prototype.setTimeRange = function(obj) {
    for (let p in obj) {
      if (obj.hasOwnProperty(p) && this.timeRange.hasOwnProperty(p)) {
        this.timeRange[p] = obj[p];
      }
    }
  }

  GraphSettings.prototype.setMuteTypes = function(types) {
    this.muteTypes = types;
  }

  return new GraphSettings();
})();

const updateGraphSettings = function(onStartup) {
  if (onStartup) {
    // load from hash
    let hash = location.hash.substr(1);
    if (hash !== "") {
      let params = jsonFromHash(hash);
      let settings = calculateGraphSettings("hash", params);
      if (settings.error) {
        // TODO show popup with error message?
      } else {
        graphSettings.setTimeRange(settings);
        updateStepIndicator(settings.step);
        // remove active from all step buttons, activate button if step is matching with its range id
        $(".btn-group-step button").each(function (i, e) {
          $(this).removeClass("active");
          let rangeId = $(this).attr("data-time-range");
          let step = getStepForRangeId(rangeId);
          if (step === settings.step) {
            $(this).addClass("active");
          }
        });
        return
      }
    }
  }

  // update mute types
  let muteTypes = [];
  $(".btn-group-mutetype button.active").each(function(i, e){
    muteTypes.push($(this).attr("data-mute-type"));
  });
  graphSettings.setMuteTypes(muteTypes);

  let rangeId = $(".btn-group-step button.active").attr("data-time-range");

  let to = graphSettings.timeRange.to;

  if (!to) {
    to = Math.floor(DateTime.utc().toSeconds());
  }

  let settings = calculateGraphSettings(rangeId, {"to": to});
  graphSettings.setTimeRange(settings);

  // Save to hash, update address bar.
  let newHash = '#'+jsonToHash({
    from: settings.from,
    to: settings.to,
    step: settings.step
    //count: settings.count
    // TODO add mute types
  });
  if(history.pushState) {
    history.pushState(null, null, newHash);
  }
  else {
    location.hash = newHash;
  }

  updateStepIndicator(settings.step);
}

const updateStepIndicator = function(stepSeconds) {
  let humanStep = secondsToHumanReadable(stepSeconds);
  $('#step-indicator').text("Step: "+humanStep);
}


const calculateGraphSettings = function(rangeId, hashObj) {
  let nowUnix = Math.floor(DateTime.utc().toSeconds());
  let from = 0, to = 0, step = 0, count = 12;

  if (rangeId === "hash") {
    if (!!hashObj["to"] && hashObj["to"] === "now" && !!hashObj["step"]) {
      step = +hashObj["step"];
      to = nowUnix;
      from = to - count * step;
    } else if (!!hashObj["from"] && !!hashObj["to"] && !!hashObj["step"]) {
      step = +hashObj["step"];
      to = +hashObj["to"];
      from = +hashObj["from"];
      count = Math.floor((to - from) / step);
    } else {
      let error = "from, to and step are required params"
      return {error: error}
    }

    return {
      from: from,
      to: to,
      step: step,
      count: count,
      topTickFormat: hashObj["fmt"],
      // topTicks: generateTicks(now, count, true, step, hashObj["fmt"]),
      // topTicksBetween: true
    }
  }

//  if ( === )
  to = +hashObj["to"];

  step = getStepForRangeId(rangeId);

  if (rangeId === "week") {
    count = 7;
  } else {
    count = 12;
  }

  return {
    from: to - count * step,
    to: to,
    step: step,
    count: count,
    // topTicks: generateTicks(now, count, true, step),
    // topTicksBetween: true
  }

}

const getStepForRangeId = function(rangeId) {
  let step = 300;
  if (rangeId === "3hr") {
    step = 20 * 60; // 20 minutes
  } else if (rangeId === "day") {
    step = 2 * 60 * 60; // 2 hours
  } else if (rangeId === "week") {
    step = 24 * 60 * 60; // 1 day
  } else if (rangeId === "month") {
    step = 3 * 24 * 60 * 60; // 3 days
  } else if (rangeId === "quarter") {
    step = 7 * 24 * 60 * 60; // 7 days
  } else if (rangeId === "year") {
    step = 30 * 24 * 60 * 60; // 30 days
  }
  return step;
}


const requestGroups = function() {
  d3.json(`/api/probe`).then(function(data){
    data.map(function(d){
      if (!graphSettings.groupProbes[d.group]) {
        graphSettings.groupProbes[d.group] = [];
      }
      graphSettings.groupProbes[d.group].push(d.probe)
    });
    graphSettings.groups = [];
    for(let k in graphSettings.groupProbes) {
      graphSettings.groups.push(k);
    }

    // Init dataset
    dataset.clear();
    graphSettings.groups.forEach(function(group) {
      dataset.push({
        "group": group,
        "label": group,
        "statuses": [],
        "timeRange": graphSettings.timeRange
      });
      dataset.push({
        "group": group,
        "probes": [],
        "state": "startup",
        "timeRange": graphSettings.timeRange
      });
    });

    graphSettings.groups.forEach(function(group) {
      requestGroupData(group, graphSettings, function(d) {
        dataset.forEach(function(item, i){
          if (item.group === group && item.statuses) {
            dataset.get(i).statuses = d.statuses[group]["__total__"];
          }
        })
        renderGraph(graphSettings);
      });
    });
  });
}

const requestGroupData = function(group, settings, onSuccess) {
  const url = `/api/status/range`+
    `?from=${settings.timeRange.from}&to=${settings.timeRange.to}&step=${settings.timeRange.step}` +
    `&group=${group}&probe=__total__` +
    `&muteDowntimeTypes=${settings.muteTypes}`
  d3.json(url).then(function(d) {
    // Ignore empty response
    if (!d || d.length === 0 || !(d["statuses"]) || d.statuses.length === 0 || !(d.statuses[group]) ) {
      return
    }

    dataset.forEach(function(item, i){
      if (item.group === group && item["statuses"]) {
        dataset.get(i).statuses = d.statuses[group]["__total__"];
      }
    })
    renderGraph(graphSettings);
  });
}

const requestGroupProbesData = function(group, settings, onSuccess) {
  const url = `/api/status/range`+
    `?from=${settings.timeRange.from}&to=${settings.timeRange.to}&step=${settings.timeRange.step}` +
    `&group=${group}&probe=__all__` +
    `&muteDowntimeTypes=${settings.muteTypes}`

  d3.json(url).then(function(d) {
    if (!d || d.length === 0 || !(d["statuses"]) || d.statuses.length === 0 || !(d.statuses[group]) ) {
      return
    }

    dataset.forEach(function(item, i){
      if (item.group === group && item["probes"]) {
        for(let k in d.statuses[group]) {
          dataset.get(i).probes.push({
            "probe": k,
            "statuses": d.statuses[group][k]
          });
        }
      }
    });
    renderGraph(graphSettings);
  });
}

