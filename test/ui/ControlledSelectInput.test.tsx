import { describe, expect, test } from '@jest/globals';
import React = require('react');

import { render } from 'ink-testing-library';
import { ControlledSelectInput } from 'src/ui/ControlledSelectInput';

describe('<ControlledSelectInput />', () => {
  const items = [
    { label: 'Item 1', value: 1 },
    { label: 'Item 2', value: 2 },
  ];

  test('renders the list of items', () => {
    const props = {
      items,
    };
    const { lastFrame } = render(<ControlledSelectInput {...props} />);
    expect(lastFrame()).toMatchSnapshot();
  });

  test('renders the selected item with a pointer', () => {
    const props = { items, selectedIndex: 1 };
    const { lastFrame } = render(<ControlledSelectInput {...props} />);
    expect(lastFrame()).toMatchSnapshot();
  });
});
