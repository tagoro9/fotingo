import R from 'ramda';

export const ISSUE_TYPES = {
  Bug: 'b',
  Feature: 'f',
  Story: 'f',
  'Sub-task': 'f',
  Task: 'c',
};

const TEMPLATE_KEY_MATCHERS = {
  'issue.key': '(\\w+-\\d+)',
  'issue.shortName': '\\w+',
  'issue.sanitizedSummary': '[\\w_-]+',
};

const sanitizeSummary = R.compose(
  R.take(72),
  R.replace(/(_|-)$/, ''),
  R.replace(/--+/, '-'),
  R.replace(/__+/, '_'),
  R.replace(/\s|\(|\)/g, '_'),
  R.replace(/\/|\./g, '-'),
  R.replace(/,|\[|]|"|'|”|“|@|’|`|:|\$|\?|\*/g, ''),
  R.toLower,
);

const defaultBranchTemplate = '{issue.shortName}/{issue.key}_{issue.sanitizedSummary}';

const getTemplate = config => config.get(['jira', 'templates', 'branch']) || defaultBranchTemplate;

const getTemplateData = (config, issue) => ({
  template: getTemplate(config),
  matchers: TEMPLATE_KEY_MATCHERS,
  data: issue && {
    'issue.shortName': config.get(['jira', 'issueTypes', issue.fields.issuetype.id]).shortName,
    'issue.key': issue.key.toLowerCase(),
    'issue.sanitizedSummary': sanitizeSummary(issue.fields.summary),
  },
});

// Object -> Object -> String
export const createBranchName = R.curryN(
  2,
  R.compose(
    R.converge(R.reduce((msg, [k, v]) => R.replace(`{${k}}`, v, msg)), [
      R.prop('template'),
      R.compose(R.toPairs, R.prop('data')),
    ]),
    getTemplateData,
  ),
);

export const getProject = R.compose(R.nth(1), R.match(/\/((\w|-)+)$/));

export const getIssueIdFromBranch = R.curryN(
  2,
  R.compose(
    R.converge(R.compose(R.ifElse(R.isNil, R.identity, R.toUpper), R.last, R.match), [
      R.compose(
        // This returns the regex
        R.reduce(
          (msg, [k, v]) => R.replace(`{${k}}`, v, msg),
          R.__, // Here goes the template
          R.toPairs(TEMPLATE_KEY_MATCHERS),
        ),
        getTemplate, // New function that I suggested above
        R.nthArg(0), // Config
      ),
      R.nthArg(1), // Branch name
    ]),
  ),
);
