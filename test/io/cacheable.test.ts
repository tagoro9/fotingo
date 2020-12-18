import { describe, expect, jest, test } from '@jest/globals';
import * as Keyv from 'keyv';
import { cacheable } from 'src/io/cacheable';
import { mocked } from 'ts-jest/utils';

jest.mock('keyv', () => {
  /* eslint-disable @typescript-eslint/no-explicit-any */
  const values: { [k: string]: any } = {};
  const get = jest.fn((key: string): any => values[key]);
  const set = jest.fn((key: string, value: any) => {
    values[key] = value;
  });
  /* eslint-enable @typescript-eslint/no-explicit-any */
  return class InMemoryKeyV {
    static get = get;
    static set = set;
    public get = get;
    public set = set;
  };
});

const mockKeyv = mocked(Keyv, true);

class CacheableClass {
  private value = 'prefix';
  private innerMock: () => void;

  constructor(innerMock: () => void) {
    this.innerMock = innerMock;
  }

  @cacheable({
    getPrefix(this: CacheableClass) {
      return this.value;
    },
    minutes: 60,
  })
  cached(name: string): Promise<string[]> {
    this.innerMock();
    return Promise.resolve([name, name]);
  }
}

describe('@cacheable', () => {
  test('caches values returned from the function', async () => {
    const mock = jest.fn() as () => unknown;
    const cacheable = new CacheableClass(mock);
    await cacheable.cached('test');
    await cacheable.cached('test');
    expect(mock).toHaveBeenCalledTimes(1);
    const getMock = ((mockKeyv as unknown) as { get: jest.Mock }).get;
    const setMock = ((mockKeyv as unknown) as { set: jest.Mock }).set;
    expect(getMock).toHaveBeenCalledTimes(2);
    expect(getMock.mock.calls[0]).toMatchSnapshot();
    expect(getMock.mock.calls[1]).toMatchSnapshot();
    expect(setMock).toHaveBeenCalledTimes(1);
    expect(setMock.mock.calls[0]).toMatchSnapshot();
  });

  test('propagates any error of the cached function', async () => {
    const mock = jest.fn(() => {
      throw new Error('Fail');
    });
    const cacheable = new CacheableClass(mock);
    await expect(cacheable.cached('some name')).rejects.toThrow('Fail');
  });
});
