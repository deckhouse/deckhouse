import React, { Component } from "react"
import { css, cx } from "emotion"

import { Tooltip } from "@grafana/ui"
import { getTimeRangeSrv } from "../services/TimeRangeSrv"
import { PieTooltip } from "./PieTooltip"
import { Episode } from "../services/DatasetSrv"

export interface Props {
  size: number
  onClick: () => void
  episode: Episode
  className?: string
}

export class PieBoundingRect extends Component<Props, any> {
  constructor(props: Props) {
    super(props)
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
    let step = getTimeRangeSrv().step
    if (step <= 300) {
      return
    }
    this.props.onClick()
  }

  render() {
    let step = getTimeRangeSrv().step
    let style = css``
    if (step > 300) {
      style = css`
        cursor: zoom-in;
      `
    } else {
      style = css`
        cursor: default;
      `
    }

    const tooltip = <PieTooltip episode={this.props.episode} />
    const { size, className } = this.props

    return (
      <Tooltip content={tooltip} placement="right-start">
        <rect
          x={-size / 2}
          y={-size / 2}
          width={size}
          height={size}
          fill="rgba(0,0,0,0)"
          className={cx(style, className)}
          onClick={this.onClick}
        />
      </Tooltip>
    )
  }
}
