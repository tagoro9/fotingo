export function series(providers: Array<() => Promise<any>>) {
  const ret = Promise.resolve(null);
  const results: any[] = [];

  return providers
    .reduce((result, provider, index) => {
      return result.then(() => {
        return provider().then(val => {
          results[index] = val;
        });
      });
    }, ret)
    .then(() => {
      return results;
    });
}
