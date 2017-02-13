import R from 'ramda';

export const ISSUE_TYPES = {
  Bug: 'b',
  Feature: 'f',
  Story: 'f',
  'Sub-task': 'f',
  Task: 'c'
};

const sanitizeSummary = R.compose(
  R.take(72),
  R.replace(/(_|-)$/, ''),
  R.replace(/--+/, '-'),
  R.replace(/__+/, '_'),
  R.replace(/\s|\(|\)/g, '_'),
  R.replace(/\/|\./g, '-'),
  R.replace(/,|\[|]|"|'|”|“|@|’|`|:|\$|\?|\*/g, ''),
  R.toLower
);

// This should go in the issue?
export const createBranchName = ({ key, fields: { issuetype: { name: type }, summary } }) =>
  `${ISSUE_TYPES[type]}/${key.toLowerCase()}_${sanitizeSummary(summary)}`;

export const getProject = R.compose(R.nth(1), R.match(/\/((\w|-)+)$/));

export const getIssueIdFromBranch = R.compose(R.toUpper, R.last, R.match(/\w\/(\w+-\d+)/));
