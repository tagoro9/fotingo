import program from 'commander';
import R from 'ramda';

import { getProject } from './git/util';
import { handleError, throwControlledError, errors } from './error';
import config from './config';
import init from './init';
import reporter from './reporter';
import { wrapInPromise } from './util';

const getReleaseId = R.compose(
  R.when(R.isNil, throwControlledError(errors.jira.releaseNameRequired)),
  R.head,
  R.prop('args'),
);

try {
  program
    .option('-n, --no-branch-issue', 'Do not pick issue from the branch name')
    .option('-i, --issue [issue]', 'Specify more issues to include in the release', R.append, [])
    .parse(process.argv);

  const releaseId = getReleaseId(program);
  const shouldGetIssue = R.compose(R.equals(true), R.prop('branchIssue'))(program);
  const project = getProject(process.cwd());

  const { step, stepCurried, stepCurriedP } = reporter.stepFactory(5);
  step(1, 'Initializing services', 'rocket');
  R.composeP(
    reporter.footer,
    ({ git, github, issueTracker }) =>
      R.composeP(
        ({ issues, notes }) =>
          R.composeP(
            github.createOrUpdateRelease(config, project, notes, releaseId),
            stepCurriedP(5, `Creating release ${releaseId} in github`, 'ship'),
            issueTracker.setIssuesFixVersion(issues),
            stepCurried(4, 'Setting the fix version to the affected issues', 'pencil2'),
            issueTracker.createVersion(releaseId),
          )(issues, notes),
        stepCurried(3, `Creating release ${releaseId} in ${issueTracker.name}`, 'ship'),
        R.apply(R.merge),
        R.converge((...promises) => Promise.all(promises), [
          R.compose(wrapInPromise, R.set(R.lensProp('issues'), R.__, {})),
          R.composeP(
            R.set(R.lensProp('notes'), R.__, {}),
            issueTracker.createReleaseNotes(releaseId),
          ),
        ]),
        promises => Promise.all(promises),
        R.map(issueTracker.getIssue),
        issues =>
          stepCurried(2, `Getting ${issues.join(', ')} from ${issueTracker.name}`, 'bug', issues),
        git.getIssuesInBranch(config, program.issue, shouldGetIssue),
      )(),
    init,
  )(config, program).catch(handleError);
} catch (e) {
  handleError(e);
  program.help();
}
