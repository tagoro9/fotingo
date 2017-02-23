import program from 'commander';
import R from 'ramda';

import { getProject } from './git/util';
import config from './config';
import { handleError } from './error';
import init from './init';
import reporter from './reporter';
import { wrapInPromise } from './util';


try {
  program
    .option('-n, --no-branch-issue', 'Do not pick issue from branch name')
    .option('-s, --simple', 'Do not use any issue tracker')
    .parse(process.argv);

  const shouldGetIssue = R.partial(R.both(
    R.compose(R.equals(true), R.prop('branchIssue')),
    R.compose(R.not, R.equals(true), R.prop('simple'))
  ), [program]);
  const getTotalSteps = R.ifElse(
    R.propEq('simple', true),
    R.always(4),
    R.ifElse(R.propEq('branchIssue', false), R.always(5), R.always(7))
  );
  const { step, stepCurried } = reporter.stepFactory(getTotalSteps(program));
  const stepOffset = shouldGetIssue() ? 0 : 2;
  const project = getProject(process.cwd());
  step(1, 'Initializing services', 'rocket');
  init(config, program)
    .then(({ git, github, issueTracker }) =>
      wrapInPromise(step(2, 'Pushing current branch to Github', 'arrow_up'))
        .then(R.partial(git.pushBranchToGithub, [config]))
        .then(R.ifElse(
          shouldGetIssue,
          R.compose(
            issueTracker.getIssue,
            stepCurried(4, `Getting issue from ${issueTracker.name}`, 'bug'),
            git.extractIssueFromCurrentBranch,
            stepCurried(3, 'Extracting issue from current branch', 'mag_right')
          ),
          R.always(undefined),
        ))
        .then(issue => {
          step(5 - stepOffset, 'Getting your commit history', 'books');
          return git.getBranchInfo(issue)
            .then(step(6 - stepOffset, 'Creating pull request', 'speaker'))
            .then(github.createPullRequest(config, project, issue, issueTracker.issueRoot));
        })
        .then(R.ifElse(
          R.partial(R.compose(R.not, R.propEq('simple', true)), [program]),
          ({ branchInfo: { issues }, pullRequest }) =>
            R.compose(
              promise => promise.then(R.always(wrapInPromise({ pullRequest }))),
              promises => Promise.all(promises),
              R.map(R.composeP(
                R.partial(
                  issueTracker.setIssueStatus,
                  [{ status: issueTracker.status.IN_REVIEW, comment: `PR: ${pullRequest.html_url}` }]
                ),
                issueTracker.getIssue,
                R.compose(wrapInPromise, R.prop('issue'))
              )),
              stepCurried(7 - stepOffset, `Setting issues to code review on ${issueTracker.name}`, 'bookmark'),
            )(issues),
          R.identity
        )))
    .then(R.compose(
      reporter.footer,
      R.ifElse(R.isNil, R.identity, R.path(['pullRequest', 'html_url']))
    ))
    .catch(handleError);
} catch (e) {
  handleError(e);
  program.help();
}
