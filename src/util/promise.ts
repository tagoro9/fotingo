/**
 * Given a list of promise providers, execute them in series and return a promise that resolves with
 * all the resolved values from the providers
 * @param providers Promise providers
 */
export function series<T>(providers: Array<() => Promise<T>>): Promise<T[]> {
  const returnPromise = Promise.resolve(undefined);
  const results: T[] = [];

  return providers
    .reduce((result, provider, index) => {
      return result.then(() => {
        return provider().then((value) => {
          results[index] = value;
        });
      });
    }, returnPromise)
    .then(() => {
      return results;
    });
}
