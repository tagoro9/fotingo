import R from 'ramda';
import { wrapInPromise } from '../../util';
import reporter from '../../reporter';

export const status = {
  BACKLOG: 'BACKLOG',
  DONE: 'DONE',
  IN_PROGRESS: 'IN_PROGRESS',
  IN_REVIEW: 'IN_REVIEW',
  SELECTED_FOR_DEVELOPMENT: 'SELECTED_FOR_DEVELOPMENT',
};

const statusRegex = {
  BACKLOG: /backlog/i,
  IN_PROGRESS: /in progress/i,
  IN_REVIEW: /review/i,
  DONE: /done/i,
  SELECTED_FOR_DEVELOPMENT: /(todo)|(to do)|(selected for development)/i,
};

const statusMatcher = statusToFind =>
  R.compose(
    R.ifElse(R.isNil, R.identity, R.compose(parseInt, R.prop('id'))),
    R.find(R.compose(R.test(statusRegex[statusToFind]), R.prop('name'))),
  );

export default R.curryN(2, (config, issue = { transitions: [] }) => {
  const askForStatus = R.composeP(
    config.update(['jira', 'status']),
    R.compose(
      wrapInPromise,
      ([BACKLOG, SELECTED_FOR_DEVELOPMENT, IN_PROGRESS, IN_REVIEW, DONE]) => ({
        BACKLOG,
        SELECTED_FOR_DEVELOPMENT,
        IN_PROGRESS,
        IN_REVIEW,
        DONE,
      }),
    ),
    R.compose(wrapInPromise, JSON.parse),
    R.compose(wrapInPromise, R.concat('['), R.concat(R.__, ']')),
    R.compose(
      reporter.question,
      R.always({
        question: 'Please, insert a comma separated list of the ids',
      }),
      reporter.info,
      R.concat(
        R.__,
        "\n     We need you to input the ids of the 'Backlog', 'To do', 'In progress', 'In review' and 'Done'\n    " +
          " (one state can represent multiple values (e.g. 'Backlog' and 'To do' could be the same id).\n     " +
          'It all depends on your workflow.\n',
      ),
      R.concat(
        'In order to update the jira issues correctly, we need to know a little bit more about your workflow.\n     ' +
          'We need to identify the different states an issue can be on. We tried inferring those values, but we\n     ' +
          "were unable to do so. We are looking for 5 specific states: 'Backlog', 'To do', 'In progress',\n     " +
          "'In review' and 'Done'. These are the names and ids of the states the issue can transition to:\n     ",
      ),
    ),
  );

  const inferStatus = R.compose(
    R.ifElse(
      R.compose(R.any(R.isNil), R.values),
      R.compose(
        askForStatus,
        R.concat(R.__, '\n'),
        R.concat('\n'),
        R.join('\n'),
        R.map(({ name, id }) => `         * ${name}: ${id}`),
        R.prop('transitions'),
      ),
      R.compose(wrapInPromise, config.update(['jira', 'status']), R.omit(['transitions'])),
    ),
    transitions => ({
      [status.BACKLOG]:
        statusMatcher(status.BACKLOG)(transitions) ||
        statusMatcher(status.SELECTED_FOR_DEVELOPMENT)(transitions),
      [status.IN_PROGRESS]: statusMatcher(status.IN_PROGRESS)(transitions),
      [status.DONE]: statusMatcher(status.DONE)(transitions),
      [status.IN_REVIEW]: statusMatcher(status.IN_REVIEW)(transitions),
      [status.SELECTED_FOR_DEVELOPMENT]: statusMatcher(status.SELECTED_FOR_DEVELOPMENT)(
        transitions,
      ),
      transitions,
    }),
  );

  const anyStatusIsMissing = (configStatus = {}) =>
    R.compose(R.not, R.all(R.contains(R.__, R.keys(configStatus))), R.values)(status);

  return R.ifElse(
    R.compose(anyStatusIsMissing, R.invoker(1, 'get')(['jira', 'status'])),
    R.partial(R.compose(inferStatus, R.map(R.prop('to')), R.prop('transitions')), [issue]),
    R.compose(wrapInPromise, R.invoker(1, 'get')(['jira', 'status'])),
  )(config);
});
