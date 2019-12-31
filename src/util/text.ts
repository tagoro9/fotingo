import * as Fuse from 'fuse.js';
import { compose, converge, filter, flip, invoker, isEmpty, map, not, nthArg } from 'ramda';

interface SearchOptions<T extends any> {
  // Allow to search for exact matches before using fuzzy search
  checkForExactMatchFirst?: boolean;
  // In order to search for exact matches, we can pass a function that cleans the data
  // before searching for the exact match
  cleanData?: (item: string) => string;
  data: T[];
  fields: string[];
}

/**
 * Given a searcher and the data where to search, return a list of items matching any of the strings to match
 */
const search: <T>(searcher: (t: string) => T[], data: string[]) => T[][] = compose<
  (t: string) => any[],
  string[],
  any[],
  any[][]
>(filter(compose(not, isEmpty)), map);

/**
 * Given search options produce a search method that will find the matches
 */
const buildSearcher: <T>(options: SearchOptions<T>) => (t: string[]) => T[] = compose<
  SearchOptions<any>,
  { search: (s: string) => any[] },
  (t: string[]) => any[]
>(flip(invoker(1, 'search')), opts => {
  const fuse = new Fuse(opts.data, {
    caseSensitive: false,
    keys: opts.fields,
    shouldSort: true,
    threshold: 0.3,
  });
  if (opts.checkForExactMatchFirst) {
    return {
      search: (s: string) => {
        const exactMatch = opts.data.find(item =>
          opts.fields.some(field => {
            const cleanedData = opts.cleanData ? opts.cleanData(item[field]) : item[field];
            return cleanedData === s;
          }),
        );
        if (exactMatch) {
          return [exactMatch];
        }
        return fuse.search(s);
      },
    };
  }
  return fuse;
});

/**
 * Given a search options and a list of strings to find, return the list of data objects that
 * have any match in the specified fields for the list of strings
 */
export const findMatches: <T>(
  options: SearchOptions<T>,
  search: string[],
) => T[][] = converge(search, [buildSearcher, nthArg(1)]);
