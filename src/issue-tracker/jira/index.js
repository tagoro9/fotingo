import R from 'ramda';
import httpClient from '../http-client';
import {
  debug,
  debugCurried,
  debugCurriedP,
  wrapInPromise,
  allowUserToEditMessage,
} from '../../util';
import reporter from '../../reporter';
import { errors, catchPromiseAndThrow, throwControlledError } from '../../error';
import issueUtil from './issue';
import readIssueStatus, { status } from './status';

export default config => () => {
  debug('jira', 'Initializing Jira api');
  const root = config.get(['jira', 'root'])
    ? wrapInPromise(config.get(['jira', 'root']))
    : R.composeP(config.update(['jira', 'root']), reporter.question)({
        question: "What's your jira root?",
      });

  return root.then(jiraRoot => {
    const { get, post, put, setAuth } = httpClient(jiraRoot);
    const issueRoot = `${jiraRoot}/browse/`;
    const readUserInfo = () => {
      // It doesn't feel like logger should be used here
      debug('jira', 'Reading user login info');
      const readUsernamePromise = reporter.question({ question: "What's your Jira username?" });
      const readPasswordPromise = readUsernamePromise.then(
        R.partial(reporter.question, [{ question: "What's your Jira password?", password: true }]),
      );
      return Promise.all([readUsernamePromise, readPasswordPromise]).then(([login, password]) => ({
        login,
        password,
      }));
    };

    const getCurrentUser = () => get('/rest/api/2/myself?expand=groups').then(R.prop('body'));

    const getIssueTypes = () => get('/rest/api/2/issuetype').then(R.prop('body'));

    const getShortName = name => {
      if (name.match(/feature|story/i)) {
        return 'f';
      }
      if (name.match(/task/i)) {
        return 'c';
      }
      return name[0].toLowerCase();
    };

    const initialize = user =>
      R.composeP(
        R.always(user),
        R.compose(
          R.ifElse(
            R.compose(R.either(R.isNil, R.isEmpty)),
            R.composeP(
              R.compose(
                wrapInPromise,
                config.update(['jira', 'issueTypes']),
                R.mergeAll,
                R.map(({ id, name }) => ({ [id]: { name, shortName: getShortName(name) } })),
              ),
              getIssueTypes,
            ),
            R.compose(wrapInPromise, R.identity),
          ),
          config.get,
        ),
      )(['jira', 'issueTypes']);

    const doLogin = R.composeP(
      R.compose(catchPromiseAndThrow('jira', errors.jira.couldNotAuthenticate), getCurrentUser),
      setAuth,
      config.update(['jira', 'user']),
      readUserInfo,
    );
    let loginPromise;
    if (config.isJiraLoggedIn()) {
      setAuth(config.get(['jira', 'user']));
      loginPromise = getCurrentUser().catch(
        R.compose(doLogin, debugCurried('jira', 'Current authentication failed. Attempting login')),
      );
    } else {
      loginPromise = doLogin();
    }

    const parseIssue = R.compose(
      wrapInPromise,
      R.converge(R.set(R.lensProp('url')), [
        R.compose(R.concat(issueRoot), R.prop('key')),
        R.identity,
      ]),
      R.prop('body'),
    );

    const addCommentToIssue = R.curry((issue, comment) =>
      post(`/rest/api/2/issue/${issue.key}/comment`, { body: { body: comment } }),
    );

    return loginPromise.then(initialize).then(user => {
      const jira = {
        name: 'jira',
        issueRoot,
        getCurrentUser: R.always(wrapInPromise(user)),
        getIssue: R.composeP(
          parseIssue,
          R.compose(
            catchPromiseAndThrow('jira', errors.jira.issueNotFound),
            get,
            R.concat(R.__, '?expand=transitions'),
            R.concat('/rest/api/2/issue/'),
          ),
          debugCurriedP('jira', 'Getting issue from jira'),
        ),
        setIssueStatus: R.curryN(2, ({ status: issueStatus, comment }, issue) =>
          R.composeP(
            R.always(issue),
            // Jira api for transition is not adding the comment so we need an extra api call
            R.partial(R.unless(R.isNil, addCommentToIssue(issue)), [comment]),
            post(`/rest/api/2/issue/${issue.key}/transitions`),
            debugCurriedP('jira', `Updating issue status to ${issueStatus}`),
            statuses =>
              wrapInPromise({
                body: {
                  transition: R.compose(
                    R.pick(['id']),
                    R.find(
                      R.compose(R.equals(statuses[issueStatus]), Number, R.path(['to', 'id'])),
                    ),
                    R.prop('transitions'),
                  )(issue),
                  fields: {},
                },
              }),
            readIssueStatus(config),
          )(issue),
        ),
        canWorkOnIssue: R.curryN(
          2,
          R.converge((canWonOnIssue, promise) => promise.then(canWonOnIssue), [
            R.curryN(2, R.invoker(2, 'canWorkOnIssue')),
            R.composeP(R.compose(wrapInPromise, issueUtil), R.flip(readIssueStatus(config))),
          ]),
        ),
        // String -> Array -> String
        createReleaseNotes: R.curryN(
          2,
          R.composeP(
            R.ifElse(R.isEmpty, throwControlledError(errors.jira.releaseNotesInvalid), notes => ({
              title: R.compose(R.head, R.split('\n'), R.trim)(notes),
              body: R.compose(
                R.join('\n'),
                R.filter(R.compose(R.not, R.isEmpty)),
                R.tail,
                R.split('\n'),
                R.trim,
              )(notes),
            })),
            allowUserToEditMessage(`/tmp/fotingo-notes-file-${Date.now()}`),
            R.compose(
              wrapInPromise,
              R.converge(R.unapply(R.join('\n')), [
                R.compose(
                  R.concat(R.__, '\n'),
                  R.ifElse(
                    R.compose(R.equals(1), R.length, R.nthArg(1)),
                    R.compose(R.view(R.lensPath(['fields', 'summary'])), R.head, R.nthArg(1)),
                    R.nthArg(0),
                  ),
                ),
                R.compose(
                  R.join('\n'),
                  R.map(issue => `* [${issue.key}](${issue.url}). ${issue.fields.summary}.`),
                  R.nthArg(1),
                ),
              ]),
            ),
          ),
        ),
        createVersion: R.curryN(2, (releaseId, issues) =>
          R.composeP(
            R.prop('body'),
            post('/rest/api/2/version'),
            R.set(R.lensPath(['body', 'projectId']), R.__, {
              body: {
                archived: false,
                released: true,
                releaseDate: new Date().toISOString().substring(0, 10),
                name: releaseId,
                description: 'Release from fotingo',
              },
            }),
            R.head,
            R.when(
              R.compose(R.lt(1), R.length, R.uniq),
              throwControlledError(errors.jira.issuesInMultipleProjects),
            ),
            R.map(R.view(R.lensPath(['fields', 'project', 'id']))),
            wrapInPromise,
          )(issues),
        ),
        setIssuesFixVersion: R.curryN(2, (issues, version) => {
          return R.compose(
            promises => Promise.all(promises),
            R.map(
              R.converge((...promises) => Promise.all(promises), [
                R.compose(
                  put(R.__, {
                    body: {
                      update: {
                        fixVersions: [
                          {
                            add: { name: version.name },
                          },
                        ],
                      },
                    },
                  }),
                  R.concat('/rest/api/2/issue/'),
                  R.prop('key'),
                ),
                jira.setIssueStatus({ status: status.DONE }),
              ]),
            ),
          )(issues);
        }),
        status,
      };
      return jira;
    });
  });
};
