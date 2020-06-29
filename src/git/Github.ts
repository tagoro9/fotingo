import { Octokit } from '@octokit/rest';
import {
  IssuesListLabelsForRepoResponseData,
  OctokitResponse,
  PullsCreateResponseData,
  PullsRequestReviewersResponseData,
  UsersGetByUsernameResponseData,
} from '@octokit/types';
import { boundMethod } from 'autobind-decorator';
import { Debugger } from 'debug';
import escapeHtml from 'escape-html';
import {
  compose,
  concat as rConcat,
  head,
  join,
  map,
  mapObjIndexed,
  pick,
  prop,
  replace,
  split,
  tail,
  take,
  uniqBy,
} from 'ramda';
import { cacheable, ONE_DAY } from 'src/io/cacheable';
import { debug } from 'src/io/debug';
import { editVirtualFile } from 'src/io/file';
import { maybeAskUserToSelectMatches } from 'src/io/input';
import { Messenger } from 'src/io/messenger';
import { Issue, Release, ReleaseNotes } from 'src/types';
import { parseTemplate } from 'src/util/template';
import { findMatches } from 'src/util/text';

import { GithubConfig } from './Config';
import { BranchInfo, Git } from './Git';
import { JointRelease, Label, PullRequest, PullRequestData, Remote, Reviewer } from './Remote';

enum PR_TEMPLATE_KEYS {
  BRANCH_NAME = 'branchName',
  CHANGES = 'changes',
  DESCRIPTION = 'description',
  FIXED_ISSUES = 'fixedIssues',
  FOTINGO_BANNER = 'fotingo.banner',
  SUMMARY = 'summary',
}

export class Github implements Remote {
  private readonly api: Octokit;
  private readonly config: GithubConfig;
  private readonly git: Git;
  private readonly messenger: Messenger;
  private readonly debug: Debugger;

  /**
   * Get the prefix to use in the methods that need to be cached at the repository level
   */
  static getCachePrefix(this: Github): string {
    return `${this.config.owner}_${this.config.repo}`;
  }

  // Promise used to allow promise chaining and only run one
  // Github API call at a time to avoid exceeding the quotas
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private apiCallsQueue: Promise<any> = Promise.resolve();

  constructor(config: GithubConfig, messenger: Messenger, git: Git) {
    this.messenger = messenger;
    this.api = new Octokit({
      auth: `token ${config.authToken}`,
    });
    this.git = git;
    this.config = config;
    this.debug = debug.extend('github');
  }

  @boundMethod
  public async createPullRequest({
    labels = [],
    reviewers = [],
    branchInfo,
    issues,
    useDefaults,
  }: PullRequestData): Promise<PullRequest> {
    this.debug(`Creating pull requests with args: %o`, { labels, reviewers, useDefaults });
    this.messenger.emit('Creating pull request');
    const prExists = await this.doesPullRequestExistForBranch(branchInfo.name);
    if (prExists) {
      throw new Error('A PR already exists for this branch');
    }
    const [ghLabels, ghReviewers] = await Promise.all([
      this.getLabels(),
      this.getPossibleReviewers(),
    ]);

    const foundLabels = findMatches({ fields: ['name'], data: ghLabels }, labels);

    const foundReviewers = reviewers.map(
      (reviewer) =>
        findMatches({ fields: ['login', 'name', 'email'], data: ghReviewers }, [reviewer])[0],
    );

    const selectedReviewers = await maybeAskUserToSelectMatches(
      {
        data: foundReviewers,
        getLabel: (r) => {
          if (r.name) {
            return `${r.name} (${r.login})`;
          }
          return r.login;
        },
        getQuestion: (match) =>
          `We couldn't find a unique match for reviewer "${match}", which one best matches?`,
        getValue: (r) => r.login,
        options: reviewers,
        useDefaults,
      },
      this.messenger,
    );

    const selectedLabels = await maybeAskUserToSelectMatches(
      {
        data: foundLabels,
        getLabel: (l) => `${l.name}`,
        getQuestion: (match) =>
          `We couldn't find a unique match for labels "${match}", which one best matches?`,
        getValue: (l) => String(l.id),
        options: labels,
        useDefaults,
      },
      this.messenger,
    );

    const initialPrContent = this.getPullRequestContentFromTemplate(branchInfo, issues);
    this.messenger.inThread(true);
    await this.pause();
    const prContent = useDefaults
      ? initialPrContent
      : await editVirtualFile({
          extension: 'md',
          initialContent: initialPrContent,
          prefix: 'fotingo-review',
        });
    this.messenger.inThread(false);

    const githubPr = await this.submitPullRequest(prContent, branchInfo.name);
    const pullRequest = {
      issues,
      number: githubPr.number,
      url: githubPr.html_url,
    };
    await Promise.all([
      selectedReviewers.length > 0
        ? this.addReviewers(selectedReviewers, pullRequest)
        : Promise.resolve(undefined),
      selectedLabels.length > 0
        ? this.addLabels(selectedLabels, pullRequest)
        : Promise.resolve(undefined),
    ]);

    return Promise.resolve(pullRequest);
  }

  @cacheable({
    getPrefix: Github.getCachePrefix,
    minutes: ONE_DAY,
  })
  public getLabels(): Promise<Label[]> {
    return this.queueCall(
      () =>
        this.api.issues
          .listLabelsForRepo({
            owner: this.config.owner,
            repo: this.config.repo,
          })
          .then(
            compose<
              OctokitResponse<IssuesListLabelsForRepoResponseData>,
              IssuesListLabelsForRepoResponseData,
              Label[]
            >(map(pick(['id', 'name'])), prop('data')),
          ),
      `Getting labels for ${this.config.owner}/${this.config.repo}`,
    );
  }

  @boundMethod
  public async createRelease(release: Release, notes: ReleaseNotes): Promise<JointRelease> {
    const ghRelease = await this.api.repos.createRelease({
      body: notes.body,
      name: notes.title,
      owner: this.config.owner,
      repo: this.config.repo,
      tag_name: release.name,
    });
    return { release, remoteRelease: { id: ghRelease.data.id, url: ghRelease.data.html_url } };
  }

  @cacheable({
    getPrefix(this: Github) {
      return `${this.config.owner}_${this.config.repo}`;
    },
    minutes: ONE_DAY,
  })
  public getPossibleReviewers(): Promise<Reviewer[]> {
    return this.listContributors()
      .then(
        compose(
          (promises: Array<Promise<UsersGetByUsernameResponseData>>) => Promise.all(promises),
          map(this.getUserInfo),
          map<{ login: string }, string>(prop('login')),
        ),
      )
      .then(uniqBy(prop('login')));
  }

  /**
   * Enqueue a call to the Github API (or literally any promise)
   * so it is not executed until the previous call finished
   * @param call Call to queue
   * @param action Action that is being enqueued
   * @param actionArgs Extra parameters to include in the debug message
   */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private queueCall<T>(call: () => Promise<T>, action?: string, ...actionArgs: any[]): Promise<T> {
    let outerResolve: (value: T) => void;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let outerReject: (reason?: any) => void;
    const promiseToReturn = new Promise<T>((resolve, reject) => {
      outerReject = reject;
      outerResolve = resolve;
    });

    // TODO Test that errors don't break the queue
    this.apiCallsQueue = this.apiCallsQueue.then(() => {
      if (action) {
        this.debug(action, ...actionArgs);
      }
      return call()
        .then((...resolvedValues) => {
          if (action) {
            this.debug(`Done with: ${action}`, ...actionArgs);
          }
          outerResolve(...resolvedValues);
        })
        .catch(outerReject);
    });
    return promiseToReturn;
  }

  /**
   * Get the information for a user given their user name
   * @param username User name
   */
  @boundMethod
  @cacheable({
    minutes: 10 * ONE_DAY,
  })
  private getUserInfo(username: string): Promise<UsersGetByUsernameResponseData> {
    return this.queueCall(
      () => this.api.users.getByUsername({ username }).then(prop('data')),
      `Getting user info for %s`,
      username,
    );
  }

  /**
   * Submit a pull request for review
   * @param content Content of the pull request
   * @param pullRequestHead Name of the branch to use as head of the pull request
   */
  private async submitPullRequest(
    content: string,
    pullRequestHead: string,
  ): Promise<PullsCreateResponseData> {
    // TODO This should not here and baseBranch should just be an argument to the constructor
    const baseBranch = await this.git.findBaseBranch(true);
    return this.api.pulls
      .create({
        base: baseBranch,
        body: compose<string, string[], string[], string>(join('\n'), tail, split('\n'))(content),
        head: pullRequestHead,
        owner: this.config.owner,
        repo: this.config.repo,
        title: compose<string, string[], string>(head, split('\n'))(content),
      })
      .then((response) => response.data);
  }

  /**
   * Add reviewers to a pull request
   * @param reviewers Reviewers
   * @param pullRequest Pull request
   */
  private async addReviewers(
    reviewers: Reviewer[],
    pullRequest: PullRequest,
  ): Promise<PullsRequestReviewersResponseData> {
    return this.api.pulls
      .requestReviewers({
        owner: this.config.owner,
        pull_number: pullRequest.number,
        repo: this.config.repo,
        reviewers: map(prop('login'), reviewers),
      })
      .then((response) => response.data);
  }

  /**
   * Add labels to a Pull request
   * @param labels Labels
   * @param pullRequest Pull request
   */
  private async addLabels(
    labels: Label[],
    pullRequest: PullRequest,
  ): ReturnType<Octokit['issues']['addLabels']> {
    return this.api.issues.addLabels({
      issue_number: pullRequest.number,
      labels: labels.map((label) => label.name),
      owner: this.config.owner,
      repo: this.config.repo,
    });
  }

  /**
   * Get the list of collaborators for the current repo
   */
  @cacheable({
    getPrefix: Github.getCachePrefix,
    minutes: 5 * ONE_DAY,
  })
  private async listContributors(): Promise<Array<{ login: string }>> {
    const collaborators = await this.queueCall(
      () =>
        this.api.repos.listCollaborators({
          owner: this.config.owner,
          per_page: 100,
          repo: this.config.repo,
        }),
      `Getting contributors for ${this.config.owner}/${this.config.repo}`,
    );
    return compose(
      map<{ login: string }, { login: string }>(pick(['login'])),
      prop('data'),
    )(collaborators);
  }

  /**
   * Check if a PR already exists for the specified branch
   * @param branchName Branch name
   */
  private doesPullRequestExistForBranch(branchName: string): Promise<boolean> {
    // TODO check if this works with forks
    return this.queueCall(
      () =>
        this.api.pulls
          .list({
            head: `${this.config.owner}:${branchName}`,
            owner: this.config.owner,
            repo: this.config.repo,
          })
          .then((response) => response.data.length > 0),
      'Checking if there is a PR for %s',
      branchName,
    );
  }

  /**
   * Generate the pull request default content using the configured template with the data
   * from the branch and the issues
   * @param branchInfo Branch info
   * @param issues List of issues
   */
  private getPullRequestContentFromTemplate(branchInfo: BranchInfo, issues: Issue[]): string {
    const data = this.getPrSummaryAndDescription(branchInfo, issues);
    return parseTemplate<PR_TEMPLATE_KEYS>({
      data: mapObjIndexed(escapeHtml, {
        [PR_TEMPLATE_KEYS.CHANGES]: branchInfo.commits
          .reverse()
          .map((c) => `* ${c.header}`)
          .join('\n'),
        [PR_TEMPLATE_KEYS.FIXED_ISSUES]:
          issues.length > 0
            ? rConcat('Fixes ', issues.map((issue) => `[#${issue.key}](${issue.url})`).join(', '))
            : '',
        [PR_TEMPLATE_KEYS.BRANCH_NAME]: branchInfo.name,
        [PR_TEMPLATE_KEYS.DESCRIPTION]: data.description,
        [PR_TEMPLATE_KEYS.SUMMARY]: data.summary,
        [PR_TEMPLATE_KEYS.FOTINGO_BANNER]:
          '🚀 PR created with [fotingo](https://github.com/tagoro9/fotingo)',
      }),
      template: this.config.pullRequestTemplate,
    });
  }

  /**
   * Pause the execution if we are debugging Github
   */
  private async pause(): Promise<void> {
    if (this.debug.enabled) {
      await this.messenger.pause().toPromise();
    }
  }

  /**
   * Extract the summary and description for the pull request
   * @param branchInfo Branch info
   * @param issues List of issues
   */
  private getPrSummaryAndDescription(
    branchInfo: BranchInfo,
    issues: Issue[],
  ): { description: string; summary: string } {
    if (issues.length > 0) {
      return {
        description: replace(/\r\n/g, '\n', issues[0].description || ''),
        summary: take(60, `${issues[0].key}: ${issues[0].summary}`),
      };
    }
    if (branchInfo.commits.length > 0) {
      const firstCommit = branchInfo.commits[branchInfo.commits.length - 1];
      return {
        description: firstCommit.body || '',
        summary: firstCommit.header,
      };
    }
    return {
      description: '',
      summary: branchInfo.name,
    };
  }
}
