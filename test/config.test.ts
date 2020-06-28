/**
 * Config testsC
 */

import 'jest';

import * as cosmiconfig from 'cosmiconfig';
import { readConfig } from 'src/config';

jest.mock('cosmiconfig', () => ({
  cosmiconfigSync: jest.fn().mockReturnValue(jest.fn()),
}));

// TODO Use real Config instances for tests

describe('config', () => {
  test('reads config from current path and home path', () => {
    const search = jest.fn().mockReturnValue({ isEmpty: true });
    (cosmiconfig.cosmiconfigSync as jest.Mock).mockImplementation(() => ({
      search,
    }));
    expect(readConfig()).toEqual({});
    expect(search).toHaveBeenCalledTimes(2);
    expect(search).toBeCalledWith(process.env.HOME);
  });

  test('merges configuration objects', () => {
    const search = jest.fn().mockReturnValueOnce({
      config: {
        someDeepKey: { key1: 'value', keyShared: 'first' },
        someKey: 'value',
        test: 'value',
      },
      isEmpty: false,
    });
    search.mockReturnValueOnce({
      config: {
        anotherValue: 'anotherValue',
        someDeepKey: { key2: 'value', keyShared: 'second' },
        test: 'value2',
      },
      isEmpty: false,
    });
    (cosmiconfig.cosmiconfigSync as jest.Mock).mockImplementation(() => ({
      search,
    }));
    expect(readConfig()).toEqual({
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
