import R from 'ramda';
import { wrapInPromise } from '../../util';
import reporter from '../../reporter';

export const status = {
  BACKLOG: 'BACKLOG',
  IN_PROGRESS: 'IN_PROGRESS',
  IN_REVIEW: 'IN_PREVIEW',
  SELECTED_FOR_DEVELOPMENT: 'SELECTED_FOR_DEVELOPMENT'
};

export default R.curryN(2, (config, issue) => {
  const askForStatus = R.composeP(
    config.update(['jira', 'status']),
    R.compose(
      wrapInPromise,
      ([BACKLOG, SELECTED_FOR_DEVELOPMENT, IN_PROGRESS, IN_REVIEW]) => ({
        BACKLOG, SELECTED_FOR_DEVELOPMENT, IN_PROGRESS, IN_REVIEW
      })
    ),
    R.compose(wrapInPromise, JSON.parse),
    R.compose(wrapInPromise, R.concat('['), R.concat(R.__, ']')),
    R.compose(
      reporter.question,
      R.always({
        question: 'What are the ids for the transitions that represent\n' +
        'Backlog, to do, in progress, in review (enter a comma separated list)?'
      }),
      reporter.info,
      R.concat('These are the possible transitions found for the issue:\n'),
    )
  );

  return R.ifElse(
    R.compose(
      R.either(R.isNil, R.isEmpty), R.invoker(1, 'get')(['jira', 'status'])
    ),
    R.partial(R.compose(
      askForStatus,
      R.join('\n'),
      R.map(({ name, id }) => ` * ${name}: ${id}`),
      R.map(R.prop('to')),
      R.prop('transitions')
    ), [issue]),
    R.compose(wrapInPromise, R.invoker(1, 'get')(['jira', 'status']))
  )(config);
});

