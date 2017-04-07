import program from 'commander';
import R from 'ramda';

import { validate } from './issue-tracker/util';
import config from './config';
import { handleError } from './error';
import init from './init';
import reporter from './reporter';
import { wrapInPromise } from './util';

const getIssueId = R.compose(validate, R.head, R.prop('args'));

try {
  program
    .option('-b, --branch [branch]', 'Name of the base branch to use')
    .option('-n, --no-branch-issue', 'Do not create a branch with the issue name')
    .parse(process.argv);
  config.update(['git', 'branch'], program.branch, true);
  const issueId = getIssueId(program);
  const { step, stepCurried } = reporter.stepFactory(program.branchIssue ? 4 : 3);
  step(1, 'Initializing services', 'rocket');
  init(config, program)
    .then(({ git, issueTracker }) =>
      issueTracker
        .getCurrentUser()
        .then(stepCurried(2, `Getting '${issueId}' from ${issueTracker.name}`, 'bug'))
        .then(user =>
          issueTracker
            .getIssue(issueId)
            .then(issueTracker.canWorkOnIssue(user))
            .then(stepCurried(3, `Setting '${issueId}' in progress`, 'bookmark'))
            .then(issueTracker.setIssueStatus({ status: issueTracker.status.IN_PROGRESS }))
            .then(R.compose(wrapInPromise, git.createBranchName))
            .then(
              R.ifElse(
                R.partial(R.propEq('branchIssue', true), [program]),
                R.compose(
                  git.createIssueBranch(config),
                  stepCurried(4, name => `Creating branch '${name}'`, 'tada'),
                ),
                R.always(undefined),
              ),
            )))
    .then(reporter.footer)
    .catch(handleError);
} catch (e) {
  handleError(e);
  program.help();
}
