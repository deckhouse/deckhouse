import React from "react"

import { dateTime } from "@grafana/data"
import { availabilityPercent, nanosecondsToHumanReadable, secondsToHumanReadable } from "../util/humanSeconds"
import { cx } from "emotion"
import { Episode } from "../services/DatasetSrv"

export function PieTooltip({ episode }: { episode: Episode }): JSX.Element {
  const start = dateTime(episode.start)
  const end = dateTime(episode.end)
  const duration = secondsToHumanReadable(Math.floor(end.unix() - start.unix()))
  const percent = availabilityPercent(episode.up, episode.down, episode.muted, 4)

  return (
    <>
      <p className="tooltip-head">{percent ? percent : "--"}</p>

      <ul className="tooltip-pie-time">
        <li>
          <small>Start:</small>
          <br />
          {start.format("HH:mm DD.MM.YYYY")}
        </li>
        <li>
          <small>End:</small>
          <br />
          {end.format("HH:mm DD.MM.YYYY")}
        </li>
        <li>
          <small>Duration:</small>
          <br />
          {duration}
        </li>
      </ul>

      <ul className="tooltip-pie-data">
        <TooltipSeconds className="tooltip-up" label="Up" nanoseconds={episode.up} />
        <TooltipSeconds className="tooltip-down" label="Down" nanoseconds={episode.down} />
        <TooltipSeconds className="tooltip-muted" label="Muted" nanoseconds={episode.muted} />
        <TooltipSeconds className="tooltip-unknown" label="Unknown" nanoseconds={episode.unknown} />
        <TooltipSeconds className="tooltip-nodata" label="Nodata" nanoseconds={episode.nodata} />
      </ul>

      <TooltipDowntimes downtimes={episode.downtimes} />
    </>
  )
}

interface TooltipSecondsProps {
  className: string
  label: string
  nanoseconds: number
}

function TooltipSeconds(props: React.PropsWithChildren<TooltipSecondsProps>) {
  // zero
  if (props.nanoseconds === 0) {
    return null
  }

  // not sufficient to show
  const durationString = nanosecondsToHumanReadable(props.nanoseconds)
  if (!durationString) {
    return null
  }

  return (
    <li key={props.label}>
      <small>{props.label}:</small>
      <br />
      <i className={cx("fas fa-fw fa-square", props.className)} /> {durationString}
    </li>
  )
}

function TooltipDowntimes({ downtimes }: { downtimes: any[] }) {
  if (!downtimes || downtimes.length == 0) {
    return null
  }

  const descriptions = downtimes.map((item: any) => <li key={item.DowntimeName}>{item.Description}</li>)

  return (
    <>
      <small>Downtimes:</small>
      <br />
      <ul className="tooltip-downtimes">{descriptions}</ul>
    </>
  )
}
