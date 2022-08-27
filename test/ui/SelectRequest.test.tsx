import { render } from 'ink-testing-library';
import { MessageType, RequestType, SelectRequest as SelectRequestType } from 'src/io/messenger';
import { SelectRequest } from 'src/ui/SelectRequest';
import { describe, expect, test, vi } from 'vitest';

import React = require('react');

describe('<SelectRequest />', () => {
  const request: SelectRequestType = {
    allowTextSearch: true,
    message: 'Select an option',
    options: [
      { label: 'Item 1', value: 1 },
      { label: 'Item 2', value: 2 },
      { label: 'Item 3', value: 3 },
    ],
    showSpinner: false,
    requestType: RequestType.SELECT,
    type: MessageType.REQUEST,
  };

  test('displays the list of options filtering by the text input', async () => {
    const actual = render(<SelectRequest onSubmit={vi.fn()} request={request} />);
    expect(actual.lastFrame()).toMatchSnapshot();
    actual.stdin.write('1');
    expect(actual.lastFrame()).toMatchSnapshot();
  });

  // eslint-disable-next-line jest/no-disabled-tests
  test.skip('allows to navigate the list of options using the arrow keys', async () => {
    const ARROW_DOWN = '\u001B[B';

    const actual = render(
      <SelectRequest
        onSubmit={vi.fn()}
        request={{
          ...request,
          allowTextSearch: false,
        }}
      />,
    );
    expect(actual.lastFrame()).toMatchSnapshot();
    actual.stdin.write(ARROW_DOWN);
    expect(actual.lastFrame()).toMatchSnapshot();
  });
});
