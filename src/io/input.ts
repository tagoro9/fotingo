import { take, uniqBy } from 'ramda';
import { series } from 'src/util/promise';

import { Messenger } from './messenger';

interface AskToSelectMatchData<T> {
  allowTextSearch?: boolean;
  data: T[][];
  getLabel: (item: T) => string;
  getQuestion: (item: string) => string;
  getValue: (item: T) => string;
  limit?: number;
  options?: string[];
  useDefaults: boolean;
}

/**
 * Given the options that a user selected and the found matches, ask the user to select
 * the best match out of the first 5 matches. Select the first match in the list if using defaults
 * or the list only has one element
 * @param options Options for selecting the matches
 * @param options.data Data found for the options introduced by the user
 * @param options.getLabel Function that given a match, returns the label to present to the user
 * @param options.getQuestion Function that returns the question to present to the user. It receives the option introduced by the user
 * @param options.getValue Function that given a match, returns its value (typically an id)
 * @param options.options List of options that the use introduced
 * @param options.useDefaults Flag indicating if the useDefaults options was set
 */
export function maybeAskUserToSelectMatches<T>(
  {
    allowTextSearch = false,
    data,
    getLabel,
    getQuestion,
    getValue,
    limit = 5,
    options = [],
    useDefaults,
  }: AskToSelectMatchData<T>,
  messenger: Messenger,
): Promise<T[]> {
  return series(
    data.map((matches, i) => (): Promise<T> => {
      if (!matches || matches.length === 0) {
        throw new Error(`No match found for ${options[i]}`);
      }
      if (useDefaults || matches.length === 1) {
        return Promise.resolve(matches[0]);
      }
      return (
        messenger
          .request(getQuestion(options[i]), {
            allowTextSearch,
            options: uniqBy<T, string>(getValue, limit > 0 ? take(limit, matches) : matches).map(
              r => ({
                label: getLabel(r),
                value: getValue(r),
              }),
            ),
          })
          .toPromise()
          // We know the user selected an option
          // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
          .then((option: string) => matches.find(r => String(getValue(r)) === String(option))!)
      );
    }),
  );
}
