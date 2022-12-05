import { dateTime, dateTimeForTimeZone } from "@grafana/data"
import { Dataset } from "app/services/Dataset"
import { LegacySettings } from "app/services/DatasetSrv"
import { getTimeRangeSrv } from "../services/TimeRangeSrv"

export interface Tick {
  text: string
  ts?: number
  to?: number
}

let topTicks: Tick[] = []

export function calculateTopTicks(dataset: Dataset, settings: LegacySettings): Tick[] {
  topTicks = _calculateTopTicks(dataset, topTicks, settings)
  return topTicks
}

function _calculateTopTicks(dataset: Dataset, topTicks: Tick[], settings: LegacySettings): Tick[] {
  if (dataset.length() === 0) {
    return generateTicks(settings)
  }

  // timeslots in dataset
  let meta = new Map<number, { hasData: number }>()

  // iterate only over __total__ results (GroupInfo objects).
  dataset.data.forEach((item: any) => {
    const episodes = item["statuses"]
    if (!episodes || episodes.length == 0) {
      return
    }

    for (const episode of episodes) {
      // ignore total for period
      if (episode.ts === -1) {
        continue
      }

      if (!meta.has(episode.ts)) {
        meta.set(episode.ts, { hasData: 0 })
      }

      if (+episode.nodata < settings.timeRange.step) {
        meta.get(episode.ts).hasData++
      }
    }
  })

  const timestamps = Array.from(meta.keys()).sort()
  const ticks = generateTicksFromTimestamps(timestamps, settings)

  return ticks
}

// TODO calc ticks from data.
function generateTicksFromTimestamps(timestamps: number[], settings: LegacySettings): Tick[] {
  if (!timestamps || timestamps.length === 0) {
    return []
  }

  if (timestamps.length === 1) {
    const singleTick = {
      text: formatTickForTimezone(+timestamps[0], guessTickFormat(1)),
      ts: timestamps[0],
    }
    return [singleTick]
  }

  let count = timestamps.length
  let step = timestamps[1] - timestamps[0]
  let format = settings.timeRange.topTickFormat
  if (!format) {
    format = guessTickFormat(count, step)
  }

  let ticks = []

  let to = +timestamps[count - 1] + step

  let dt = dateTimeFromSeconds(to)

  // create count+1 ticks
  for (let i = count; i >= 0; i--) {
    let dtClone = dateTime(dt)
    ticks.push({
      text: dtClone.subtract(step * i, "seconds").format(format),
      ts: to - step * i,
    })
  }
  return ticks
}

function generateTicks(settings: LegacySettings) {
  const { from, to, step } = settings.timeRange
  let format = settings.timeRange.topTickFormat

  const ticks = []

  const adjustedFrom = from - (from % step)
  const adjustedTo = to - (to % step)
  const count = Math.floor((adjustedTo - adjustedFrom) / step)

  if (!format) {
    format = guessTickFormat(count, step)
  }

  const dt = dateTimeFromSeconds(adjustedTo)
  for (let i = count; i > 0; i--) {
    const dtClone = dateTime(dt)
    ticks.push({
      text: dtClone.subtract(step * i, "seconds").format(format),
      ts: adjustedTo - step * i,
    })
  }

  // display 'to' without adjust
  ticks.push({
    text: formatTickForTimezone(to, format),
    ts: to,
  })
  return ticks
}

function guessTickFormat(tickCount: number, step?: number): string {
  if (tickCount === 1) {
    return "HH:mm DD.MM" // luxon: 'HH:mm dd.MM'
  }
  if (tickCount * step >= 90 * 24 * 60 * 60) {
    return "DD.MM.YY" // luxon: 'dd.MM.yy'
  }
  if (step >= 24 * 60 * 60) {
    return "DD.MM" // luxon: 'dd.MM'
  }
  if (tickCount * step >= 12 * 60 * 60) {
    return "HH:mm DD.MM" // luxon: 'HH:mm dd.MM'
  }
  return "HH:mm" // luxon: 'HH:mm'
}

function dateTimeFromSeconds(ts: number) {
  return dateTimeForTimeZone(getTimeRangeSrv().getTimeZone(), ts, "X")
}

function formatTickForTimezone(ts: number, fmt: string) {
  return dateTimeFromSeconds(ts).format(fmt)
}
