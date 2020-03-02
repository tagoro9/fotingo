import 'jest';

import * as Keyv from 'keyv';
import { cacheable } from 'src/io/cacheable';

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

class CacheableClass {
  private value = 'prefix';
  private innerMock: () => {};

  constructor(innerMock: () => {}) {
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
    const mock = jest.fn();
    const cacheable = new CacheableClass(mock);
    await cacheable.cached('test');
    await cacheable.cached('test');
    expect(mock).toHaveBeenCalledTimes(1);
    const getMock = ((Keyv as unknown) as { get: jest.Mock }).get;
    const setMock = ((Keyv as unknown) as { set: jest.Mock }).set;
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
