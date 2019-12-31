import React = require('react');

import { render } from 'ink';

import { Fotingo } from './Fotingo';
import { FotingoProps } from './props';

export function renderUi(props: FotingoProps) {
  render(<Fotingo {...props} />);
}
