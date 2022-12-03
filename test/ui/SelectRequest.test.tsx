import { describe, expect, jest, test } from '@jest/globals';
import { render } from 'ink-testing-library';
import { MessageType, RequestType, SelectRequest as SelectRequestType } from 'src/io/messenger';
import { SelectRequest } from 'src/ui/SelectRequest';
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
    const actual = render(<SelectRequest onSubmit={jest.fn()} request={request} />);
    expect(actual.lastFrame()).toMatchSnapshot();
    await new Promise((resolve) => setTimeout(resolve, 100));
    actual.stdin.write('1');
    expect(actual.lastFrame()).toMatchSnapshot();
  });

  // eslint-disable-next-line jest/no-disabled-tests
  test('allows to navigate the list of options using the arrow keys', async () => {
    const ARROW_DOWN = '\u001B[B';

    const actual = render(
      <SelectRequest
        onSubmit={jest.fn()}
        request={{
          ...request,
          allowTextSearch: false,
        }}
      />,
    );
    expect(actual.lastFrame()).toMatchSnapshot();
    await new Promise((resolve) => setTimeout(resolve, 100));
    actual.stdin.write(ARROW_DOWN);
    expect(actual.lastFrame()).toMatchSnapshot();
  });
});
