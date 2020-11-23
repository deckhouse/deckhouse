import React from 'react';

import {dateTime} from '@grafana/data';

import {secondsToHumanReadable, availabilityPercent} from "../util/humanSeconds";
import {cx} from "emotion";

export const PieTooltip = ({ data }: { data: any }) => {
  let d = data;

  let from = dateTime(d.start)
  let to = dateTime(d.end)
  let duration = secondsToHumanReadable(Math.floor(to.unix() - from.unix()))
  let percent = availabilityPercent(d.up, d.down, d.muted, 4)

  return (
  <>
    <p className="tooltip-head">
      {percent ? percent : '--'}
    </p>

    <ul className="tooltip-pie-time">
      <li>
        <small>Start:</small><br/>
        {from.format("HH:mm DD.MM.YYYY")}
      </li>
      <li>
        <small>End:</small><br/>
        {to.format("HH:mm DD.MM.YYYY")}
      </li>
      <li>
        <small>Duration:</small><br/>
        {duration}
      </li>
    </ul>

    <ul className="tooltip-pie-data">
      <TooltipSeconds
        className="tooltip-up"
        label="Up"
        seconds={+d.up}/>
      <TooltipSeconds
        className="tooltip-down"
        label="Down"
        seconds={+d.down}/>
      <TooltipSeconds
        className="tooltip-muted"
        label="Muted"
        seconds={+d.muted}/>
      <TooltipSeconds
        className="tooltip-unknown"
        label="Unknown"
        seconds={+d.unknown}/>
      <TooltipSeconds
        className="tooltip-nodata"
        label="Nodata"
        seconds={+d.nodata}/>
    </ul>

    <TooltipDowntimes
      downtimes={d.downtimes}
    />
  </>
  );
};

interface TooltipSecondsProps {
  className: string
  label: string
  seconds: number
}
export const TooltipSeconds = (props: React.PropsWithChildren<TooltipSecondsProps>) => {
  if (!props.seconds) {
    return null;
  }
  return (
    <li>
      <small>{props.label}:</small><br/>
      <i className={cx("fas fa-fw fa-square", props.className)}></i>
      {secondsToHumanReadable(+props.seconds)}
    </li>
  );
}

export const TooltipDowntimes = ({downtimes} : {downtimes: any[]}) => {
  if (!downtimes || downtimes.length == 0) {
    return null;
  }

  return (
    <>
      <small>
        Downtimes:
      </small><br/>
      <ul className="tooltip-downtimes">
        {downtimes.map(function(item:any){
          return <li key={item.DowntimeName}>{item.Description}</li>
        })}
      </ul>
    </>
  )
}
