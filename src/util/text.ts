import Fuse from 'fuse.js';
import { compose, converge, filter, flip, invoker, isEmpty, map, not, nthArg, prop } from 'ramda';

// Base types that we allow to search for
// eslint-disable-next-line @typescript-eslint/ban-types
type Searchable = Object | string | number;

interface SearchOptions<T extends Searchable> {
  // Allow to search for exact matches before using fuzzy search
  checkForExactMatchFirst?: boolean;
  // In order to search for exact matches, we can pass a function that cleans the data
  // before searching for the exact match
  cleanData?: (item: string) => string;
  data: T[];
  fields?: (keyof T)[];
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
): { search: (s: string) => Fuse.FuseResult<T>[] } => {
  const fuse = new Fuse<T>(options.data, {
    keys: (options.fields || []) as string[],
    includeMatches: false,
    includeScore: false,
    isCaseSensitive: false,
    shouldSort: true,
    threshold: 0.3,
  });
  if (options.checkForExactMatchFirst) {
    return {
      search(s: string): Fuse.FuseResult<T>[] {
        const exactMatch = options.data.find((item) =>
          (options.fields || []).some((field) => {
            const cleanedData = options.cleanData
              ? options.cleanData(String(item[field]))
              : item[field];
            return cleanedData === s;
          }),
        );
        if (exactMatch) {
          return [{ item: exactMatch, refIndex: 0 }];
        }
        return fuse.search(s);
      },
    };
  }
  return fuse as { search: (s: string) => Fuse.FuseResult<T>[] };
};

/**
 * Given a search options and a list of strings to find, return the list of data objects that
 * have any match in the specified fields for the list of strings
 */
export const findMatches: <T extends Searchable>(
  options: SearchOptions<T>,
  search: string[],
) => T[][] = converge(search, [
  compose(map(map(prop('item'))), flip(invoker(1, 'search')), getSearcher),
  nthArg(1),
]);
