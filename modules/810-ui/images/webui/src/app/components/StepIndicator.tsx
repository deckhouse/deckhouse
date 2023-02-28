import React, { Component } from "react"

export interface Props {
  label: string
  text: string
}

export class StepIndicator extends Component<Props> {
  constructor(props: Props) {
    super(props)
  }

  render() {
    return (
      <button className="btn btn-outline-primary btn-sm" disabled>
        {this.props.label}: <span className="badge badge-primary">{this.props.text}</span>
      </button>
    )
  }
}
