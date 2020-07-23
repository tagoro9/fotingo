import React = require('react');

import { Instance, render } from 'ink';

import { Fotingo } from './Fotingo';
import { FotingoProps } from './props';

export function renderUi(props: FotingoProps): Instance {
  return render(<Fotingo {...props} />);
}
