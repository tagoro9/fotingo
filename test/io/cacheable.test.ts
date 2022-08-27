import Keyv from 'keyv';
import { cacheable } from 'src/io/cacheable';
import { describe, expect, test, vi } from 'vitest';

vi.mock('keyv', () => {
  /* eslint-disable @typescript-eslint/no-explicit-any */
  const values: { [k: string]: any } = {};
  const get = vi.fn((key: string): any => values[key]);
  const set = vi.fn((key: string, value: any) => {
    values[key] = value;
  });
  /* eslint-enable @typescript-eslint/no-explicit-any */
  return {
    default: class InMemoryKeyV {
      static get = get;
      static set = set;
      public get = get;
      public set = set;
    },
  };
});

const mockKeyv = vi.mocked(Keyv, true);

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
    const mock = vi.fn() as () => unknown;
    const cacheable = new CacheableClass(mock);
    await cacheable.cached('test');
    await cacheable.cached('test');
    expect(mock).toHaveBeenCalledTimes(1);
    const getMock = (mockKeyv as unknown as { get: ReturnType<typeof vi.fn> }).get;
    const setMock = (mockKeyv as unknown as { set: ReturnType<typeof vi.fn> }).set;
    expect(getMock).toHaveBeenCalledTimes(2);
    expect(getMock.mock.calls[0]).toMatchSnapshot();
    expect(getMock.mock.calls[1]).toMatchSnapshot();
    expect(setMock).toHaveBeenCalledTimes(1);
    expect(setMock.mock.calls[0]).toMatchSnapshot();
  });

  test('propagates any error of the cached function', async () => {
    const mock = vi.fn(() => {
      throw new Error('Fail');
    });
    const cacheable = new CacheableClass(mock);
    await expect(cacheable.cached('some name')).rejects.toThrow('Fail');
  });
});
