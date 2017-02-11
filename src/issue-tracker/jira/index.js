import R from 'ramda';
import httpClient from '../http-client';
import { debug, debugCurried, debugCurriedP, wrapInPromise } from '../../util';
import reporter from '../../reporter';
import { errors, catchPromiseAndThrow } from '../../error';
import issueUtil from './issue';

export default config => () => {
  debug('jira', 'Initializing Jira api');
  const root = config.get(['jira', 'root'])
    ? wrapInPromise(config.get(['jira', 'root']))
    : R.composeP(config.update(['jira', 'root']), reporter.question)({ question: 'What\'s your jira root?' });
  const statusPromise = R.either(R.isNil, R.isEmpty)(config.get(['jira', 'status']))
    ? R.composeP(
        config.update(['jira', 'status']),
        R.compose(
          wrapInPromise,
          ([BACKLOG, SELECTED_FOR_DEVELOPMENT, IN_PROGRESS, IN_REVIEW]) => ({
            BACKLOG, SELECTED_FOR_DEVELOPMENT, IN_PROGRESS, IN_REVIEW
          })
        ),
        R.compose(wrapInPromise, JSON.parse),
        R.compose(wrapInPromise, R.concat('['), R.concat(R.__, ']')),
        reporter.question
    )({ question: 'What are your jira step ids (Backlog, to do, in progress, in review)? Enter a comma separated list' })
    : wrapInPromise(config.get(['jira', 'status']));

  return Promise.all([root, statusPromise]).then(([jiraRoot, status]) => {
    const { get, post, setAuth } = httpClient(jiraRoot);
    const issueRoot = `${jiraRoot}/browse/`;
    const readUserInfo = () => {
      // It doesn't feel like logger should be used here
      debug('jira', 'Reading user login info');
      const readUsernamePromise = reporter.question({ question: 'What\'s your Jira username?' });
      const readPasswordPromise = readUsernamePromise.then(
        R.partial(reporter.question, [{ question: 'What\'s your Jira password?', password: true }])
      );
      return Promise.all([readUsernamePromise, readPasswordPromise]).then(([login, password]) => ({ login, password }));
    };

    const getCurrentUser = () => get('/rest/api/2/myself?expand=groups').then(R.prop('body'));

    const doLogin = R.composeP(getCurrentUser, setAuth, config.update(['jira', 'user']), readUserInfo);
    let loginPromise;
    if (config.isJiraLoggedIn()) {
      setAuth(config.get(['jira', 'user']));
      loginPromise = getCurrentUser().catch(R.compose(
        doLogin,
        debugCurried('jira', 'Current authentication failed. Attempting login')
      ));
    } else {
      loginPromise = doLogin();
    }

    const parseIssue = R.compose(
      wrapInPromise,
      R.converge(R.set(R.lensProp('url')), [
        R.compose(R.concat(issueRoot), R.prop('key')),
        R.identity
      ]),
      R.prop('body')
    );

    const addCommentToIssue = R.curry((issue, comment) =>
      post(`/rest/api/2/issue/${issue.key}/comment`, { body: { body: comment } })
    );

    return loginPromise.then((user) => ({
      name: 'jira',
      issueRoot,
      getCurrentUser: R.always(wrapInPromise(user)),
      getIssue: R.composeP(
        parseIssue,
        R.compose(
          catchPromiseAndThrow('jira', errors.jira.issueNotFound),
          get,
          R.concat(R.__, '?expand=transitions'),
          R.concat('/rest/api/2/issue/')
        ),
        debugCurriedP('jira', 'Getting issue from jira')
      ),
      setIssueStatus: R.curryN(2, ({ status: issueStatus, comment }, issue) => R.composeP(
        R.always(issue),
        // Jira api for transition is not adding the comment so we need an extra api call
        R.partial(R.unless(R.isNil, addCommentToIssue(issue)), [comment]),
        post(`/rest/api/2/issue/${issue.key}/transitions`),
        debugCurriedP('jira', `Updating issue status to ${issueStatus}`)
        )({
          body: {
            transition: R.compose(
              R.pick(['id']),
              R.find(R.compose(R.equals(issueStatus), Number, R.path(['to', 'id']))),
              R.prop('transitions')
            )(issue),
            fields: {},
          }
        })
      ),
      canWorkOnIssue: issueUtil(status).canWorkOnIssue,
      status
    }));
  });
};
