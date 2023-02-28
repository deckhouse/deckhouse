/**
 * TimeRangeSrv is a holder of graph settings:
 * - from
 * - to
 * - step
 * - mute - an array of muted types (Accident, Maintenance, Infrastructure Accident, Infrastructure Maintenance)
 * - fmt - user defined tick format
 * This service updates these values from address bar on startup and from components in runtime.
 */
import { TimeZone, TimeRange, dateMath, dateTime, DateTime } from "@grafana/data"

import { isNumber, isString } from "../util/type"
import { secondsToHumanReadable } from "../util/humanSeconds"

// Services
import { getSettingsStore, Settings, SettingsStore } from "./SettingsStore"
import { getEventsSrv } from "./EventsSrv"
import { MuteItems } from "../i18n"
import { RangeStepsLadder } from "../util/rangeSteps"

export class TimeRangeSrv {
  public static defaultTimeRange = {
    from: dateTime(),
    to: dateTime(),
    raw: { from: "now - 6h", to: "now" },
  }

  from: number // unix timestamp in UTC
  to: number // unix timestamp in UTC
  step: number // a multiple of 300, minimum is 300;
  count: number // calculated
  muteSelection: Map<keyof MuteItems, boolean> // state of a mute types menu
  expandedGroups: Map<string, boolean> // state of expanded groups
  timeRange: TimeRange // state of a TimeRange picker
  timezone: TimeZone // timezone from a TimeRange picker
  settings: Settings // state from hash from location
  // hideZoomButton: boolean; // hide zoom button on last allowed step

  constructor() {
    this.muteSelection = new Map<keyof MuteItems, boolean>([
      ["Acd", false],
      ["Mnt", true],
      ["InfAcd", true],
      ["InfMnt", true],
    ])
    this.expandedGroups = new Map<string, boolean>()
    this.timeRange = newTimeRange("now - 6h", "now")
    this.timezone = "browser"
  }

  init() {
    let settings = getSettingsStore().load()
    this.loadFromObj(settings)
  }

  loadFromObj = (obj: Settings) => {
    let nowUnix = Math.floor(dateTime().unix())
    let from = 0,
      to = 0,
      step = 0,
      count = 12

    if (isNumber(obj.to) && !isNaN(obj.to)) {
      to = obj.to
    }
    if (isString(obj.to) && obj.to === "now") {
      to = nowUnix
    }

    if (!isNaN(obj.from)) {
      from = obj.from
    }

    if (!isNaN(obj.step)) {
      step = obj.step
    } else {
      // reasonable default
      step = 300
    }

    if (from > 0 && to > 0 && step > 0) {
      count = Math.floor((to - from) / step)
      // load timeRange
      // Create new timeRange to update memoized label in TimeRangePicker.
      this.timeRange = newTimeRange(from, to)
    } else {
      if (from == 0 && to == 0) {
        // now - 6h / now
        this.timeRange = newTimeRange("now - 6h", "now")
        from = this.timeRange.from.unix()
        to = this.timeRange.to.unix()
        step = Math.floor((to - from) / count / 300) * 300
      }
      if (from == 0 && to > 0 && step > 0) {
        from = to - count * step
        this.timeRange = newTimeRange(from, to)
      } else {
        //let error = "from, to and step are required params"
        //getEventsSrv().fireEvent("error");
      }
    }
    this.from = from
    this.to = to
    this.step = step
    this.count = count

    this.settings = {
      from: obj.from,
      to: obj.to,
      step: obj.step,
      mute: new Map<keyof MuteItems, boolean>(),
      fmt: obj.fmt,
      expand: new Map<string, boolean>(),
    }

    //console.log("Got obj", obj);
    for (let k of this.muteSelection.keys()) {
      if (!!obj.mute) {
        this.muteSelection.set(k, obj.mute.get(k))
        this.settings.mute.set(k, obj.mute.get(k))
      } else {
        this.muteSelection.set(k, SettingsStore.defaultMuteFlags.get(k))
        this.settings.mute.set(k, SettingsStore.defaultMuteFlags.get(k))
      }
    }

    // Reset expandedGroups and set them from obj.expand.
    for (let group of this.expandedGroups.keys()) {
      this.expandedGroups.set(group, false)
    }
    if (obj.expand) {
      for (let group of obj.expand.keys()) {
        this.expandedGroups.set(group, true)
      }
    }
  }

  // raw.from and raw.to can be a string. from and to are always DateTime moment objects.
  updateFromTimeRange = (range: TimeRange) => {
    let newFrom = range.from.unix()
    let newTo = range.to.unix()

    if (newFrom == this.from && newTo == this.to) {
      return
    }

    // const adjustedFrom = dateMath.isMathString(timeRange.raw.from) ? timeRange.raw.from : timeRange.from;
    // const adjustedTo = dateMath.isMathString(timeRange.raw.to) ? timeRange.raw.to : timeRange.to;
    // const nextRange = {
    //   from: adjustedFrom,
    //   to: adjustedTo,
    // };
    this.timeRange = newTimeRange(range.from, range.to)

    // Get unix timestamps from range.
    this.from = newFrom
    this.to = newTo
    this.step = Math.floor((this.to - this.from) / this.count / 300) * 300
    // TODO also adjust count
    if (this.step == 0) {
      this.step = 300
    }

    this.save()
    getEventsSrv().fireEvent("Refresh")
    getEventsSrv().fireEvent("UpdateGraph", { msg: "" })
  }

  updateToNow = () => {
    let to = Math.floor(dateTime().unix())
    if (this.to === to) {
      return
    }
    this.from = Math.floor(to - (this.to - this.from))
    this.to = to
    this.timeRange = newTimeRange(this.from, this.to)
    this.save()
    getEventsSrv().fireEvent("Refresh")
    getEventsSrv().fireEvent("UpdateGraph", { msg: "" })
  }

  drillDownStep = (timestamp: number) => {
    if (this.step <= 300) {
      return
    }

    this.from = timestamp
    this.to = timestamp + this.step

    this.step = Math.floor(this.step / this.count / 300) * 300
    if (this.step < 300) {
      this.step = 300
    }

    this.timeRange = newTimeRange(this.from, this.to)
    this.save()
    getEventsSrv().fireEvent("Refresh")
    getEventsSrv().fireEvent("UpdateGraph", { msg: "" })
  }

  rangeMoveBack = () => {
    this.from = this.from - this.step * 5
    this.to = this.to - this.step * 5

    this.timeRange = newTimeRange(this.from, this.to)
    this.save()
    getEventsSrv().fireEvent("Refresh")
    getEventsSrv().fireEvent("UpdateGraph", { msg: "" })
  }

  rangeMoveForward = () => {
    this.from = this.from + this.step * 5
    this.to = this.to + this.step * 5
    this.timeRange = newTimeRange(this.from, this.to)
    this.save()
    getEventsSrv().fireEvent("Refresh")
    getEventsSrv().fireEvent("UpdateGraph", { msg: "" })
  }

  rangeZoomOut = () => {
    // choose next step from ladder;
    let newStep = 0
    for (let stepInfo of RangeStepsLadder) {
      newStep = stepInfo.step
      if (this.step < stepInfo.step) {
        break
      }
    }
    this.step = newStep

    this.from = this.to - this.count * this.step
    this.timeRange = newTimeRange(this.from, this.to)
    this.save()
    getEventsSrv().fireEvent("Refresh")
    getEventsSrv().fireEvent("UpdateGraph", { msg: "" })
  }

  toggleMuteType = (id: keyof MuteItems) => {
    let sel = this.muteSelection.get(id)
    this.muteSelection.set(id, !sel)
  }

  // TODO use muteSelection for save, do not update settings here.
  saveMuteSelection = () => {
    // Copy from state to settings to save.
    let changed = false
    for (let k of this.muteSelection.keys()) {
      if (this.muteSelection.get(k) !== this.settings.mute.get(k)) {
        changed = true
      }
      this.settings.mute.set(k, this.muteSelection.get(k))
    }

    if (changed) {
      this.save()
      getEventsSrv().fireEvent("UpdateGraph", { msg: "" })
    }
  }

  save() {
    getSettingsStore().save({
      from: this.from,
      to: this.to,
      step: this.step,
      mute: this.settings.mute,
      fmt: this.settings.fmt,
      expand: this.expandedGroups,
    })
  }

  getMuteSelection = (): Map<keyof MuteItems, boolean> => {
    return this.muteSelection
  }

  // property for TimeRangePicker
  getTimeRange = (): TimeRange => {
    return this.timeRange
  }

  getTimeZone = (): TimeZone => {
    return this.timezone
  }

  getHideZoomButton = (): boolean => {
    let lastStep = RangeStepsLadder[RangeStepsLadder.length - 1].step
    return this.step >= lastStep
  }

  updateTimezone = (timeZone: TimeZone) => {
    console.log("new TZ:", timeZone)
    this.timezone = timeZone
    //graphSettings.setTimeZone(timeZone);
    //this.forceUpdate();
    //this.onRefresh();
  }

  getHumanStep = (): string => {
    return secondsToHumanReadable(this.step)
  }

  getFromToStep(): { from: number; to: number; step: number } {
    return {
      from: this.from,
      to: this.to,
      step: this.step,
    }
  }

  getMuteTypes(): string[] {
    return Array.from(this.muteSelection.keys()).filter((k) => this.muteSelection.get(k))
  }

  isGroupExpanded = (group: string): boolean => {
    if (this.expandedGroups.has(group)) {
      return this.expandedGroups.get(group)
    }
    return false
  }

  onExpandGroup = (group: string, state: boolean) => {
    let shouldSave: boolean
    if (this.expandedGroups.has(group)) {
      shouldSave = this.expandedGroups.get(group) !== state
    } else {
      shouldSave = true
    }

    this.expandedGroups.set(group, state)

    if (shouldSave) {
      this.save()
    }
  }
}

function newTimeRange(from: number | string | DateTime, to: number | string | DateTime): TimeRange {
  let tr: TimeRange = {
    from: undefined,
    to: undefined,
    raw: {
      from: undefined,
      to: undefined,
    },
  }

  if (isNumber(from)) {
    tr.from = dateTime(from * 1000)
    tr.raw.from = tr.from
  } else if (isString(from) && dateMath.isMathString(from)) {
    tr.from = dateMath.parse(from)
    tr.raw.from = from
  } else {
    tr.from = dateTime(from)
    tr.raw.from = from
  }
  if (isNumber(to)) {
    tr.to = dateTime(to * 1000)
    tr.raw.to = tr.to
  } else if (isString(to) && dateMath.isMathString(to)) {
    tr.to = dateMath.parse(to)
    tr.raw.to = to
  } else {
    tr.to = dateTime(to)
    tr.raw.to = to
  }

  return tr
}

let instance: TimeRangeSrv

export function setTimeRangeSrv(srv: TimeRangeSrv) {
  instance = srv
}

export function getTimeRangeSrv(): TimeRangeSrv {
  return instance
}
