# Fotingo (beta)

A CLI to ease the interaction between _Git_, _GitHub_ and _Jira_ when working on tasks.

[![Standard Version](https://img.shields.io/badge/release-standard%20version-brightgreen.svg)](https://github.com/conventional-changelog/standard-version)

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

The command line supports three main commands: `start`, `review` and `release`.

### start

`fotingo start [issue-id]` - Start working on a new issue.

If no `issue-id` is specified, then fotingo will display a list with all the tickets assigned to you.

- Assign the issue to current user
- Clean current working directory (stash it)
- Checkout latest master
- Create a new branch that follows a naming convention
- Set the issue to _In Progress_

If you want to set an issue in progress in Jira without creating a new branch, you can use the `-n` (`--no-branch-issue`) option.

If you want to create a new issue you can use the `-c` (`--create`) option. You will also have to specify the type (`-t` or `--type`) and project (`-p` or `--project`) where the issue is going to be created. Optionally you can also specify labels (`-l` or `--label`) and a description for the created issue (`-d` or `--description`).

The next command creates a bug in the `Test` project with the label `team-test`:

```bash
fotingo start -t But -p Test -l team-test -d "This is the description" -c "This is the title"
```

### review

`fotingo review -l mylabel -l "Another Label" -r github_user` - Submit a pull request for review.

- _Halt if you are not in the correct branch or if the pull request already exists_
- Push the current branch to _GitHub_
- Create a new pull request against master with the commit messages and a link to the issue. _Default editor will open so user can edit message_
- Add labels and review requests, if any, to the pull request
- Set issue to _In Review_ and add comment with a link to the pull request

Labels and reviews lookup is done using fuzzy search, meaning that if you want to add the `Needs tests` label to a PR you can just use `-l "tests"` and the correct label will be applied or you will be prompted to select the correct if multiple options match the search.

If you just want to create a pull request without connecting to Jira at all, you can use the option `-s` (`--simple`).

### release

`fotingo release <release-name> -i <issue-id> -i <another-issue-id>` - Creates a Github and Jira release

- Create a Jira version with the indicated name (e.g. `1.0.5`)
- Set the issues fix version to the newly created version
- Update the issues' status to _Done_
- Create a GitHub release pointing to the Jira release. _Default editor will open so user can edit the release notes_

If you just want to create a release in Github without connecting to Jira at all, you can use the `-s` (`--simple`) option.

### Using the defaults

By default fotingo will ask the user to edit the contents of pull requests and releases as well as to select the correct reviewer / label when there are multiple matches for the search. Using the `-y` (`--yes`) option you can skip these questions and tell fotingo to use the defaults / first matches.

### A different base branch

The default base branch for any fotingo command is `master`, but sometimes this is not true for some people. In that case, the option `-b` (`--branch`) allows to specify the base branch in every command. This config can also be made permanent via the configuration file. The branch lookup is also done via a fuzzy search like it is done for labels and reviewers

## Configuration

Fotingo will try to guess most of the information based on the user environment, but there is some data that it still needs to be stored. On the first run, fotingo will create a `.fotingorc` configuration file inside your home directory. This file is used to store access tokens and some basic configuration information.

### Local configuration files

You can create project-specific configuration files. Just create a `.fotingorc` file inside your project root folder.
This file, needs to be in JSON format as well. You can also overwrite global configuration in this file. An example config file may just be:

```js
{
  "github": {
    "baseBranch": "develop"
  }
}
```

### Configuration properties

Fotingo will use as many defaults as possible to make it easier to use, but maybe you want to change some of the defaults. In that case, you can update any of the next properties in a fotingo configuration file

| Path                       | Description                                       | default                     |
| -------------------------- | ------------------------------------------------- | --------------------------- |
| git.remote                 | Git remote to use                                 | origin                      |
| git.baseBranch             | Git base branch to use when creating new branches | master                      |
| git.branchTemplate         | Template used when creating a new branch          | See [templates](#templates) |
| github.authToken           | Auth token to connect to Github                   | -                           |
| github.baseBranch          | Base branch to use to create PRs                  | master                      |
| github.owner               | Owner of the repository when creating a PR        | Extracted from remote       |
| github.repo                | Name of the repository when creating a PR         | Extracted from remote       |
| github.pullRequestTemplate | Template to use when creating a PR                | See [templates](#templates) |
| jira.user.login            | User login to connect to Jira                     | -                           |
| jira.user.token            | User token to connect to Jira                     | -                           |
| jira.releaseTemplate       | Template to use when creating a release           | See [templates](#templates) |
| jira.root                  | URL root to the Jira server                       | -                           |

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
