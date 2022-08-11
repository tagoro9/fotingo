# Fotingo

A CLI to ease the interaction between _Git_, _GitHub_ and _Jira_ when working on tasks.

[![Standard Version](https://img.shields.io/badge/release-standard%20version-brightgreen.svg)](https://github.com/conventional-changelog/standard-version)
![Scarf](https://static.scarf.sh/a.png?x-pxid=31890a02-9148-498c-8158-3e3db7f11422)
## Installation

Run:

```bash
npm install -g fotingo
```

## Requirements

The first time that you run fotingo, it will ask you for some information:

- A GitHub personal access token. You can create one [here](https://github.com/settings/tokens). Only _repo_ permissions are needed
- Your Jira username
- Your Jira server root URL
- A Jira access token for your user. You can create one [here](https://id.atlassian.com/manage/api-tokens)

## Usage

<!-- prettier-ignore-start -->
<!-- usage -->
```sh-session
$ npm install -g fotingo
$ fotingo COMMAND
running command...
$ fotingo (-v|--version|version)
fotingo/4.0.0 linux-x64 node-v16.16.0
$ fotingo --help [COMMAND]
USAGE
  $ fotingo COMMAND
...
```
<!-- usagestop -->
<!-- prettier-ignore-end -->

## Commands

<!-- prettier-ignore-start -->
<!-- commands -->
* [`fotingo help [COMMAND]`](#fotingo-help-command)
* [`fotingo release RELEASE`](#fotingo-release-release)
* [`fotingo review`](#fotingo-review)
* [`fotingo start [ISSUE]`](#fotingo-start-issue)
* [`fotingo verify`](#fotingo-verify)

## `fotingo help [COMMAND]`

display help for fotingo

```
USAGE
  $ fotingo help [COMMAND]

ARGUMENTS
  COMMAND  command to show help for

OPTIONS
  --all  see all commands in CLI
```

_See code: [@oclif/plugin-help](https://github.com/oclif/plugin-help/blob/v3.2.18/src/commands/help.ts)_

## `fotingo release RELEASE`

Create a release with your changes

```
USAGE
  $ fotingo release RELEASE

ARGUMENTS
  RELEASE  Name of the release to be created

OPTIONS
  -i, --issues=issues  Specify more issues to include in the release
  -n, --noVcsRelease   Do not create a release in the remote VCS
  -s, --simple         Do not use any issue tracker
  -y, --yes            Do not prompt for any input but accept all the defaults
```

_See code: [src/commands/release.ts](https://github.com/tagoro9/fotingo/blob/v4.0.0/src/commands/release.ts)_

## `fotingo review`

Submit current issue for review

```
USAGE
  $ fotingo review

OPTIONS
  -b, --branch=branch        Name of the base branch of the pull request
  -d, --draft                Create a draft pull request
  -l, --labels=labels        Labels to add to the pull request
  -r, --reviewers=reviewers  Request some people to review your pull request
  -s, --simple               Do not use any issue tracker
  -y, --yes                  Do not prompt for any input but accept all the defaults
```

_See code: [src/commands/review.ts](https://github.com/tagoro9/fotingo/blob/v4.0.0/src/commands/review.ts)_

## `fotingo start [ISSUE]`

Start working on an issue

```
USAGE
  $ fotingo start [ISSUE]

ARGUMENTS
  ISSUE  Id of the issue to start working with

OPTIONS
  -a, --parent=parent            Parent of the issue to be created
  -b, --branch=branch            Name of the base branch of the pull request
  -d, --description=description  Description of the issue to be created
  -k, --kind=kind                Kind of issue to be created
  -l, --labels=labels            Labels to add to the issue
  -n, --no-branch-issue          Do not create a branch with the issue name
  -p, --project=project          Name of the project where to create the issue
  -t, --title=title              Title of issue to create
```

_See code: [src/commands/start.ts](https://github.com/tagoro9/fotingo/blob/v4.0.0/src/commands/start.ts)_

## `fotingo verify`

Verify that fotingo can authenticate with the remote services

```
USAGE
  $ fotingo verify
```

_See code: [src/commands/verify.ts](https://github.com/tagoro9/fotingo/blob/v4.0.0/src/commands/verify.ts)_
<!-- commandsstop -->
<!-- prettier-ignore-end -->

## Configuration

Fotingo will try to guess most of the information based on the user environment, but there is some data that it still needs to be stored. On the first run, fotingo will create a `.fotingorc` configuration file inside your home directory. This file is used to store access tokens and some basic configuration information.

### Local configuration files

You can create project-specific configuration files. Just create a `.fotingorc` file inside your project root folder.
This file, needs to be in JSON format as well. You can also overwrite global configuration in this file. An example config file may just be:

```json
{
  "github": {
    "baseBranch": "develop"
  }
}
```

### Configuration properties

Fotingo will use as many defaults as possible to make it easier to use, but maybe you want to change some of the defaults. In that case, you can update any of the next properties in a fotingo configuration file

| Path                       | Description                                       | default                          |
| -------------------------- | ------------------------------------------------- | -------------------------------- |
| git.baseBranch             | Git base branch to use when creating new branches | master                           |
| git.branchTemplate         | Template used when creating a new branch          | See [templates](#templates)      |
| git.remote                 | Git remote to use                                 | origin                           |
| github.authToken           | Auth token to connect to Github                   | -                                |
| github.baseBranch          | Base branch to use to create PRs                  | master                           |
| github.owner               | Owner of the repository when creating a PR        | Extracted from remote            |
| github.pullRequestTemplate | Template to use when creating a PR                | See [templates](#templates)      |
| github.repo                | Name of the repository when creating a PR         | Extracted from remote            |
| jira.releaseTemplate       | Template to use when creating a release           | See [templates](#templates)      |
| jira.root                  | URL root to the Jira server                       | -                                |
| jira.status                | Regexes to identify workflow statuses             | See [jira status](#jira-status) |
| jira.user.login            | User login to connect to Jira                     | -                                |
| jira.user.token            | User token to connect to Jira                     | -                                |

### Templates

There are some configuration properties in fotingo that are template based, meaning that they can be customized to better suit your needs.

You can use `{` and `}` to interpolate the desired data. This is the data that is available in each template:

- `jira.releaseTemplate`

  - `version`. The version name specified through the CLI
  - `fixedIssuesByCategory`. Text that contains a list of the fixed issues by category
  - `fotingo.banner`. Banner that indicates that the release was created with fotingo

- `github.pullRequestTemplate`

  - `branchName`. Name of the branch
  - `changes`. List of the commit messages in the PR
  - `fixedIssues`. Text with the comma separated list of the fixed issues
  - `summary`. Pull request summary. Summary of the first Jira issue in the PR or first commit header if there are no fixed issues
  - `description`. Description of the first Jira issue in the PR or first commit body if there are no fixed issues
  - `fotingo.banner`. Banner that indicates that the release was created with fotingo

- `git.branchTemplate`

  - `issue.shortName`. A short name that represents a Jira issue type (e.g. _f_ for features).
  - `issue.key`. The key of the issue.
  - `issue.sanitizedSummary`. This is the summary of the issue, sanitized for use as a branch name.

### Jira status

Fotingo internally uses 5 status for an issue: `Backlog`, `Selected for Development`, `In progress`, `In review`, `Done`.
It automatically tries to map these statuses to Jira statuses, but sometimes projects may have simplified statuses in
Jira and fotingo won't be able to do the mapping automatically. If that is the case, the `jira.status` config can be
used to help fotingo do the mapping. Each entry should be a regex to map to that status:

```json
{
  "jira": {
    "status": {
      "BACKLOG": "backlog",
      "IN_PROGRESS": "in progress",
      "IN_REVIEW": "review",
      "DONE": "done",
      "SELECTED_FOR_DEVELOPMENT": "(todo)|(to do)|(selected for development)"
    }
  }
}
```

Multiple fotingo status can point to the same Jira status.

## Why fotingo?

When working on _Jira_ backed projects, I see a common pattern I repeat several times a day:

- Pick an issue in _Jira_ to work on
- Assign the issue to me and transition it to _in progress_
- Create a new branch in my local Git repository that follows certain naming conventions
- Do the work and commit some changes
- Create a _GitHub_ pull request with a description very similar to the ticket and a link back to the _Jira_ issue
- Set different _GitHub_ labels and request reviewers
- Set the _Jira_ issue state to _In Review_ and add a comment with the pull request URL
- Merge the PR and deploy the code via a CI server
- Create a Jira release and update the issue fix version and status
- Create a GitHub release that points back to Jira and has meaningful release notes

This seems like a reasonable workflow, but when addressing several issues on a given day, this process becomes very cumbersome. Thus... Fotingo.

## Debugging

If you run into problems, you can get more verbose output from the tool by adding:

    DEBUG="fotingo:*" fotingo ...

## Running locally

In order to run the tool locally you will have to clone the repo and then run:

    yarn link

After that, just run `yarn run watch` and the script will compile with every change you make to the source code.

## Troubleshooting

### `fotingo not found` when installing with yarn.

You need to have the directory where yarn installs global packages in your path. You can read more about that [here](https://github.com/yarnpkg/yarn/issues/648)
and [here](https://github.com/yarnpkg/yarn/issues/851). You just need to add `export PATH="$(yarn global bin | grep -o '/.*'):$PATH"`
to your `.bash_profile` or equivalent.

## Contributing

If you want to extend this tool with anything, feel free to submit a pull request.

## What is a fotingo?

This word in Spanish (Canary Islands) means _Rickety old car_, but what is more interesting is the word's origin: In 1908 the _Ford Motor Company_ released the _Ford Model T_ with the slogan of _foot 'n go_, as in, just put your "foot on the pedal and go". In some hispanic regions this morphed into the current version of _fotingo_. With a single command, you can put your _foot 'n go_ on your next issue.
