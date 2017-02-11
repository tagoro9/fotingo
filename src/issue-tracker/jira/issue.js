import R from 'ramda';
import { throwControlledError, errors } from '../../error';

export default status => {
  // issue -> boolean
  const isInNoGroup = R.compose(R.isEmpty, R.path(['fields', 'labels']));
  // issue -> login -> Boolean
  const isInUserGroup = R.converge(
    (groups, labels) => R.any(R.contains(R.__, groups), labels),
    [
      R.compose(R.map(R.prop('name')), R.path(['groups', 'items']), R.nthArg(1)),
      R.compose(R.map(R.replace('team-', '')), R.path(['fields', 'labels']), R.nthArg(0))
    ]
  );
  // issue -> login -> Boolean
  const isAssignedToUser = (issue, user) => R.pathEq(['fields', 'assignee', 'key'], user.key, issue);
  // issue -> Boolean
  const isUnassigned = R.pathSatisfies(R.isNil, ['fields', 'assignee']);
  // issue -> Boolean
  const hasValidStatus = R.compose(
    R.contains(R.__, [status.SELECTED_FOR_DEVELOPMENT, status.BACKLOG]), Number, R.path(['fields', 'status', 'id'])
  );

  return {
    // user -> issue -> issue
    canWorkOnIssue: R.curryN(2, R.flip(
      R.ifElse(
        R.either(
          isAssignedToUser,
          R.both(
            R.both(hasValidStatus, isUnassigned),
            R.either(isInUserGroup, isInNoGroup)
          )
        ),
        R.nthArg(0, Promise.resolve),
        throwControlledError(errors.jira.cantWorkOnIssue)
      ))
    )
  };
};
