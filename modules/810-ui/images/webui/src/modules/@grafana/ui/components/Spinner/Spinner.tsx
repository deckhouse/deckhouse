import React, { FC } from 'react';
import { cx, css } from 'emotion';
import { stylesFactory } from '../../themes';
import { Icon } from '../Icon/Icon';

const getStyles = stylesFactory((size: number, inline: boolean) => {
  return {
    wrapper: css`
      font-size: ${size}px;
      ${inline
        ? css`
            display: inline-block;
          `
        : ''}
    `,
  };
});

type Props = {
  className?: string;
  style?: React.CSSProperties;
  iconClassName?: string;
  inline?: boolean;
  size?: number;
};
export const Spinner: FC<Props> = (props: Props) => {
  const { className, inline = false, iconClassName, style, size = 16 } = props;
  const styles = getStyles(size, inline);
  return (
    <div style={style} className={cx(styles.wrapper, className)}>
      <Icon className={cx('fa-spin', iconClassName)} name="fa fa-spinner" />
    </div>
  );
};
