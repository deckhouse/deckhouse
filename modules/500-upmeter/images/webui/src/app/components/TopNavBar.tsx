import React, { Component } from "react"
import { TimeRange, TimeZone } from "@grafana/data"

// Components
import { TimeRangePicker } from "./TimePicker/TimeRangePicker"
import { MultiSelect, SelectItemState } from "./MultiSelect"

// Services
import { i18n, MuteItems } from "../i18n"
import { getTimeRangeSrv } from "../services/TimeRangeSrv"
import { getEventsSrv } from "../services/EventsSrv"

import logo from "../../assets/upmeter.png"

export interface Props {
  muteSelection: Map<keyof MuteItems, boolean>
}

export interface State {
  muteItems: SelectItemState[]
  muteSelection: Map<keyof MuteItems, boolean>
}

export class TopNavBar extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = {
      muteItems: [],
      muteSelection: new Map<keyof MuteItems, boolean>(),
    }

    // Load mute types from langPack.
    i18n().mute.order.map((id: keyof MuteItems) => {
      let item = i18n().mute.items[id]
      let muteItem: SelectItemState = {
        id: id,
        label: item.label,
        tooltip: item.tooltip,
      }
      this.state.muteItems.push(muteItem)
    })

    // Copy selection to state.
    for (let k of this.props.muteSelection.keys()) {
      this.state.muteSelection.set(k, this.props.muteSelection.get(k))
    }
  }

  onClickMuteType = (id: keyof MuteItems) => {
    getTimeRangeSrv().toggleMuteType(id)
    this.setState({ muteSelection: getTimeRangeSrv().getMuteSelection() })
  }

  onCloseMuteSelect = () => {
    // Update mute types, save state and render graph.
    getTimeRangeSrv().saveMuteSelection()
  }

  onChangeTimeRangePicker = (range: TimeRange) => {
    getTimeRangeSrv().updateFromTimeRange(range)
  }

  onChangeTimeZone = (timeZone: TimeZone) => {
    getTimeRangeSrv().updateTimezone(timeZone)
    this.forceUpdate()
  }

  onMoveBack = () => {
    getTimeRangeSrv().rangeMoveBack()
  }

  onMoveForward = () => {
    getTimeRangeSrv().rangeMoveForward()
  }

  onZoom = () => {
    getTimeRangeSrv().rangeZoomOut()
  }

  onRefresh = () => {
    let sel = getTimeRangeSrv().getMuteSelection()
    for (let k of sel.keys()) {
      this.state.muteSelection.set(k, sel.get(k))
    }
    this.forceUpdate()
  }

  componentDidMount() {
    // TODO Update mute selection!
    getEventsSrv().listenEvent("Refresh", "topNav", this.onRefresh)
  }

  componentWillUnmount() {
    getEventsSrv().unlistenEvent("Refresh", "topNav")
  }

  render() {
    let timeRange = getTimeRangeSrv().getTimeRange()
    let timeZone = getTimeRangeSrv().getTimeZone()
    let step = getTimeRangeSrv().getHumanStep()

    let hideZoomButton = getTimeRangeSrv().getHideZoomButton()

    return (
      <div className="top-navbar-container" role="toolbar" aria-label="Toolbar with button groups">
        <a href="./">
          <img src={logo} alt="Upmeter" height="50" />
        </a>
        <TimeRangePicker
          value={timeRange}
          onChange={this.onChangeTimeRangePicker}
          timeZone={timeZone}
          onChangeTimeZone={this.onChangeTimeZone}
          onMoveBackward={this.onMoveBack}
          onMoveForward={this.onMoveForward}
          onZoom={this.onZoom}
          hideZoomButton={hideZoomButton}
          step={step}
        />
        <MultiSelect
          label={i18n().mute.menu.label}
          items={this.state.muteItems}
          selection={this.state.muteSelection}
          onClickItem={this.onClickMuteType}
          onClose={this.onCloseMuteSelect}
        />
      </div>
    )
  }
}
