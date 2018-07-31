import * as Fuse from 'fuse.js';
import { compose, converge, filter, flip, invoker, isEmpty, map, not, nthArg } from 'ramda';

interface SearchOptions<T extends any> {
  fields: string[];
  data: T[];
}

/**
 * Given a searcher and the data where to search, return a list of items matching any of the strings to match
 */
const searchAndGetFirstResult: <T>(searcher: (t: string) => T[], data: string[]) => T[][] = compose<
  (t: string) => any[],
  string[],
  any[],
  any[][]
>(
  filter(
    compose(
      not,
      isEmpty,
    ),
  ),
  map,
);

/**
 * Given search options produce a search method that will find the matches
 */
const buildSearcher: <T>(options: SearchOptions<T>) => (t: string[]) => T[] = compose<
  SearchOptions<any>,
  Fuse<any>,
  (t: string[]) => any[]
>(
  flip(invoker(1, 'search')),
  opts =>
    new Fuse(opts.data, {
      caseSensitive: false,
      keys: opts.fields,
      shouldSort: true,
    }),
);

/**
 * Given a search options and a list of strings to find, return the list of data objects that
 * have any match in the specified fields for the list of strings
 */
export const findMatches: <T>(options: SearchOptions<T>, search: string[]) => T[][] = converge(
  searchAndGetFirstResult,
  [buildSearcher, nthArg(1)],
);
