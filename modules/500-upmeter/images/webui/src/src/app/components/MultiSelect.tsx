import React, { Component, FormEvent } from "react"
import { cx } from "emotion"

// Components
import { Tooltip, ClickOutsideWrapper } from "@grafana/ui"
import { Icon } from "./Icon"
import { MuteTypeTooltip } from "./MuteTypeTooltip"

// Services
import { i18n, MuteItems } from "../i18n"

export interface Props {
  label: string
  selection: Map<keyof MuteItems, boolean>
  items: SelectItemState[]
  onClickItem: (id: keyof MuteItems) => void
  onClose: () => void
}

export interface State {
  isOpen: boolean
}

export class MultiSelect extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = {
      isOpen: false,
    }
  }

  onOpen = (event: FormEvent<HTMLButtonElement>) => {
    const isOpen = this.state.isOpen
    event.stopPropagation()
    this.setState({ isOpen: !isOpen })
  }

  onClose = () => {
    this.setState({ isOpen: false })
    this.props.onClose()
  }

  render() {
    const isOpen = this.state.isOpen
    //const styles = getStyles()
    const tooltip = i18n().mute.menu.tooltip

    return (
      <div className="multi-select-container">
        <Tooltip content={tooltip} placement="bottom">
          <button className="btn btn-outline-primary btn-sm" onClick={this.onOpen}>
            <Icon name="fa-filter" />
            <span className="multi-select-item-label">{this.props.label}</span>
            <Icon name={isOpen ? "fa-caret-up" : "fa-caret-down"} />
          </button>
        </Tooltip>
        {isOpen && (
          <ClickOutsideWrapper onClick={this.onClose}>
            <div className={cx("multi-select-content-container", "btn-group-vertical")}>
              {this.props.items.map((item: SelectItemState) => (
                <SelectItem
                  key={item.id}
                  onClick={this.props.onClickItem}
                  selected={this.props.selection.get(item.id)}
                  id={item.id}
                  label={item.label}
                />
              ))}
            </div>
          </ClickOutsideWrapper>
        )}
      </div>
    )
  }
}

export interface SelectItemState {
  id: keyof MuteItems
  label: string
  tooltip: string
}

export interface SelectItemProps {
  id: keyof MuteItems
  label: string
  selected: boolean
  onClick: (id: keyof MuteItems) => void
}

export class SelectItem extends Component<SelectItemProps> {
  constructor(props: SelectItemProps) {
    super(props)
  }

  render() {
    return (
      <button
        className="btn btn-secondary text-left"
        onClick={() => {
          this.props.onClick(this.props.id)
        }}
      >
        <Icon name={this.props.selected ? "fa-check-square" : "fa-square"} />
        <span className="multi-select-item-label">{this.props.label}</span>

        <Tooltip content={<MuteTypeTooltip muteTypeId={this.props.id} />} placement="bottom">
          <Icon name="fa-info-circle" className="multi-select-info" />
        </Tooltip>
      </button>
    )
  }
}
