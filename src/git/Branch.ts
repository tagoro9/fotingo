/**
 * Git branch handler
 */
import escapeStringRegexp from 'escape-string-regexp';
import {
  compose,
  converge,
  curryN,
  identity,
  ifElse,
  isNil,
  lt,
  mapObjIndexed,
  match,
  nth,
  nthArg,
  prop,
  reduce,
  replace,
  subtract,
  toPairs,
  toUpper,
} from 'ramda';
import { Issue, IssueType } from 'src/types';

import { GitConfig } from './Config';

const ISSUE_TYPE_TO_BRANCH_PREFIX: { [S in IssueType]: string } = {
  [IssueType.BUG]: 'b',
  [IssueType.FEATURE]: 'f',
  [IssueType.STORY]: 'f',
  [IssueType.SPIKE]: 's',
  [IssueType.SUB_TASK]: 'f',
  [IssueType.TASK]: 'c',
  [IssueType.TECH_DEBT]: 'd',
};

enum TemplateKey {
  ISSUE_KEY = 'issue.key',
  ISSUE_SANITIZED_SUMMARY = 'issue.sanitizedSummary',
  ISSUE_SHORT_NAME = 'issue.shortName',
}

const TEMPLATE_KEYS_TO_MATCHERS: { [S in TemplateKey]: string } = {
  [TemplateKey.ISSUE_SHORT_NAME]: '(\\w+)',
  [TemplateKey.ISSUE_KEY]: '(\\w+-\\d+)',
  [TemplateKey.ISSUE_SANITIZED_SUMMARY]: '([\\w_-]+)',
};

interface TemplateData {
  data: {
    [TemplateKey.ISSUE_SHORT_NAME]: string;
    [TemplateKey.ISSUE_KEY]: string;
    [TemplateKey.ISSUE_SANITIZED_SUMMARY]: string;
  };
  template: string;
}

/**
 * Given a git configuration and an issue build the branch template data
 * @param config Git configuration
 * @param issue Issue
 */
function getTemplateData(config: GitConfig, issue: Issue): TemplateData {
  return {
    data: {
      [TemplateKey.ISSUE_SHORT_NAME]: ISSUE_TYPE_TO_BRANCH_PREFIX[issue.type],
      [TemplateKey.ISSUE_KEY]: issue.key.toLowerCase(),
      [TemplateKey.ISSUE_SANITIZED_SUMMARY]: issue.sanitizedSummary,
    },
    template: config.branchTemplate,
  };
}

/**
 * Get a branch name for the passed configuration and issue
 * @param config Git configuration
 * @param issue Issue
 */
export const getName = curryN(
  2,
  compose(
    converge(
      reduce((message: string, [k, v]: string[]) => replace(`{${k}}`, v, message)),
      [prop('template'), compose(toPairs, prop('data'))],
    ),
    getTemplateData,
  ),
);

/**
 * Build a branch template regex from the current git config branch template
 * @param config Git configuration
 */
const buildBranchTemplateRegex = compose(
  (branchTemplate: string) =>
    reduce(
      (message: string, [k, v]: [TemplateKey, string]) => replace(`{${k}}`, `${v}`, message),
      branchTemplate,
      toPairs(TEMPLATE_KEYS_TO_MATCHERS),
    ),
  prop('branchTemplate'),
);

/**
 * Given a regex that matches a branchTemplate, build a map between the template keys and the
 * matching group index in the regex
 * @param regex Regex to match a branch name
 */
const getTemplateKeysMatchIndexMap = (regex: string): { [S in TemplateKey]: number } => {
  const indexMap = Object.fromEntries(
    Object.values(TemplateKey).map((value: TemplateKey) => [
      value,
      (
        regex.match(escapeStringRegexp(TEMPLATE_KEYS_TO_MATCHERS[value])) || {
          index: -1,
        }
      ).index,
    ]),
  );
  const orderedValues = Object.values(indexMap).sort(subtract).filter(lt(-1));
  return mapObjIndexed((index) => 1 + orderedValues.indexOf(index), indexMap) as {
    [S in TemplateKey]: number;
  };
};

/**
 * Extract the issue id from the branch name given a git configuration
 * @param config Git configuration
 * @param branchName Branch name
 */
export const getIssueId = curryN(
  2,
  converge(
    compose(
      ifElse(isNil, identity, toUpper),
      converge(nth, [compose(prop(TemplateKey.ISSUE_KEY), getTemplateKeysMatchIndexMap), match]),
    ),
    [buildBranchTemplateRegex, nthArg(1)],
  ),
);
