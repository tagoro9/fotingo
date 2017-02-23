'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});

var _slicedToArray = function () { function sliceIterator(arr, i) { var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"]) _i["return"](); } finally { if (_d) throw _e; } } return _arr; } return function (arr, i) { if (Array.isArray(arr)) { return arr; } else if (Symbol.iterator in Object(arr)) { return sliceIterator(arr, i); } else { throw new TypeError("Invalid attempt to destructure non-iterable instance"); } }; }();

var _nodegit = require('nodegit');

var _nodegit2 = _interopRequireDefault(_nodegit);

var _ramda = require('ramda');

var _ramda2 = _interopRequireDefault(_ramda);

var _conventionalCommitsParser = require('conventional-commits-parser');

var _conventionalCommitsParser2 = _interopRequireDefault(_conventionalCommitsParser);

var _error = require('../../error');

var _util = require('../util');

var _util2 = require('../../util');

var _package = require('../../../package.json');

var _package2 = _interopRequireDefault(_package);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

var fetchOptions = {
  callbacks: {
    certificateCheck: _ramda2['default'].always(1),
    credentials: _ramda2['default'].compose(_nodegit2['default'].Cred.sshKeyFromAgent, (0, _util2.debugCurried)('git', 'Getting authentication from SSH agent'),
    // TODO Detect ssh key not present
    _ramda2['default'].nthArg(1))
  }
};

var repository = null;

var getCurrentBranchName = _ramda2['default'].composeP(function (ref) {
  return _nodegit2['default'].Branch.name(ref);
}, function () {
  return repository.head();
});

// Commit -> Object
var transformCommit = _ramda2['default'].compose(_conventionalCommitsParser2['default'].sync, _ramda2['default'].invoker(0, 'message'));

// Object -> Array -> Array
var getIssues = _ramda2['default'].converge(_ramda2['default'].concat, [_ramda2['default'].compose(_ramda2['default'].ifElse(_ramda2['default'].isNil, _ramda2['default'].always([]), function (_ref) {
  var key = _ref.key;
  return [{ raw: '#' + key, issue: key }];
}), _ramda2['default'].nthArg(0)), _ramda2['default'].compose(_ramda2['default'].flatten, _ramda2['default'].map(_ramda2['default'].compose(_ramda2['default'].map(_ramda2['default'].pick(['raw', 'issue'])), _ramda2['default'].prop('references'))), _ramda2['default'].nthArg(1))]);

exports['default'] = {
  init: function () {
    function init(config, pathToRepo) {
      return function () {
        (0, _util2.debug)('git', 'Initializing ' + pathToRepo + ' repository');
        return _nodegit2['default'].Repository.open(pathToRepo).then(function (repo) {
          repository = repo;
          return Promise.resolve(undefined);
        })['catch']((0, _error.throwControlledError)(_error.errors.git.couldNotInitializeRepo, { pathToRepo: pathToRepo }));
      };
    }

    return init;
  }(),
  createIssueBranch: _ramda2['default'].curryN(2, function (config, issue) {
    (0, _util2.debug)('git', 'Creating branch for issue');
    var name = (0, _util.createBranchName)(issue);

    var _config$get = config.get(['git']),
        remote = _config$get.remote,
        branch = _config$get.branch;

    (0, _util2.debug)('git', 'Fetching data from remote');
    // We should fetch -> co master -> reset to origin/master -> create branch
    return repository.fetch(remote, fetchOptions).then((0, _util2.debugCurriedP)('git', 'Getting local repository status')).then(function () {
      return repository.getStatus();
    }).then(_ramda2['default'].ifElse(_ramda2['default'].isEmpty, _ramda2['default'].identity, function () {
      return _nodegit2['default'].Stash.save((0, _util2.debugCurried)('git', 'Stashing changes', repository), repository.defaultSignature(), 'auto generated stash by ' + _package2['default'].name, _nodegit2['default'].Stash.FLAGS.INCLUDE_UNTRACKED);
    })).then(function () {
      return repository.getBranchCommit(remote + '/' + branch);
    }).then((0, _util2.debugCurriedP)('git', 'Creating new branch')).then(_ramda2['default'].compose((0, _error.catchPromiseAndThrow)('git', _error.errors.git.branchAlreadyExists), function (commit) {
      return repository.createBranch(name, commit);
    })).then(function () {
      return repository.checkoutBranch(name);
    });
  }),
  pushBranchToGithub: _ramda2['default'].converge(_ramda2['default'].composeP(function (_ref2) {
    var _ref3 = _slicedToArray(_ref2, 3),
        remote = _ref3[0],
        branch = _ref3[1],
        _ref3$ = _ref3[2],
        branchName = _ref3$.branchName,
        ref = _ref3$.ref;

    return remote.push([ref], fetchOptions).then(function () {
      return _nodegit2['default'].Branch.setUpstream(branch, remote.name() + '/' + branchName);
    });
  }, function () {
    for (var _len = arguments.length, promises = Array(_len), _key = 0; _key < _len; _key++) {
      promises[_key] = arguments[_key];
    }

    return Promise.all(promises);
  }), [_ramda2['default'].compose(function (remote) {
    return _nodegit2['default'].Remote.lookup(repository, remote);
  }, _ramda2['default'].prop('remote'), _ramda2['default'].invoker(1, 'get')(['git'])), function () {
    return repository.head();
  }, function () {
    return getCurrentBranchName().then(function (name) {
      return { branchName: name, ref: 'refs/heads/' + name + ':refs/heads/' + name };
    });
  }]),
  extractIssueFromCurrentBranch: function () {
    function extractIssueFromCurrentBranch() {
      return _ramda2['default'].composeP((0, _util2.debugCurriedP)('git', 'Extracting issue from current branch'), _ramda2['default'].compose(_util2.wrapInPromise, _util.getIssueIdFromBranch), getCurrentBranchName)();
    }

    return extractIssueFromCurrentBranch;
  }(),
  getBranchInfo: function () {
    function getBranchInfo(issue) {
      (0, _util2.debug)('git', 'Getting branch commit history');
      return Promise.all([repository.getHeadCommit(), repository.getBranchCommit('origin/master')]).then(_ramda2['default'].when(_ramda2['default'].compose(_ramda2['default'].not, _ramda2['default'].allUniq, _ramda2['default'].map(_ramda2['default'].compose(_ramda2['default'].toString, _ramda2['default'].invoker(0, 'id')))), (0, _error.throwControlledError)(_error.errors.git.noChanges))).then(function (_ref4) {
        var _ref5 = _slicedToArray(_ref4, 2),
            latestCommit = _ref5[0],
            latestMasterCommit = _ref5[1];

        return Promise.all([_nodegit2['default'].Merge.base(repository, latestCommit, latestMasterCommit).then(function (latestCommonCommit) {
          (0, _util2.debug)('git', 'Created history walker. Latest common commit: ' + latestCommonCommit);
          var historyWalker = repository.createRevWalk();
          var commitStopper = function () {
            function commitStopper(commit) {
              return !latestCommonCommit.equal(commit.id());
            }

            return commitStopper;
          }();
          historyWalker.push(latestCommit);
          return historyWalker.getCommitsUntil(commitStopper);
        }).then(function (commits) {
          return _ramda2['default'].compose(_ramda2['default'].reverse, _ramda2['default'].map(transformCommit), _ramda2['default'].init)(commits);
        }), getCurrentBranchName()]).then(function (_ref6) {
          var _ref7 = _slicedToArray(_ref6, 2),
              commits = _ref7[0],
              name = _ref7[1];

          return {
            name: name,
            commits: commits,
            issues: getIssues(issue, commits)
          };
        });
      });
    }

    return getBranchInfo;
  }()
};