import program from 'commander';
import R from 'ramda';

import { validateIssueId, validateIssueDescription } from './issue-tracker/util';
import config from './config';
import { handleError, throwControlledError, errors } from './error';
import init from './init';
import reporter from './reporter';
import { wrapInPromise } from './util';

const getMainArgument = validator => R.compose(validator, R.head, R.prop('args'));
const validateOptions = R.when(
  R.both(
    R.propEq('create', true),
    R.either(
      R.compose(
        R.either(R.compose(R.not, R.is(String)), R.either(R.isEmpty, R.isNil)),
        R.prop('project'),
      ),
      R.compose(
        R.either(R.compose(R.not, R.is(String)), R.either(R.isEmpty, R.isNil)),
        R.prop('type'),
      ),
    ),
  ),
  throwControlledError(errors.start.createSyntaxError),
);

try {
  program
    .option('-b, --branch [branch]', 'Name of the base branch to use')
    .option('-n, --no-branch-issue', 'Do not create a branch with the issue name')
    .option('-c, --create', 'Create a new issue instead of searching for it')
    .option('-p, --project [project]', 'Name of the project where to create the issue')
    .option('-t, --type [type]', 'Type of the issue to be created')
    .parse(process.argv);
  validateOptions(program);
  config.update(['git', 'branch'], program.branch, true);
  const isCreating = R.propEq('create', true);
  const issueIdOrDescription = getMainArgument(
    isCreating ? validateIssueDescription : validateIssueId,
  )(program);
  const { step, stepCurried } = reporter.stepFactory(program.branchIssue ? 4 : 3);
  step(1, 'Initializing services', 'rocket');
  init(config, program)
    .then(({ git, issueTracker }) =>
      issueTracker
        .getCurrentUser()
        .then(
          stepCurried(
            2,
            isCreating(program)
              ? `Creating issue in ${issueTracker.name}`
              : `Getting '${issueIdOrDescription}' from ${issueTracker.name}`,
            'bug',
          ),
        )
        .then(user =>
          R.ifElse(
            isCreating,
            R.compose(
              R.apply(issueTracker.createIssue),
              R.converge(R.unapply(R.concat(R.__, [issueIdOrDescription])), [
                R.prop('project'),
                R.prop('type'),
              ]),
            ),
            () =>
              issueTracker.getIssue(issueIdOrDescription).then(issueTracker.canWorkOnIssue(user)),
          )(program)
            .then(stepCurried(3, `Setting '${issueIdOrDescription}' in progress`, 'bookmark'))
            .then(issueTracker.setIssueStatus({ status: issueTracker.status.IN_PROGRESS }))
            .then(R.compose(wrapInPromise, git.createBranchName(config)))
            .then(
              R.ifElse(
                R.partial(R.propEq('branchIssue', true), [program]),
                R.compose(
                  git.createIssueBranch(config),
                  stepCurried(4, name => `Creating branch '${name}'`, 'tada'),
                ),
                R.always(undefined),
              ),
            ),
        ),
    )
    .then(reporter.footer)
    .catch(handleError);
} catch (e) {
  handleError(e);
  program.help();
}
