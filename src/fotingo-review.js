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
    .option('-b, --branch [branch]', 'Name of the base branch of the pull request')
    .option('-l, --label [label]', 'Label to add to the PR', R.append, [])
    .option('-n, --no-branch-issue', 'Do not pick issue from branch name')
    .option('-r, --reviewer [reviewer]', 'Request some people to review your PR', R.append, [])
    .option('-s, --simple', 'Do not use any issue tracker')
    .parse(process.argv);

  config.update(['github', 'base'], program.branch, true);
  const shouldGetIssue = R.partial(
    R.both(
      R.compose(R.equals(true), R.prop('branchIssue')),
      R.compose(R.not, R.equals(true), R.prop('simple')),
    ),
    [program],
  );
  const getTotalSteps = R.ifElse(
    R.propEq('simple', true),
    R.always(4),
    R.ifElse(R.propEq('branchIssue', false), R.always(5), R.always(7)),
  );
  const { step, stepCurried, stepCurriedP } = reporter.stepFactory(getTotalSteps(program));
  const stepOffset = shouldGetIssue() ? 0 : 2;
  const project = getProject(process.cwd());
  step(1, 'Initializing services', 'rocket');
  init(config, program)
    .then(({ git, github, issueTracker }) =>
      git
        .getCurrentBranchName()
        .then(
          R.compose(
            wrapInPromise,
            stepCurried(2, name => `Pushing '${name}' to Github`, 'arrow_up'),
          ),
        )
        .then(R.partial(git.pushBranchToGithub, [config]))
        .then(
          R.ifElse(
            shouldGetIssue,
            R.composeP(
              issueTracker.getIssue,
              stepCurriedP(4, issue => `Getting '${issue}' from ${issueTracker.name}`, 'bug'),
              git.extractIssueFromCurrentBranch,
              stepCurriedP(3, 'Extracting issue from branch', 'mag_right'),
            ),
            R.always(undefined),
          ),
        )
        .then((issue) => {
          step(5 - stepOffset, 'Getting your commit history', 'books');
          return git
            .getBranchInfo(config, issue)
            .then(step(6 - stepOffset, 'Creating pull request', 'speaker'))
            .then(github.checkAndGetLabels(config, project, program.label))
            .then(R.set(R.lensProp('reviewers'), program.reviewer))
            .then(
              github.createPullRequest(config, project, issue, issueTracker.issueRoot, {
                addLinksToIssues: !program.simple,
              }),
            );
        })
        .then(
          R.ifElse(
            R.partial(R.compose(R.not, R.propEq('simple', true)), [program]),
            ({ branchInfo: { issues }, pullRequest }) =>
              R.compose(
                promise => promise.then(R.always(wrapInPromise({ pullRequest }))),
                promises => Promise.all(promises),
                R.map(
                  R.composeP(
                    R.partial(issueTracker.setIssueStatus, [
                      {
                        status: issueTracker.status.IN_REVIEW,
                        comment: `PR: ${pullRequest.html_url}`,
                      },
                    ]),
                    issueTracker.getIssue,
                    R.compose(wrapInPromise, R.prop('issue')),
                  ),
                ),
                stepCurried(
                  7 - stepOffset,
                  // eslint-disable-next-line max-len
                  `Setting ${R.compose(R.join(', '), R.map(R.prop('issue')))(issues)} to code review on ${issueTracker.name}`,
                  'bookmark',
                ),
              )(issues),
            R.identity,
          ),
        ))
    .then(
      R.compose(
        reporter.footer,
        R.ifElse(R.isNil, R.identity, R.path(['pullRequest', 'html_url'])),
      ),
    )
    .catch(handleError);
} catch (e) {
  handleError(e);
  program.help();
}
