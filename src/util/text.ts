import Fuse from 'fuse.js';
import { compose, converge, filter, flip, invoker, isEmpty, map, not, nthArg } from 'ramda';

// Base types that we allow to search for
type Searchable = object | string | number;

interface SearchOptions<T extends Searchable> {
  // Allow to search for exact matches before using fuzzy search
  checkForExactMatchFirst?: boolean;
  // In order to search for exact matches, we can pass a function that cleans the data
  // before searching for the exact match
  cleanData?: (item: string) => string;
  data: T[];
  fields: (keyof T)[];
}
/**
 * Given a search function and the data where to search, return a list of items matching any of the strings to match
 */
const search: (searcher: (t: string) => Searchable[], data: string[]) => Searchable[][] = compose<
  (t: string) => Searchable[],
  string[],
  Searchable[][],
  Searchable[][]
>(filter(compose(not, isEmpty)), map);

/**
 * Build a search function based on the specified search options
 * @param options Search options
 * @return Searcher
 */
const getSearcher = <T extends Searchable>(
  options: SearchOptions<T>,
): { search: (s: string) => T[] } => {
  const fuse = new Fuse<T, Fuse.FuseOptions<T>>(options.data, {
    caseSensitive: false,
    keys: options.fields,
    includeMatches: false,
    includeScore: false,
    shouldSort: true,
    threshold: 0.3,
  });
  if (options.checkForExactMatchFirst) {
    return {
      search(s: string): T[] {
        const exactMatch = options.data.find(item =>
          options.fields.some(field => {
            const cleanedData = options.cleanData
              ? options.cleanData(String(item[field]))
              : item[field];
            return cleanedData === s;
          }),
        );
        if (exactMatch) {
          return [exactMatch];
        }
        return fuse.search<T, false, false>(s);
      },
    };
  }
  return fuse as { search: (s: string) => T[] };
};

/**
 * Given a search options and a list of strings to find, return the list of data objects that
 * have any match in the specified fields for the list of strings
 */
export const findMatches: <T extends Searchable>(
  options: SearchOptions<T>,
  search: string[],
) => T[][] = converge(search, [compose(flip(invoker(1, 'search')), getSearcher), nthArg(1)]);
