# Fotingo

A CLI to ease the interaction between *git*, *github* and *jira* when working on tasks.

[![Standard Version](https://img.shields.io/badge/release-standard%20version-brightgreen.svg)](https://github.com/conventional-changelog/standard-version)

## The problem

When working on projects or fixing bugs at my $company, I see a common pattern I repeat several times a day:

* Pick an issue in *jira* to work on.
* Assign the issue to me and transition it to *in progress*.
* Create a new branch in my local git repository that follows our naming conventions.
* Do the work and commit some changes.
* Create a *github* pull request with a description very similar to the ticket and a link back to the *jira* issue. [Hub](https://github.com/github/hub) could be used to create pull requests from the CLI but there is still a lot of text duplication.
* Set the *Needs review* label in the pull request and mention / assign some people to review it.
* Set the *jira* issue state to *In Review* and add a comment with the pull request URL.

This seems like a reasonable workflow, but when addressing several issues on a given day, this process becomes very cumbersome. Thus... Fotingo.

## The solution

Fotingo - A CLI tool that does all the work for me:

* `fotingo start <issue-id>` - Start working on a new issue.

  * Assign the issue to current user. _Halt if ticket is already assigned to other person or if the ticket is not assigned to my team._
  * Clean current working directory (stash it).
  * Checkout latest from master.
  * Create a new branch following naming convention.
  * Set the issue to *In Progress*.


* `fotingo review` - Submit a pull request for review.

  * _Halt if you are not in the correct branch or if the pull request already exists._
  * Rebase current branch onto the latest master.
  * Push the current brunch to *github*.
  * Create a new pull request against master with the messages of the commits and a link to the issue. _Default editor will open so user can edit message.*
  * Set *Needs review* label on the pull request.
  * Set issue to *In Review* and add comment with a link to the pull request.
  
Before building this tool I had been using a set of scripts inside the browser to build the branch name and 
[hub](https://github.com/github/hub) to create pull requests.  Unfortunately:

* I still had to manually assign issues to me and set them to *In Progress*.
* I still had to manually create a branch and copy the name from the browser.
* Pull requests could be created from the terminal, but if I have more than 1 commit, then there is no description created by default.
* I still had to manually add the issue link to the pull request description.
* I still had to manually set the issue to *Code review* and add a comment with the pull request.
* We could have setup web hooks to change status after creation of the pull request, but issue asignment, branch and pull request creation would still be manual tasks.
 
This tool also enforces a clean commit history, as commit messages will be the default description of the 
pull request.

The JIRA integration with Github helps you to do some of these tasks, such as: creating the branch (in github, but not locally) and updating the issue status if user has enabled the triggers.  But, the creation of pull requests with meaningful content, and linking them to the JIRA ticket (not the ticket to the PR) has to be done manually.

## Installation

`fotingo` has been developed using [yarn](https://github.com/yarnpkg/yarn), the new kid on the block,
so I highly recommend you give it a try and do:

    yarn add global fotingo
      
Otherwise, just go with the classic:

    npm install -g fotingo
    
Fotingo will create a `.fotingo` configuration file inside your home directory. This file is used to store
access tokens and some basic configuration information. For now, if you want to change something, you must edit this file.

An example file might look like this:

    {
      "git": {
        "remote": "origin",
        "branch": "master"
      },
      "github": {
        "owner": "tagoro9",
        "base": "master",
        "token": "github_token"
      },
      "jira": {
        "status": {
          "BACKLOG": 1,
          "SELECTED_FOR_DEVELOPMENT": 2,
          "IN_PROGRESS": 3,
          "IN_REVIEW": 4
        }
        "login": "jira_user",
        "password": "jira_password",
        "root": "https://issues.apache.org/jira"
      }
    }
    
Right now this file will store the username and password to your JIRA account in plaintext. I'm working on 
improving that.

### Local configuration files

You can create local configuration files to your project. Just create a `.fotingo` file inside your project root folder.
This file, needs to be in JSON format as well. If such file exist, any missing configuration will be added to this file instead.
You can also overwrite global configuration in this file. An example config file may just be:

    {
      "github": {
        "owner": "another-github-username"
      }
    }
    
## Usage

The command line supports the following commands: 
 
 * `fotingo --help` - to display usage information.
 * `fotingo start <issue-id>` - to start working on an issue.
 * `fotingo review` - to submit the current issue for review.  Additional options available (e.g. adding labels, not using an issue tracker) by using `fotingo review -h`

As of today, SSH  is the only supported authorization type for communicating with Github. Your SSH key should be loaded into the SSH agent (i.e. `ssh-add -k path-to-private-key`). Otherwise, the tool will enter an infinite loop and hang.

In order to use the tool, you need to have a password for JIRA.  Using SSO with your Google account will not work; you will have to create a generic user with a set password.

The first time the tool is run, it will ask for all the needed data to run. That includes:

* A github personal access token. You can create one [here](https://github.com/settings/tokens).
* The github account owner of the repositories.
* THe root to your Jira service.
* The ids of the issue status steps in jira. This depends on how the Jira workflows have been configured. Accessing them
through the API requires admin access, so it is not implemented for now. If you wanna get them, you can read more about
it [here](https://docs.atlassian.com/jira/REST/cloud/#api/2/issue-getIssue). You will need to provide the ids for the 
workflow status that represent the Backlog, To Do, In Progress and In Review steps. I'm working on automating this 
through the tool, but for now, it is a manual process.
* Your Jira username and password.

## Debugging

If you run into problems, you can get a more verbose output of the tool by adding:

    DEBUG=fotingo:* fotingo ...

## Running locally 

In order to run the tool locally you will have to clone the repo and then run:

    yarn link

After that, just run `gulp watch` and the script will compile with every change you make to the source code.

## Implementation details

My secondary goal when building this tool, apart from saving time, was to learn how to use  [ramda](http://ramdajs.com/), a functional programming library.  As a result, this tool heavily leverages [functional programming concepts](https://github.com/MostlyAdequate/mostly-adequate-guide). This may lead to difficulties understanding the code at first, but after a little research on these concepts (e.g currying, function composition, ...), it should become more clear.
 
Also, the code has been written in ES6 using [babel](https://babeljs.io/).

The CLI output has been inspired by yarn.

I've tried to comment the majority of the code, but I think it is pretty much self explanatory, save for some ramda tricks.

These scripts have been developed in my *free* time, if you find a bug please create an issue and I can try to fix it, but *patches are welcome*. 

## Things to do

There are still tons of things that I would like to do to improve this tool:

* ADD TESTS.
* Push current branch to Github.
* Allow changes to the configuration via the CLI.
* Switch from promises to generators or monads.
* Use JIRA OAuth instead of Basic authentication.
* Add support to request reviews to pull requests.
* Rebase current branch onto master. This may be tricky if there are conflicts.
* Lookup config file in parent folders.

## Contributing 

If you want to extend this tool with anything, feel free to submit a pull request.  There are a few things that I would like to keep this tool up-to-date:

* Functional programming. That means use Ramda everywhere.
* A clean commit history. I would encourage the use of the conventional changelog angular
[commit convention](https://github.com/conventional-changelog/conventional-changelog-angular/blob/master/convention.md).

## Troubleshooting

### Can't install fotingo

This library uses [nodegit](https://github.com/nodegit/nodegit) and it's installation may sometimes [fail](https://github.com/nodegit/nodegit/issues/1134).
In order to fix it, you could try to run:

  * In OSX: `sudo xcode-select --install`
  * In ubuntu: `sudo apt install libssl-dev`

### `fotingo not found ` when installing with yarn.

You need to have the directory where yarn installs global packages in your path. You can read more about that [here](https://github.com/yarnpkg/yarn/issues/648)
and [here](https://github.com/yarnpkg/yarn/issues/851). You just need to add `export PATH="$(yarn global bin | grep -o '/.*'):$PATH"`
to your `.bash_profile` or equivalent.

## What is a fotingo?

This word in Spanish (Canary Islands) means *Rickety old car*, but what is more interesting is the word's origin: In 1908 the *Ford Motor Company* released the *Ford Model T* with the slogan of *foot 'n go*, as in, just put your "foot on the pedal and go". In some hispanic regions this morphed into the current version of *fotingo*. With a single command, you can put your *foot 'n go* on your next issue.


