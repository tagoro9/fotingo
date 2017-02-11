import R from 'ramda';
import git from './git/local';
import github from './git/github';
import getIssueTracker from './issue-tracker';

// config -> project -> promise
export default R.curryN(2, (c, program) =>
  github.init(c)()
    .then(git.init(c, process.cwd()))
    .then(getIssueTracker(program)(c))
    .then(issueTracker => ({github, git, issueTracker}))
);
