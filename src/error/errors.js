/* eslint-disable max-len */
export default {
  config: {
    malformedFile:
      'It looks like your config file is not correct. If you have modified it, make sure it is a valid JSON',
  },
  git: {
    branchAlreadyExists: 'It looks like you already have a branch for this issue.',
    branchNotFound:
      "We could not find the branch '{branch}'. Make sure the name is written correcly",
    cantPush:
      "We have trouble pushing to the remote server. It looks like the remote branch '{branch}' is behind the local one.",
    couldNotInitializeRepo:
      "We have problems accessing the repo in '{pathToRepo}'. Make sure it exists.",
    noChanges: "You haven't made any changes to this branch",
    noIssueInBranchName:
      "We couldn't find the issue name on the branch. Maybe you should run `fotingo review -n`.",
    noSshKey:
      "It looks like you haven't added you ssh key. Remember to `ssh-add -k path_to_private_key` so we can communicate with the remote repository.",
    noIssues: "We couldn't find any issue to assciate with the release",
  },
  github: {
    cantConnect: "We can' connect to github right now. Try in a few moments",
    invalidLabel: 'One of the labels you typed does not exist.',
    pullRequestDescriptionInvalid: 'The description provided for the pull request is not valid',
    tokenInvalid: 'The provided Github token is not valid',
    pullRequestAlreadyExists: "A pull request already exists for '{name}'",
  },
  jira: {
    cantWorkOnIssue:
      "It looks like you can't work on this issue right now. It may have already been fixed or someone else may be working on it.",
    couldNotAuthenticate:
      'We have trouble authenticating with Jira. Make sure username and password are correct.',
    issueIdNotValid: 'The name of this issue is not valid. Make sure your input is correct.',
    issueNotFound: 'We have problems finding that issue in Jira',
    issueTypeNotFound: 'We could not find this issue type, make sure it exists',
    projectNotFound: 'We have problems finding that project in Jira',
    userNotLoggedIn: 'User is not logged in',
    issuesInMultipleProjects:
      'There are issues associated with different projects. A release can only contain issues associated with a project',
    releaseNotesInvalid: 'The release notes provided are not valid',
    releaseNameRequired: 'It looks like you did not specify a release name',
    issueDescriptionNotValid:
      'The description of this issue is not valid. Please make sure your input is correct',
  },
  start: {
    createSyntaxError:
      'In order to create a new issue you need to specify a project and an issue type',
  },
};
/* eslint-enable max-len */
