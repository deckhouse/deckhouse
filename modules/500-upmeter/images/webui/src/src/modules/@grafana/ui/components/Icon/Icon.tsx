import React from 'react';
import { cx } from 'emotion';
import { IconName, IconType, IconSize } from '../../types/icon';



export interface IconProps extends React.HTMLAttributes<HTMLDivElement> {
  name: IconName;
  size?: IconSize;
  type?: IconType;
}

export const Icon = React.forwardRef<HTMLDivElement, IconProps>(
  ({ size = 'md', type = 'default', name, className, style, ...divElementProps }, ref) => {
    /* Temporary solution to display also font awesome icons */
    const isFontAwesome = name?.includes('fa-');
    if (isFontAwesome) {
      return <i className={cx(name, className)} {...divElementProps} style={style} />;
    }

    return <span>#</span>


  }
);

Icon.displayName = 'Icon';
