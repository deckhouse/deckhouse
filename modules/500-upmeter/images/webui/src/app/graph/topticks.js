import {dateTime, dateTimeForTimeZone} from '@grafana/data';
import { getTimeRangeSrv } from "../services/TimeRangeSrv";

let topTicks =[];

const calculateTopTicks = function(dataset, settings) {
  topTicks = _calculateTopTicks(dataset, topTicks, settings);
}

const _calculateTopTicks = function(dataset, topTicks, settings) {
  if (dataset.length() === 0 ) {
    return generateTicks(settings)
  }

  // timeslots in dataset
  let meta = {};
  // iterate only over __total__ results (GroupInfo objects).
  dataset.data.forEach(function(item, i){
    if (item["statuses"] && item["statuses"].length > 0) {
      item["statuses"].forEach(function(info, j) {
        if (!meta[info.ts]) {
          meta[info.ts] = {
            hasData: 0
          };
        }
        if (+info.nodata < settings.timeRange.step) {
          meta[info.ts].hasData++;
        }
      })
    }
  });

  let timestamps = [];
  for (let timestamp in meta) {
    if (meta.hasOwnProperty(timestamp)) {
      timestamps.push(timestamp)
    }
  }
  timestamps.sort();

  let ticks = generateTicksFromTimestamps(timestamps, settings);

  return ticks;
}

const generateTicksOld = function(now, tickCount, between, step, format) {
  if (!format) {
    format = guessTickFormat(tickCount, step)
  }
  let ticks = [];
  if (between) {
    tickCount++
  }
  for (let i = tickCount-1; i>=0; i--) {
    ticks.push(now.clone().subtract(step*i, 'seconds').format(format))
  }
  return ticks
}

// TODO calc ticks from data.
const generateTicksFromTimestamps = function(timestamps, settings) {
  if (!timestamps || timestamps.length === 0) {
    return []
  }
  if (timestamps.length === 1) {
    return [
      formatTickForTimezone(+timestamps[0], guessTickFormat(1))
    ]
  }

  let count = timestamps.length;
  let step = timestamps[1] - timestamps[0];
  let format = settings.timeRange.topTickFormat;
  if (!format) {
    format = guessTickFormat(count, step)
  }

  let ticks = [];

  let to = +timestamps[count-1]+step;

  let dt = dateTimeFromSeconds(to);

  // create count+1 ticks
  for (let i = count; i>=0; i--) {
    let dtClone = dateTime(dt)
    ticks.push({
      text: dtClone.subtract(step*i, 'seconds').format(format),
      ts: to - step*i
    })
  }
  return ticks
}

const generateTicks = function(settings) {
  let format = settings.timeRange.topTickFormat;
  let step = settings.timeRange.step;
  let from = settings.timeRange.from;
  let to = settings.timeRange.to;

  let ticks = [];

  let adjustedFrom = Math.floor(from/step)*step;
  let adjustedTo = Math.floor(to/step)*step;
  let count = Math.floor((adjustedTo - adjustedFrom) / step);
  if (!format) {
    format = guessTickFormat(count, step)
  }
  let dt = dateTimeFromSeconds(adjustedTo);
  for (let i = count; i>0; i--) {
    let dtClone = dateTime(dt)
    ticks.push({
      text: dtClone.subtract(step*i, 'seconds').format(format),
      ts: adjustedTo - step*i
    })
  }
  // display 'to' without adjust
  ticks.push({
    text: formatTickForTimezone(to, format),
    ts: to
  });
  return ticks
}

const guessTickFormat = function(tickCount, step) {
  if (tickCount === 1) {
    return "HH:mm DD.MM" // luxon: 'HH:mm dd.MM'
  }
  if (tickCount*step >= 90 * 24 * 60 * 60) {
    return 'DD.MM.YY' // luxon: 'dd.MM.yy'
  }
  if (step >= 24 * 60 * 60) {
    return 'DD.MM' // luxon: 'dd.MM'
  }
  if (tickCount*step >= 12*60*60) {
    return 'HH:mm DD.MM' // luxon: 'HH:mm dd.MM'
  }
  return 'HH:mm' // luxon: 'HH:mm'
}

const dateTimeFromSeconds = function(ts) {
  return dateTimeForTimeZone(getTimeRangeSrv().getTimeZone(), ts, 'X')
}
const formatTickForTimezone = function(ts, fmt) {
  return dateTimeFromSeconds(ts).format(fmt)
}

export {calculateTopTicks, topTicks}
