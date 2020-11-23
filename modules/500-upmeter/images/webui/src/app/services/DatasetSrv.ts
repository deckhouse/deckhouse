import { Dataset } from './Dataset';

import {renderGraphTable, renderGroupData, renderGroupProbesData} from "../graph/render";
import {updateTicks} from "../graph/render";

import * as d3 from '../libs/d3';
import {getTimeRangeSrv} from "./TimeRangeSrv";

export class DatasetSrv {
  dataset: Dataset;

  groups: string[] = [];
  groupProbes:any[] = [];
  groupState:any = {};

  constructor() {
    this.dataset = new Dataset();
  }

  getDataset():Dataset {
    return this.dataset;
  }

  getLegacySettings():any {
    return {
      groups: this.groups,
      groupProbes: this.groupProbes,
      groupState: this.groupState,
      //dataset: me.dataset,
      timeRange: {
        to: getTimeRangeSrv().to,
        from: getTimeRangeSrv().from,
        step: getTimeRangeSrv().step,
        topTickFormat: getTimeRangeSrv().settings.fmt,
      }
    }
  }

  requestGroups() {
    /*
    * data is an array of {group:string, probe: string} objects
    */
    let me = this;
    d3.json(`/api/probe`).then(function(data:any){
      me.groups = [];
      me.groupProbes = [];
      me.groupState = {};
      data.map(function(d:any){
        if (!me.groupState[d.group]) {
          me.groupProbes.push({
            "group": d.group,
            "probe": "__total__",
            "type": "group"
          })
          me.groupState[d.group] = {"expanded": false, "probe-data-loaded": false};
          me.groups.push(d.group);
        }
        me.groupProbes.push({
          "group": d.group,
          "probe": d.probe,
          "type": "probe"
        })
      });
      let flag = false;
      let count = me.groupProbes.length;
      for(let i=0; i<count; i++) {
        if (me.groupProbes[i].type=="group") {
          flag=true;
          continue;
        }
        if (flag) {
          me.groupProbes[i].type="first-in-group";
          flag=false;
        }
      }
      flag=true;
      for(let i=count-1; i>=0; i--) {
        if (me.groupProbes[i].type=="group") {
          flag=true;
          continue;
        }
        if (flag) {
          me.groupProbes[i].type="last-in-group";
          flag=false;
        }
      }

      // Fill dataset with groups and probes.
      me.dataset.clear();
      me.groups.forEach(function(group) {
        me.dataset.push({
          "group": group,
          "label": group,
          "statuses": [],
          //"timeRange": graphSettings.timeRange
        });
        me.dataset.push({
          "group": group,
          "probes": [],
          "state": "startup",
          //"timeRange": graphSettings.timeRange
        });
      });


      renderGraphTable(me.dataset, me.getLegacySettings());

      me.groups.forEach(function(group) {
        me.requestGroupData(group);
      });
    });
  }

  requestGroupData = (group:string) => {
    let me = this;
    let fromToStep = getTimeRangeSrv().getFromToStepAsUri();
    let muteTypes = getTimeRangeSrv().getMuteTypesAsUri();
    const url = `/api/status/range`+
      `?${fromToStep}` +
      `&group=${group}&probe=__total__` +
      `&muteDowntimeTypes=${muteTypes}`
    d3.json(url).then(function(d:any) {
      // Ignore empty response
      if (!d || d.length === 0 || !(d["statuses"]) || d.statuses.length === 0 || !(d.statuses[group]) ) {
        return
      }

      me.dataset.forEach(function(item:any, i:number){
        if (item.group === group && item["statuses"]) {
          me.dataset.get(i).statuses = d.statuses[group]["__total__"];
        }
      })

      updateTicks(me.dataset, me.getLegacySettings());
      renderGroupData(me.dataset, me.getLegacySettings(), group, d);

      // If group is expanded and state is not expanded, then load data for group probes.
      // If group is not xpanded and group state is expanded, then hide group probes;
      let expanded = getTimeRangeSrv().isGroupExpanded(group);
      let groupState = false;
      if (me.groupState && me.groupState[group]) {
        if (me.groupState[group].hasOwnProperty("expanded")) {
          groupState = me.groupState[group]["expanded"];
        }
      }

      if (expanded !== groupState) {
        // TODO: migrate graph to React and remove this hack.
        // HACK: groups are not expanded by default, so emulate click on label to expand group.
        d3.selectAll(`[data-group-id="${group}"][data-probe-id=__total__] .graph-cell.graph-labels`).dispatch('click');
      }
    });

  }

  requestGroupProbesData = (group:string) => {
    let me = this;
    let fromToStep = getTimeRangeSrv().getFromToStepAsUri();
    let muteTypes = getTimeRangeSrv().getMuteTypesAsUri();
    const url = `/api/status/range`+
      `?${fromToStep}` +
      `&group=${group}&probe=__all__` +
      `&muteDowntimeTypes=${muteTypes}`

    d3.json(url).then(function(d:any) {
      if (!d || d.length === 0 || !(d["statuses"]) || d.statuses.length === 0 || !(d.statuses[group]) ) {
        return
      }

      me.dataset.forEach(function(item, i){
        if (item.group === group && item["probes"]) {
          for(let k in d.statuses[group]) {
            me.dataset.get(i).probes.push({
              "probe": k,
              "statuses": d.statuses[group][k]
            });
          }
        }
      });
      me.groupState[group]["probe-data-loaded"] = true

      renderGroupProbesData(me.getLegacySettings(), group, d);
    });
  }

}

let instance: DatasetSrv;

export function setDatasetSrv(srv: DatasetSrv) {
  instance = srv;
}

export function getDatasetSrv(): DatasetSrv {
  return instance;
}
