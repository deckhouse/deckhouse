/**
 * Custom version of TimeRangePicker from @grafana/ui.
 * Changes:
 * - remove CTRL+Z from zoom out tooltip
 * - use fontawesome icons
 * - hide history
 * - always show backward and forward buttons
 * - remove last 5min, 15 min, 30 min ranges from rangeOptions.
 */

// Libraries
import React, { PureComponent, memo, FormEvent } from "react"
import { css, cx } from "emotion"

// Components
import { Tooltip } from "@grafana/ui"
import { Icon } from "../Icon"
import { TimePickerContent } from "./TimeRangePicker/TimePickerContent"
import { StepIndicator } from "../StepIndicator"
import { ClickOutsideWrapper } from "@grafana/ui"

// Utils & Services
import { stylesFactory } from "@grafana/ui"
import { withTheme, useTheme } from "@grafana/ui"

// Types
import {
  //isDateTime,
  rangeUtil,
  GrafanaTheme,
  dateTimeFormat,
  timeZoneFormatUserFriendly,
} from "@grafana/data"
import { TimeRange, TimeZone, dateMath } from "@grafana/data"
import { Themeable } from "@grafana/ui"
import { otherOptions, quickOptions } from "./rangeOptions"

const getStyles = stylesFactory((theme: GrafanaTheme) => {
  return {
    container: css`
      position: relative;
      display: flex;
      flex-flow: column nowrap;
    `,
    caretIcon: css`
      margin-left: ${theme.spacing.xs};
    `,
    clockIcon: css`
      margin-left: ${theme.spacing.xs};
      margin-right: ${theme.spacing.xs};
    `,
    noRightBorderStyle: css`
      label: noRightBorderStyle;
      border-right: 0;
    `,
  }
})

const getLabelStyles = stylesFactory((theme: GrafanaTheme) => {
  return {
    container: css`
      display: inline-block;
    `,
    utc: css`
      color: ${theme.palette.orange};
      font-size: 75%;
      padding: 3px;
      font-weight: ${theme.typography.weight.semibold};
    `,
  }
})

export interface Props extends Themeable {
  hideText?: boolean
  value: TimeRange
  timeZone?: TimeZone
  timeSyncButton?: JSX.Element
  isSynced?: boolean
  onChange: (timeRange: TimeRange) => void
  onChangeTimeZone: (timeZone: TimeZone) => void
  onMoveBackward: () => void
  onMoveForward: () => void
  onZoom: () => void
  history?: TimeRange[]
  hideZoomButton?: boolean
  step: string
}

export interface State {
  isOpen: boolean
}

export class UnthemedTimeRangePicker extends PureComponent<Props, State> {
  state: State = {
    isOpen: false,
  }

  onChange = (timeRange: TimeRange) => {
    this.props.onChange(timeRange)
    this.setState({ isOpen: false })
  }

  onOpen = (event: FormEvent<HTMLButtonElement>) => {
    const { isOpen } = this.state
    event.stopPropagation()
    event.preventDefault()
    this.setState({ isOpen: !isOpen })
  }

  onClose = () => {
    this.setState({ isOpen: false })
  }

  render() {
    const {
      value,
      onMoveBackward,
      onMoveForward,
      onZoom,
      timeZone,
      timeSyncButton,
      isSynced,
      theme,
      history,
      onChangeTimeZone,
      hideZoomButton,
      step,
    } = this.props

    const { isOpen } = this.state
    const styles = getStyles(theme)
    //const hasAbsolute = isDateTime(value.raw.from) || isDateTime(value.raw.to);
    const syncedTimePicker = timeSyncButton && isSynced
    //const timePickerIconClass = cx({ ['icon-brand-gradient']: syncedTimePicker });
    const timePickerButtonClass = cx("btn btn-outline-primary btn-sm", {
      [`btn--radius-right-0 ${styles.noRightBorderStyle}`]: !!timeSyncButton,
      [`explore-active-button`]: syncedTimePicker,
    })

    return (
      <div className={styles.container}>
        <div className="btn-group">
          <button className="btn btn-outline-primary btn-sm" onClick={onMoveBackward}>
            <Icon name="fa-step-backward" size="lg" />
          </button>

          <Tooltip content={<TimePickerTooltip timeRange={value} timeZone={timeZone} />} placement="bottom">
            <button aria-label="TimePicker Open Button" className={timePickerButtonClass} onClick={this.onOpen}>
              <TimePickerButtonLabel {...this.props} />
              <span className={styles.caretIcon}>
                {<Icon name={isOpen ? "fa-caret-up" : "fa-caret-down"} size="lg" />}
              </span>
            </button>
          </Tooltip>
          {isOpen && (
            <ClickOutsideWrapper includeButtonPress={false} onClick={this.onClose}>
              <TimePickerContent
                timeZone={timeZone}
                value={value}
                onChange={this.onChange}
                otherOptions={otherOptions}
                quickOptions={quickOptions}
                history={history}
                showHistory={false}
                onChangeTimeZone={onChangeTimeZone}
                hideTimeZone={true}
              />
            </ClickOutsideWrapper>
          )}

          {timeSyncButton}

          <button className="btn btn-outline-primary btn-sm" onClick={onMoveForward}>
            <Icon name="fa-step-forward" size="lg" />
          </button>

          <StepIndicator label="Step" text={step} />

          {!hideZoomButton && (
            <Tooltip content={ZoomOutTooltip} placement="bottom">
              <button className="btn btn-outline-primary btn-sm" onClick={onZoom}>
                <Icon name="fa-search-minus" size="lg" />
              </button>
            </Tooltip>
          )}
        </div>
      </div>
    )
  }
}

const ZoomOutTooltip = () => <>Time range zoom out</>

const TimePickerTooltip = ({ timeRange, timeZone }: { timeRange: TimeRange; timeZone?: TimeZone }) => {
  const theme = useTheme()
  const styles = getLabelStyles(theme)

  return (
    <>
      {dateTimeFormat(timeRange.from, { timeZone })}
      <div className="text-center">to</div>
      {dateTimeFormat(timeRange.to, { timeZone })}
      <div className="text-center">
        <span className={styles.utc}>{timeZoneFormatUserFriendly(timeZone)}</span>
      </div>
    </>
  )
}

type LabelProps = Pick<Props, "hideText" | "value" | "timeZone">

export const TimePickerButtonLabel = memo<LabelProps>(({ hideText, value, timeZone }) => {
  const theme = useTheme()
  const styles = getLabelStyles(theme)

  if (hideText) {
    return null
  }

  return (
    <span className={styles.container}>
      <span>{formattedRange(value, timeZone)}</span>
      <span className={styles.utc}>{rangeUtil.describeTimeRangeAbbreviation(value, timeZone)}</span>
    </span>
  )
})

const formattedRange = (value: TimeRange, timeZone?: TimeZone) => {
  const adjustedTimeRange = {
    to: dateMath.isMathString(value.raw.to) ? value.raw.to : value.to,
    from: dateMath.isMathString(value.raw.from) ? value.raw.from : value.from,
  }
  return rangeUtil.describeTimeRange(adjustedTimeRange, timeZone)
}

export const TimeRangePicker = withTheme(UnthemedTimeRangePicker)
