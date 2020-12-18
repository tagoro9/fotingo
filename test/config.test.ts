/**
 * Config testsC
 */
import { describe, expect, jest, test } from '@jest/globals';
import * as cosmiconfig from 'cosmiconfig';
import { readConfig } from 'src/config';
import { mocked } from 'ts-jest/utils';

jest.mock('cosmiconfig', () => ({
  cosmiconfigSync: jest.fn().mockReturnValue(jest.fn()),
}));

const mockCosmiconfig = mocked(cosmiconfig);

// TODO Use real Config instances for tests

const mockSearch = (search: () => unknown) => {
  mockCosmiconfig.cosmiconfigSync.mockImplementation(
    () =>
      (({
        search,
      } as unknown) as ReturnType<typeof mockCosmiconfig.cosmiconfigSync>),
  );
};

describe('config', () => {
  test('reads config from current path and home path', () => {
    const search = jest.fn().mockReturnValue({ isEmpty: true });
    mockSearch(search);
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
    mockSearch(search);
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
