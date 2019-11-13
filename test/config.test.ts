/**
 * Config testsC
 */

import 'jest';

import * as cosmiconfig from 'cosmiconfig';
import { read } from 'src/config';

jest.mock('cosmiconfig');

// TODO Use real Config instances for tests

describe('config', () => {
  test('reads config from current path and home path', () => {
    const searchSync = jest.fn().mockReturnValue({ isEmpty: true });
    (cosmiconfig as jest.Mock).mockImplementation(() => ({
      searchSync,
    }));
    expect(read()).toEqual({});
    expect(searchSync).toHaveBeenCalledTimes(2);
    expect(searchSync).toBeCalledWith(process.env.HOME);
  });

  test('merges configuration objects', () => {
    const searchSync = jest.fn().mockReturnValueOnce({
      config: {
        someDeepKey: { key1: 'value', keyShared: 'first' },
        someKey: 'value',
        test: 'value',
      },
      isEmpty: false,
    });
    searchSync.mockReturnValueOnce({
      config: {
        anotherValue: 'anotherValue',
        someDeepKey: { key2: 'value', keyShared: 'second' },
        test: 'value2',
      },
      isEmpty: false,
    });
    (cosmiconfig as jest.Mock).mockImplementation(() => ({
      searchSync,
    }));
    expect(read()).toEqual({
      anotherValue: 'anotherValue',
      someDeepKey: {
        key1: 'value',
        key2: 'value',
        keyShared: 'first',
      },
      someKey: 'value',
      test: 'value',
    });
  });
});
