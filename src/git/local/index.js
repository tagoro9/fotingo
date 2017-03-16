import Git from 'nodegit';
import R from 'ramda';
import conventionalCommitsParser from 'conventional-commits-parser';
import { catchPromiseAndThrow, throwControlledError, errors } from '../../error';
import { createBranchName, getIssueIdFromBranch } from '../util';
import { debug, debugCurried, debugCurriedP, wrapInPromise } from '../../util';
import app from '../../../package.json';

const fetchOptions = {
  callbacks: {
    certificateCheck: R.always(1),
    credentials: R.compose(
      Git.Cred.sshKeyFromAgent,
      debugCurried('git', 'Getting authentication from SSH agent'),
      // TODO Detect ssh key not present
      R.nthArg(1)
    )
  }
};

let repository = null;

const getCurrentBranchName = R.composeP(
  (ref) => Git.Branch.name(ref),
  () => repository.head(),
);

// Commit -> Object
const transformCommit = R.compose(
  conventionalCommitsParser.sync,
  R.invoker(0, 'message')
);

// Object -> Array -> Array
const getIssues = R.converge(R.concat, [
  R.compose(R.ifElse(R.isNil,
    R.always([]),
    ({ key }) => ([{ raw: `#${key}`, issue: key }])
  ), R.nthArg(0)),
  R.compose(R.flatten, R.map(R.compose(
    R.map(ref => ({ raw: `${ref.prefix}${ref.issue}`, issue: ref.issue })),
    R.prop('references'))),
    R.nthArg(1)
  )
]);

export default {
  init: (config, pathToRepo) => () => {
    debug('git', `Initializing ${pathToRepo} repository`);
    return Git.Repository.open(pathToRepo).then((repo) => {
      repository = repo;
      return Promise.resolve(this);
    })
      .catch(throwControlledError(errors.git.couldNotInitializeRepo, { pathToRepo }));
  },
  createBranchName,
  createIssueBranch: R.curryN(2, (config, name) => {
    debug('git', 'Creating branch for issue');
    const { remote, branch } = config.get(['git']);
    debug('git', 'Fetching data from remote');
    // We should fetch -> co master -> reset to origin/master -> create branch
    return repository.fetch(remote, fetchOptions)
      .then(debugCurriedP('git', 'Getting local repository status'))
      .then(() => repository.getStatus())
      .then(R.ifElse(
        R.isEmpty,
        R.identity,
        () => Git.Stash.save(
          debugCurried('git', 'Stashing changes', repository),
          repository.defaultSignature(),
          `auto generated stash by ${app.name}`,
          Git.Stash.FLAGS.INCLUDE_UNTRACKED
        )
      ))
      .then(() => repository.getBranchCommit(`${remote}/${branch}`))
      .then(debugCurriedP('git', 'Creating new branch'))
      .then(R.compose(
        catchPromiseAndThrow('git', errors.git.branchAlreadyExists),
        (commit) => repository.createBranch(name, commit)
      ))
      .then(() => repository.checkoutBranch(name));
  }),
  getCurrentBranchName,
  pushBranchToGithub: R.converge(
    R.composeP(([remote, branch, { branchName, ref }]) => remote.push([ref], fetchOptions)
        .then(() => Git.Branch.setUpstream(branch, `${remote.name()}/${branchName}`)),
    (...promises) => Promise.all(promises)),
    [
      R.compose(
        remote => Git.Remote.lookup(repository, remote),
        R.prop('remote'),
        R.invoker(1, 'get')(['git'])
      ),
      () => repository.head(),
      () => getCurrentBranchName().then(name => ({ branchName: name, ref: `refs/heads/${name}:refs/heads/${name}` }))
    ]
  ),
  extractIssueFromCurrentBranch: () =>
    R.composeP(
      debugCurriedP('git', 'Extracting issue from current branch'),
      R.compose(
        wrapInPromise,
        R.when(R.isNil, throwControlledError(errors.git.noIssueInBranchName)),
        getIssueIdFromBranch
      ),
      getCurrentBranchName
    )(),
  getBranchInfo(config, issue) {
    debug('git', 'Getting branch commit history');
    const { remote, branch } = config.get(['git']);
    return Promise.all([
      repository.getHeadCommit(),
      repository.getBranchCommit(`${remote}/${branch}`)
    ]).then(R.when(
      R.compose(R.not, R.allUniq, R.map(R.compose(R.toString, R.invoker(0, 'id')))),
      throwControlledError(errors.git.noChanges))
    )
      .then(([latestCommit, latestMasterCommit]) =>
        Promise.all([
          Git.Merge.base(repository, latestCommit, latestMasterCommit)
            .then(latestCommonCommit => {
              debug('git', `Created history walker. Latest common commit: ${latestCommonCommit}`);
              const historyWalker = repository.createRevWalk();
              const commitStopper = (commit) => !(latestCommonCommit.equal(commit.id()));
              historyWalker.push(latestCommit);
              return historyWalker.getCommitsUntil(commitStopper);
            })
            .then(commits => R.compose(R.reverse, R.map(transformCommit), R.init)(commits)),
          getCurrentBranchName()
        ])
      .then(([commits, name]) => ({
        name,
        commits,
        issues: getIssues(issue, commits)
      })));
  }
};
