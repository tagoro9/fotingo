/* eslint-disable max-len */
export default {
  config: {
    malformedFile: 'It looks like your config file is not correct. If you have modified it, make sure it is a valid JSON',
  },
  git: {
    branchAlreadyExists: 'It looks like you already have a branch for this issue.',
    branchNotFound: "We could not find the branch '{branch}'. Make sure the name is written correcly",
    couldNotInitializeRepo: "We have problems accessing the repo in '{pathToRepo}'. Make sure it exists.",
    noChanges: "You haven't made any changes to this branch",
    noIssueInBranchName: "We couldn't find the issue name on the branch. Maybe you should run `fotingo review -n`.",
    noSshKey: "It looks like you haven't added you ssh key. Remember to `ssh-add -k path_to_private_key` so we can communicate with the remote repository.",
  },
  github: {
    cantConnect: "We can' connect to github right now. Try in a few moments",
    invalidLabel: 'One of the labels you typed does not exist.',
    pullRequestDescriptionInvalid: 'The description provided for the pull request is not valid',
    tokenInvalid: 'The provided Github token is not valid',
    pullRequestAlreadyExists: "A pull request already exists for '{name}'",
  },
  jira: {
    cantWorkOnIssue: "It looks like you can't work on this issue right now. It may have already been fixed or someone else may be working on it.",
    issueIdNotValid: 'The name of this issue is not valid. Make sure your input is correct.',
    issueNotFound: 'We have problems finding that issue in Jira',
    userNotLoggedIn: 'User is not logged in',
  },
};
/* eslint-enable max-len */
