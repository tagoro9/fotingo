import Keyv from 'keyv';
import { homedir } from 'os';

const path = `sqlite://${homedir()}/.fotingo_config/cache.sqlite3`;
const keyv = new Keyv(path);

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type PromiseFunction = (...args: any[]) => Promise<any>;

type Cacheable = (
  // eslint-disable-next-line @typescript-eslint/ban-types
  target: Object,
  propertyKey: string | symbol,
  descriptor: TypedPropertyDescriptor<PromiseFunction>,
) => TypedPropertyDescriptor<PromiseFunction>;

const isCacheDisabled = process.env.FOTINGO_DISABLE_CACHE !== undefined;

/**
 * Decorator that caches the output of the decorated function
 * in an external data source (SQLite DB) so it can be
 * accessed across multiple executions
 * Caching is based on the function input, the specified
 * prefix and the number of minutes the data is supposed to be
 * cached
 */
export function cacheable({
  getPrefix,
  minutes,
}: {
  getPrefix?: () => string;
  minutes?: number;
}): Cacheable {
  return (
    // eslint-disable-next-line @typescript-eslint/ban-types
    target: Object,
    propertyKey: string | symbol,
    descriptor: TypedPropertyDescriptor<PromiseFunction>,
  ): TypedPropertyDescriptor<PromiseFunction> => {
    const method = descriptor.value;
    if (typeof method !== 'function') {
      throw new TypeError(
        `@cacheable decorator can only be applied to methods not: ${typeof method}`,
      );
    }

    const cachedFunction: PromiseFunction = async function (...functionArguments) {
      if (isCacheDisabled) {
        return method.call(this, ...functionArguments);
      }
      const prefix = getPrefix ? getPrefix.call(this, ...functionArguments) : '';
      const keyArguments =
        functionArguments.length > 0
          ? `_${functionArguments.map((value) => JSON.stringify(value)).join('_')}`
          : '';
      const key = `${prefix}${target.constructor.name}_${String(propertyKey)}${keyArguments}`;
      const cachedValue = await keyv.get(key);
      if (cachedValue) {
        return Promise.resolve(cachedValue);
      }
      const result = await method.call(this, ...functionArguments);
      await keyv.set(key, result, minutes ? minutes * 60 * 1000 : undefined);
      return Promise.resolve(result);
    };

    descriptor.value = cachedFunction;
    return descriptor;
  };
}

// One day in minutes
export const ONE_DAY = 60 * 24;
