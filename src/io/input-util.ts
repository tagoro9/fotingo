import { take, uniqBy } from 'ramda';

import { Messenger } from './messenger';
import { series } from './promise-util';

interface AskToSelectMatchData<T> {
  data: T[][];
  options: string[];
  useDefaults: boolean;
  getQuestion: (item: string) => string;
  getLabel: (item: T) => string;
  getValue: (item: T) => string;
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
  { data, getLabel, getQuestion, getValue, options, useDefaults }: AskToSelectMatchData<T>,
  messenger: Messenger,
): Promise<T[]> {
  return series(
    data.map((matches, i) => () => {
      if (!matches || matches.length === 0) {
        throw new Error(`No match found for ${options[i]}`);
      }
      if (useDefaults || matches.length === 1) {
        return Promise.resolve(matches[0]);
      }
      return messenger
        .request(getQuestion(options[i]), {
          options: uniqBy<T, string>(getValue, take(5, matches)).map(r => ({
            label: getLabel(r),
            value: getValue(r),
          })),
        })
        .toPromise()
        .then((option: string) => matches.find(r => String(getValue(r)) === String(option)));
    }),
  );
}
