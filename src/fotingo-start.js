import program from 'commander';
import R from 'ramda';

import { validate } from './issue-tracker/util';
import config from './config';
import { handleError } from './error';
import init from './init';
import reporter from './reporter';

const { step, stepCurried } = reporter.stepFactory(4);
const getIssueId = R.compose(validate, R.head, R.prop('args'));

try {
  program.parse(process.argv);
  const issueId = getIssueId(program);
  step(1, 'Initializing services', 'rocket');
  init(config, program)
    .then(({ git, issueTracker }) =>
      issueTracker.getCurrentUser()
        .then(stepCurried(2, `Getting issue from ${issueTracker.name}`, 'bug'))
        .then(user =>
          issueTracker.getIssue(issueId)
            .then(issueTracker.canWorkOnIssue(user))
            .then(stepCurried(3, 'Updating issue status', 'bookmark'))
            .then(issueTracker.setIssueStatus({ status: issueTracker.status.IN_PROGRESS }))
            .then(stepCurried(4, 'Creating branch to work on issue', 'tada'))
            .then(git.createIssueBranch(config))
        ))
    .then(reporter.footer)
    .catch(handleError);
} catch (e) {
  handleError(e);
  program.help();
}
