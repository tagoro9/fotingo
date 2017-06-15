import R from 'ramda';

export const ISSUE_TYPES = {
  Bug: 'b',
  Feature: 'f',
  Story: 'f',
  'Sub-task': 'f',
  Task: 'c',
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

const getTemplateData = (config, issue) => ({
  template: config.get(['jira', 'templates', 'branch']) || defaultBranchTemplate,
  data: {
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

export const getIssueIdFromBranch = R.compose(
  R.ifElse(R.isNil, R.identity, R.toUpper),
  R.last,
  R.match(/\w\/(\w+-\d+)/),
);
