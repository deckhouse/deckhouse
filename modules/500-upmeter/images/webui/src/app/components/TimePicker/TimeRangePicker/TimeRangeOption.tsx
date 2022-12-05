/**
 * Custom version of TimeRangeOption from @grafana/ui.
 * Changes:
 * - change linear-gradient for hover
 */

import React, { memo } from "react"
import { css } from "emotion"
import { GrafanaTheme, TimeOption } from "@grafana/data"
import { useTheme, stylesFactory, selectThemeVariant } from "@grafana/ui"
import { Icon } from "../../Icon"

const getStyles = stylesFactory((theme: GrafanaTheme) => {
  const background = selectThemeVariant(
    {
      light: theme.palette.gray7,
      dark: theme.palette.dark3,
    },
    theme.type,
  )

  return {
    container: css`
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 7px 9px 7px 9px;
      border-left: 2px solid rgba(255, 255, 255, 0);

      &:hover {
        background: ${background};
        border-image: linear-gradient(#007bff 30%, #deeefd 99%);
        border-image-slice: 1;
        border-style: solid;
        border-top: 0;
        border-right: 0;
        border-bottom: 0;
        border-left-width: 2px;
        cursor: pointer;
      }
    `,
  }
})

interface Props {
  value: TimeOption
  selected?: boolean
  onSelect: (option: TimeOption) => void
}

export const TimeRangeOption = memo<Props>(({ value, onSelect, selected = false }) => {
  const theme = useTheme()
  const styles = getStyles(theme)

  return (
    <div className={styles.container} onClick={() => onSelect(value)} tabIndex={-1}>
      <span>{value.display}</span>
      {selected ? <Icon name="check" /> : null}
    </div>
  )
})
