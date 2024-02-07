import React, { FormEvent, memo, useCallback, useEffect, useState } from 'react';
import { css } from 'emotion';
import Calendar from 'react-calendar/dist/entry.nostyle';
import { dateTime, DateTime, dateTimeParse, GrafanaTheme, TimeZone } from '@grafana/data';
import { stylesFactory, useTheme } from '../../../themes';
import { TimePickerTitle } from './TimePickerTitle';
import { Button } from '../../Button';
import { Icon } from '../../Icon/Icon';
import { Portal } from '../../Portal/Portal';
import { ClickOutsideWrapper } from '../../ClickOutsideWrapper/ClickOutsideWrapper';

const getStyles = stylesFactory((theme: GrafanaTheme, isReversed = false) => {
  const containerBorder = theme.isDark ? theme.palette.dark9 : theme.palette.gray5;

  return {
    container: css`
      top: -1px;
      position: absolute;
      ${isReversed ? 'left' : 'right'}: 544px;
      box-shadow: ${isReversed ? '10px' : '0px'} 0px 20px ${theme.colors.dropdownShadow};
      background-color: ${theme.colors.bodyBg};
      z-index: -1;
      border: 1px solid ${containerBorder};
      border-radius: 2px 0 0 2px;

      &:after {
        display: block;
        background-color: ${theme.colors.bodyBg};
        width: 19px;
        height: 100%;
        content: ${!isReversed ? ' ' : ''};
        position: absolute;
        top: 0;
        right: -19px;
        border-left: 1px solid ${theme.colors.border1};
      }
    `,
    modal: css`
      position: fixed;
      top: 20%;
      width: 100%;
      z-index: ${theme.zIndex.modal};
    `,
    content: css`
      margin: 0 auto;
      width: 268px;
    `,
    backdrop: css`
      position: fixed;
      top: 0;
      right: 0;
      bottom: 0;
      left: 0;
      background: #202226;
      opacity: 0.7;
      z-index: ${theme.zIndex.modalBackdrop};
      text-align: center;
    `,
  };
});

const getFooterStyles = stylesFactory((theme: GrafanaTheme) => {
  return {
    container: css`
      background-color: ${theme.colors.bodyBg};
      display: flex;
      justify-content: center;
      padding: 10px;
      align-items: stretch;
    `,
    apply: css`
      margin-right: 4px;
      width: 100%;
      justify-content: center;
    `,
  };
});

const getBodyStyles = stylesFactory((theme: GrafanaTheme) => {
  return {
    title: css`
      color: ${theme.colors.text};
      background-color: ${theme.colors.bodyBg};
      font-size: ${theme.typography.size.md};
      border: 1px solid transparent;

      &:hover {
        position: relative;
      }
    `,
    body: css`
      z-index: ${theme.zIndex.modal};
      background-color: ${theme.colors.bodyBg};
      width: 268px;

      .react-calendar__navigation__label,
      .react-calendar__navigation__arrow,
      .react-calendar__navigation {
        padding-top: 4px;
        background-color: inherit;
        color: ${theme.colors.text};
        border: 0;
        font-weight: ${theme.typography.weight.semibold};
      }

      .react-calendar__month-view__weekdays {
        background-color: inherit;
        text-align: center;
        color: ${theme.palette.blue77};

        abbr {
          border: 0;
          text-decoration: none;
          cursor: default;
          display: block;
          padding: 4px 0 4px 0;
        }
      }

      .react-calendar__month-view__days {
        background-color: inherit;
      }

      .react-calendar__tile,
      .react-calendar__tile--now {
        margin-bottom: 4px;
        background-color: inherit;
        height: 26px;
      }

      .react-calendar__navigation__label,
      .react-calendar__navigation > button:focus,
      .time-picker-calendar-tile:focus {
        outline: 0;
      }

      .react-calendar__tile--active,
      .react-calendar__tile--active:hover {
        color: ${theme.palette.white};
        font-weight: ${theme.typography.weight.semibold};
        background: ${theme.palette.blue95};
        box-shadow: none;
        border: 0px;
      }

      .react-calendar__tile--rangeEnd,
      .react-calendar__tile--rangeStart {
        padding: 0;
        border: 0px;
        color: ${theme.palette.white};
        font-weight: ${theme.typography.weight.semibold};
        background: ${theme.palette.blue95};

        abbr {
          background-color: ${theme.palette.blue77};
          border-radius: 100px;
          display: block;
          padding-top: 2px;
          height: 26px;
        }
      }

      .react-calendar__tile--rangeStart {
        border-top-left-radius: 20px;
        border-bottom-left-radius: 20px;
      }

      .react-calendar__tile--rangeEnd {
        border-top-right-radius: 20px;
        border-bottom-right-radius: 20px;
      }
    `,
  };
});

const getHeaderStyles = stylesFactory((theme: GrafanaTheme) => {
  return {
    container: css`
      background-color: ${theme.colors.bodyBg};
      display: flex;
      justify-content: space-between;
      padding: 7px;
    `,
  };
});

interface Props {
  isOpen: boolean;
  from: DateTime;
  to: DateTime;
  onClose: () => void;
  onApply: (e: FormEvent<HTMLButtonElement>) => void;
  onChange: (from: DateTime, to: DateTime) => void;
  isFullscreen: boolean;
  timeZone?: TimeZone;
  isReversed?: boolean;
}

const stopPropagation = (event: React.MouseEvent<HTMLDivElement>) => event.stopPropagation();

export const TimePickerCalendar = memo<Props>(props => {
  const theme = useTheme();
  const styles = getStyles(theme, props.isReversed);
  const { isOpen, isFullscreen } = props;

  if (!isOpen) {
    return null;
  }

  if (isFullscreen) {
    return (
      <ClickOutsideWrapper onClick={props.onClose}>
        <div className={styles.container} onClick={stopPropagation}>
          <Body {...props} />
        </div>
      </ClickOutsideWrapper>
    );
  }

  return (
    <Portal>
      <div className={styles.modal} onClick={stopPropagation}>
        <div className={styles.content}>
          <Header {...props} />
          <Body {...props} />
          <Footer {...props} />
        </div>
      </div>
      <div className={styles.backdrop} onClick={stopPropagation} />
    </Portal>
  );
});

const Header = memo<Props>(({ onClose }) => {
  const theme = useTheme();
  const styles = getHeaderStyles(theme);

  return (
    <div className={styles.container}>
      <TimePickerTitle>Select a time range</TimePickerTitle>
      <Icon name="times" onClick={onClose} />
    </div>
  );
});

const Body = memo<Props>(({ onChange, from, to, timeZone }) => {
  const [value, setValue] = useState<Date[]>();
  const theme = useTheme();
  const onCalendarChange = useOnCalendarChange(onChange, timeZone);
  const styles = getBodyStyles(theme);

  useEffect(() => {
    setValue(inputToValue(from, to));
  }, []);

  return (
    <Calendar
      selectRange={true}
      next2Label={null}
      prev2Label={null}
      className={styles.body}
      tileClassName={styles.title}
      value={value}
      nextLabel={<Icon name="angle-right" />}
      prevLabel={<Icon name="angle-left" />}
      onChange={onCalendarChange}
      locale="en"
    />
  );
});

const Footer = memo<Props>(({ onClose, onApply }) => {
  const theme = useTheme();
  const styles = getFooterStyles(theme);

  return (
    <div className={styles.container}>
      <Button className={styles.apply} onClick={onApply}>
        Apply time range
      </Button>
      <Button variant="secondary" onClick={onClose}>
        Cancel
      </Button>
    </div>
  );
});

export function inputToValue(from: DateTime, to: DateTime, invalidDateDefault: Date = new Date()): Date[] {
  const fromAsDate = from.toDate();
  const toAsDate = to.toDate();
  const fromAsValidDate = dateTime(fromAsDate).isValid() ? fromAsDate : invalidDateDefault;
  const toAsValidDate = dateTime(toAsDate).isValid() ? toAsDate : invalidDateDefault;

  if (fromAsValidDate > toAsValidDate) {
    return [toAsValidDate, fromAsValidDate];
  }
  return [fromAsValidDate, toAsValidDate];
}

function useOnCalendarChange(onChange: (from: DateTime, to: DateTime) => void, timeZone?: TimeZone) {
  return useCallback(
    (value: Date | Date[]) => {
      if (!Array.isArray(value)) {
        return console.error('onCalendarChange: should be run in selectRange={true}');
      }

      const from = dateTimeParse(dateInfo(value[0]), { timeZone });
      const to = dateTimeParse(dateInfo(value[1]), { timeZone });

      onChange(from, to);
    },
    [onChange]
  );
}

function dateInfo(date: Date): number[] {
  return [date.getFullYear(), date.getMonth(), date.getDate(), date.getHours(), date.getMinutes(), date.getSeconds()];
}
