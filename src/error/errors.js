/* eslint-disable max-len */
export default {
  git: {
    couldNotInitializeRepo: 'We have problems accessing the repo in \'${pathToRepo}\'. Make sure it exists.',
    noChanges: 'You haven\'t made any changes to this branch'
  },
  github: {
    cantConnect: 'We can\' connect to github right now. Try in a few moments',
    pullRequestDescriptionInvalid: 'The description provided for the pull request is not valid',
    tokenInvalid: 'The provided Github token is not valid'
  },
  jira: {
    cantWorkOnIssue: 'It looks like you can\'t work on this issue right now. It may have already been fixed or someone else may be working on it.',
    issueIdNotValid: 'The name of this issue is not valid. Make sure your input is correct.',
    issueNotFound: 'We have problems finding that issue in Jira',
    userNotLoggedIn: 'User is not logged in'
  }
};
/* eslint-enable max-len */
