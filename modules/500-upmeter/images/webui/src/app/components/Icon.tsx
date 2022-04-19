import React from "react"
import { cx } from "emotion"

export interface IconProps {
  name: string
  size?: string
  className?: string
}

export const Icon = React.forwardRef<HTMLDivElement, IconProps>(
  ({ name, size, className, ...divElementProps }, ref) => {
    return (
      <div className={cx("icon-container", className)} {...divElementProps} ref={ref}>
        <i className={cx("fas", name)}></i>
      </div>
    )
  },
)
