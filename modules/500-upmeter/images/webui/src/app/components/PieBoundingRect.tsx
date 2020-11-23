import React, { Component } from 'react';
import {css, cx} from "emotion"

import {Tooltip} from "@grafana/ui";
import {getTimeRangeSrv} from "../services/TimeRangeSrv";
import {PieTooltip} from "./PieTooltip";

export interface Props {
  width: number
  onClick: ()=>void
  data: any
  className?: string
}

export class PieBoundingRect extends Component<Props> {
  constructor(props: Props) {
    super(props);
  }

  // let boundingRect = pieRoot.append("rect")
  //   .attr("x", -pieBoxWidth/2)
  //   .attr("y", -pieBoxWidth/2)
  //   .attr("width", pieBoxWidth)
  //   .attr("height", pieBoxWidth)
  //   .style("fill", "rgba(0,0,0,0)")



  //addPieTooltip(boundingRect.node(), data)

  // let step = settings.timeRange.step;
  // if (step > 300) {
  //   boundingRect.style("cursor", "pointer");
  //   boundingRect.on("click", function (e) {
  //     // change graph to range of clicked pie (drill down)
  //     getTimeRangeSrv().drillDownStep(+data.ts)
  //   })
  // } else {
  //   boundingRect.style("cursor", "default");
  // }

  onClick = () => {
    let step = getTimeRangeSrv().step;
    if (step <= 300) {
      return
    }
    this.props.onClick();
  }

  render() {
    let step = getTimeRangeSrv().step;
    let style = css``;
    if (step > 300) {
      style = css`cursor: zoom-in`
    } else {
      style = css`cursor: default`
    }
    return (
      <Tooltip content={<PieTooltip data={this.props.data}/>} placement="right-start">
        <rect x={-this.props.width/2}
              y={-this.props.width/2}
              width={this.props.width}
              height={this.props.width}
              fill="rgba(0,0,0,0)"
              className={cx(style, this.props.className)}
              onClick={this.onClick}
      />
      </Tooltip>
  );
  }
}
