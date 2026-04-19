import { Dataset } from "./Dataset"

import { renderGraphTable, renderGroupData, renderGroupProbesData, updateTicks } from "../graph/render"

import * as d3 from "d3"
import { getTimeRangeSrv } from "./TimeRangeSrv"

interface ProbeRef {
  group: string
  probe: string
}

export interface Episode {
  ts: number
  start: Date
  end: Date
  up: number
  down: number
  unknown: number
  muted: number
  nodata: number
  downtimes: any[]
}

export interface EpisodesByProbe {
  [probe: string]: Episode[]
}

export interface Statuses {
  [group: string]: EpisodesByProbe
}

export interface StatusRange {
  step: number
  from: number
  to: number
  incidents: any[]
  statuses: Statuses
}

export interface LegacySettings {
  groups: string[]
  groupProbes: any[]
  groupState: any
  timeRange: {
    to: number
    from: number
    step: number
    topTickFormat: string
  }
}

export class DatasetSrv {
  dataset: Dataset

  groups: string[] = []
  groupProbes: any[] = []
  groupState: any = {}

  constructor() {
    this.dataset = new Dataset()
  }

  getDataset(): Dataset {
    return this.dataset
  }

  getLegacySettings(): LegacySettings {
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
      },
    }
  }

  requestGroups() {
    /*
     * data is an array of {group:string, probe: string} objects
     */
    let me = this
    d3.json(`/api/probe`).then((probeRefs: ProbeRef[]) => {
      me.groups = []
      me.groupProbes = []
      me.groupState = {}

      for (const refs of probeRefs) {
        if (!me.groupState[refs.group]) {
          me.groupProbes.push({
            group: refs.group,
            probe: "__total__",
            type: "group",
          })
          me.groupState[refs.group] = {
            expanded: false,
            probesLoaded: false,
          }
          me.groups.push(refs.group)
        }
        me.groupProbes.push({
          group: refs.group,
          probe: refs.probe,
          type: "probe",
        })
      }

      // Fill dataset with groups and probes.
      me.dataset.clear()
      me.groups.forEach((group) => {
        me.dataset.push({
          group: group,
          label: group,
          statuses: [],
          //"timeRange": graphSettings.timeRange
        })
        me.dataset.push({
          group: group,
          probes: [],
          state: "startup",
          //"timeRange": graphSettings.timeRange
        })
      })

      renderGraphTable(me.dataset, me.getLegacySettings())

      me.groups.forEach((group) => {
        me.requestGroupData(group)
      })
    })
  }

  requestGroupData = (group: string) => {
    let me = this

    const range = getTimeRangeSrv().getFromToStep()
    const muted = getTimeRangeSrv().getMuteTypes()

    const params = new URLSearchParams({
      from: encodeURIComponent(range.from),
      to: encodeURIComponent(range.to),
      step: encodeURIComponent(range.step),
      group,
      probe: "__total__",
      muteDowntimeTypes: muted.join("!"),
    })

    const path = "/api/status/range"
    const query = `${path}?${params}`

    d3.json(query).then((d: StatusRange) => {
      // Ignore empty response
      if (!d || !d["statuses"] || !d.statuses[group]) {
        return
      }

      me.dataset.forEach((item: any, i: number) => {
        if (item.group === group && item["statuses"]) {
          me.dataset.get(i).statuses = d.statuses[group]["__total__"]
        }
      })

      updateTicks(me.dataset, me.getLegacySettings())
      renderGroupData(me.dataset, me.getLegacySettings(), group, d)

      // If group is expanded and state is not expanded, then load data for group probes.
      // If group is not xpanded and group state is expanded, then hide group probes;
      let expanded = getTimeRangeSrv().isGroupExpanded(group)
      let groupState = false
      if (me.groupState && me.groupState[group]) {
        if (me.groupState[group].hasOwnProperty("expanded")) {
          groupState = me.groupState[group]["expanded"]
        }
      }

      if (expanded !== groupState) {
        // TODO: migrate graph to React and remove this hack.
        // HACK: groups are not expanded by default, so emulate click on label to expand group.
        d3.selectAll(`[data-group-id="${group}"][data-probe-id=__total__] .graph-cell.graph-labels`).dispatch("click")
      }
    })
  }

  requestGroupProbesData(group: string) {
    let me = this

    const range = getTimeRangeSrv().getFromToStep()
    const muted = getTimeRangeSrv().getMuteTypes()

    const params = new URLSearchParams({
      from: encodeURIComponent(range.from),
      to: encodeURIComponent(range.to),
      step: encodeURIComponent(range.step),
      group,
      probe: "__all__",
      muteDowntimeTypes: muted.join("!"),
    })

    const path = "/api/status/range"
    const query = `${path}?${params}`

    d3.json(query).then((d: StatusRange) => {
      if (!d || !d.statuses || !d.statuses[group]) {
        return
      }

      me.dataset.forEach((groupData, i) => {
        if (groupData.group !== group || !groupData.probes) {
          return
        }

        for (let probe in d.statuses[group]) {
          const statuses = d.statuses[group][probe]
          me.dataset.get(i).probes.push({ probe, statuses })
        }
      })
      me.groupState[group].probesLoaded = true

      renderGroupProbesData(me.getLegacySettings(), group, d)
    })
  }
}

let instance: DatasetSrv

export function setDatasetSrv(srv: DatasetSrv) {
  instance = srv
}

export function getDatasetSrv(): DatasetSrv {
  return instance
}
