import * as d3 from "d3"

export function nanosecondsToHumanReadable(nanoseconds: number): string {
  return secondsToHumanReadable(Math.round(nanoseconds / 1e9))
}

export function secondsToHumanReadable(seconds: number): string {
  if (seconds === 0) {
    return ""
  }

  if (seconds < 60) {
    return `${seconds}s`
  }

  if (seconds < 60 * 60) {
    return `${Math.floor(seconds / 60)}m ${seconds % 60}s`.replace(/ 0\w+/g, "")
  }

  let hourSec = 60 * 60
  let daySec = 24 * hourSec
  if (seconds < daySec) {
    let hour = Math.floor(seconds / hourSec)
    let min = Math.floor((seconds % hourSec) / 60)
    let sec = Math.floor((seconds % hourSec) % 60)
    return `${hour}h ${min}m ${sec}s`.replace(/ 0\w+/g, "")
  }

  let monthSec = 30 * daySec
  if (seconds < monthSec) {
    let day = Math.floor(seconds / daySec)
    let remSec = seconds % daySec
    let hour = Math.floor(remSec / hourSec)
    let min = Math.floor((remSec % hourSec) / 60)
    let sec = Math.floor((remSec % hourSec) % 60)
    return `${day}day ${hour}h ${min}m ${sec}s`.replace(/ 0\w+/g, "")
  }

  let yearSec = 365 * daySec
  if (seconds < yearSec) {
    let month = Math.floor(seconds / monthSec)
    let day = Math.floor((seconds % monthSec) / daySec)
    let remSec = (seconds % monthSec) % daySec
    let hour = Math.floor(remSec / hourSec)
    let min = Math.floor((remSec % hourSec) / 60)
    let sec = Math.floor((remSec % hourSec) % 60)
    return `${month}mon ${day}day ${hour}h ${min}m ${sec}s`.replace(/ 0\w+/g, "")
  }

  let year = Math.floor(seconds / yearSec)
  let month = Math.floor((seconds % yearSec) / monthSec)
  let day = Math.floor(((seconds % yearSec) % monthSec) / daySec)
  let remSec = ((seconds % yearSec) % monthSec) % daySec
  let hour = Math.floor(remSec / hourSec)
  let min = Math.floor((remSec % hourSec) / 60)
  let sec = Math.floor((remSec % hourSec) % 60)
  return `${year}y ${month}mon ${day}day ${hour}h ${min}m ${sec}s`.replace(/ 0\w+/g, "")
}

export function getStepForRangeId(rangeId: string): number {
  let step = 300
  if (rangeId === "3hr") {
    step = 20 * 60 // 20 minutes
  } else if (rangeId === "day") {
    step = 2 * 60 * 60 // 2 hours
  } else if (rangeId === "week") {
    step = 24 * 60 * 60 // 1 day
  } else if (rangeId === "month") {
    step = 3 * 24 * 60 * 60 // 3 days
  } else if (rangeId === "quarter") {
    step = 7 * 24 * 60 * 60 // 7 days
  } else if (rangeId === "year") {
    step = 30 * 24 * 60 * 60 // 30 days
  }
  return step
}

export const availabilityPercent = function (up: number, down: any, muted: any, precision: number) {
  if (+up + down + muted === 0) {
    return ""
  }
  if (+up + muted === 0 && +down > 0) {
    return "0%"
  }
  if (+down === 0) {
    return "100%"
  }
  let pieValue = (+up + muted) / (+up + muted + down) ///settings.step;
  return d3.format(`.${precision}%`)(pieValue)
}
