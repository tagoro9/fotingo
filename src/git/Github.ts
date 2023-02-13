import { Octokit, RestEndpointMethodTypes } from '@octokit/rest';
import { boundMethod } from 'autobind-decorator';
import { Debugger } from 'debug';
import envCi from 'env-ci';
import {
  compose,
  concat as rConcat,
  filter,
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
import { lastValueFrom } from 'rxjs';
import sanitizeHtml from 'sanitize-html';
import { serializeError } from 'serialize-error';
import { GithubErrorImpl, GithubRequestError } from 'src/git/GithubError';
import { cacheable, ONE_DAY } from 'src/io/cacheable';
import { debug } from 'src/io/debug';
import { editVirtualFile } from 'src/io/file';
import { maybeAskUserToSelectMatches } from 'src/io/input';
import { Messenger } from 'src/io/messenger';
import { Issue, Release, ReleaseNotes, RemoteUser } from 'src/types';
import { parseTemplate } from 'src/util/template';
import { findMatches } from 'src/util/text';

import { GithubConfig } from './Config';
import { BranchInfo, Git } from './Git';
import { JointRelease, Label, PullRequest, PullRequestData, Remote, Reviewer } from './Remote';

// Sanitize all the HTML tags in the passed string
const escapeHtml = (dirty: string) =>
  sanitizeHtml(dirty, {
    allowedAttributes: {},
    allowedTags: [],
  });

enum PR_TEMPLATE_KEYS {
  BRANCH_NAME = 'branchName',
  CHANGES = 'changes',
  DESCRIPTION = 'description',
  FIXED_ISSUES = 'fixedIssues',
  FOTINGO_BANNER = 'fotingo.banner',
  SUMMARY = 'summary',
}

interface SubmitPullRequestOptions {
  content: string;
  isDraft: boolean;
  pullRequestHead: string;
}

export class Github implements Remote {
  private readonly api: Octokit;
  private readonly config: GithubConfig;
  private readonly git: Git;
  private readonly messenger: Messenger;
  private readonly debug: Debugger;
  private readonly isCi: boolean;

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
    this.isCi = envCi().isCi;
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
    branchInfo,
    isDraft,
    issues,
    labels = [],
    reviewers = [],
    useDefaults,
  }: PullRequestData): Promise<PullRequest> {
    this.debug(`Creating pull requests with args: %o`, { labels, reviewers, useDefaults });
    this.messenger.emit('Creating pull request');
    const prExists = await this.doesPullRequestExistForBranch(branchInfo);
    if (prExists) {
      throw new Error('A PR already exists for this branch');
    }
    const [ghLabels, ghReviewers] = this.isCi
      ? [[], []]
      : await Promise.all([
          labels?.length > 0 ? this.getLabels() : [],
          reviewers?.length > 0 ? this.getPossibleReviewers() : [],
        ]);

    const foundLabels = findMatches({ fields: ['name'], data: ghLabels }, labels);

    const foundReviewers = reviewers.map(
      (reviewer) =>
        findMatches({ fields: ['login', 'name', 'email'], data: ghReviewers }, [reviewer])[0],
    );

    const selectedReviewers = this.isCi
      ? reviewers
      : await maybeAskUserToSelectMatches(
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

    const selectedLabels = this.isCi
      ? labels
      : await maybeAskUserToSelectMatches(
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

    const githubPr = await this.submitPullRequest({
      content: prContent,
      isDraft,
      pullRequestHead: branchInfo.name,
    });
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

    return pullRequest;
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
              RestEndpointMethodTypes['issues']['listLabelsForRepo']['response'],
              RestEndpointMethodTypes['issues']['listLabelsForRepo']['response']['data'],
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
    return this.listCollaborators()
      .then(
        compose(
          (
            promises: Array<
              Promise<RestEndpointMethodTypes['users']['getByUsername']['response']['data']>
            >,
          ) => Promise.all(promises),
          map(this.getUserInfo),
          map<{ login: string }, string>(prop('login')),
        ),
      )
      .then(
        compose(
          uniqBy(prop('login')),
          map<RestEndpointMethodTypes['users']['getByUsername']['response']['data'], Reviewer>(
            (user) => user as Reviewer,
          ),
          filter(
            (data: RestEndpointMethodTypes['users']['getByUsername']['response']['data']) =>
              data !== undefined && 'login' in data,
          ),
        ),
      );
  }

  /**
   * Enqueue a call to the Github API (or literally any promise)
   * so it is not executed until the previous call finished
   * @param call Call to queue
   * @param action Action that is being enqueued
   * @param actionArguments Extra parameters to include in the debug message
   */
  private queueCall<T>(
    call: () => Promise<T>,
    action?: string,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ...actionArguments: any[]
  ): Promise<T> {
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
        this.debug(action, ...actionArguments);
      }
      return call()
        .then((...resolvedValues) => {
          if (action) {
            this.debug(`Done with: ${action}`, ...actionArguments);
          }
          outerResolve(...resolvedValues);
        })
        .catch(compose(outerReject, this.mapError));
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
  private getUserInfo(
    username: string,
  ): Promise<RestEndpointMethodTypes['users']['getByUsername']['response']['data'] | undefined> {
    return this.queueCall(
      () =>
        this.api.users
          .getByUsername({ username })
          .then(prop('data'))
          .catch((error) => {
            if (error.status !== 404) {
              throw error;
            }
            return undefined;
          }),
      `Getting user info for %s`,
      username,
    );
  }

  public getAuthenticatedUser(): Promise<RemoteUser> {
    return this.queueCall(
      () => this.api.users.getAuthenticated().then(prop('data')),
      `Getting authenticated user info`,
    );
  }

  /**
   * Return the repository URL for the current workspace
   */
  @boundMethod
  @cacheable({
    getPrefix: Github.getCachePrefix,
    minutes: 10 * ONE_DAY,
  })
  public getRepoUrl(): Promise<string> {
    return this.queueCall(
      () =>
        this.api.repos
          .get({
            owner: this.config.owner,
            repo: this.config.repo,
          })
          .then((data) => data.data.html_url),
      `Getting repo url`,
    );
  }

  /**
   * Submit a pull request for review
   * @param content Content of the pull request
   * @param isDraft Whether the pull request should be a draft
   * @param pullRequestHead Name of the branch to use as head of the pull request
   */
  private async submitPullRequest({
    content,
    isDraft,
    pullRequestHead,
  }: SubmitPullRequestOptions): Promise<
    RestEndpointMethodTypes['pulls']['create']['response']['data']
  > {
    // TODO This should not here and baseBranch should just be an argument to the constructor
    const baseBranch = await this.git.findBaseBranch(true);
    return this.api.pulls
      .create({
        base: baseBranch,
        body: compose<string, string[], string[], string>(join('\n'), tail, split('\n'))(content),
        draft: isDraft,
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
    reviewers: Reviewer[] | string[],
    pullRequest: PullRequest,
  ): Promise<RestEndpointMethodTypes['pulls']['requestReviewers']['response']['data']> {
    return this.api.pulls
      .requestReviewers({
        owner: this.config.owner,
        pull_number: pullRequest.number,
        repo: this.config.repo,
        reviewers: reviewers.map((reviewer: Reviewer | string) =>
          typeof reviewer === 'string' ? reviewer : reviewer.login,
        ),
      })
      .then((response) => response.data);
  }

  /**
   * Add labels to a Pull request
   * @param labels Labels
   * @param pullRequest Pull request
   */
  private async addLabels(
    labels: Label[] | string[],
    pullRequest: PullRequest,
  ): ReturnType<Octokit['issues']['addLabels']> {
    return this.api.issues.addLabels({
      issue_number: pullRequest.number,
      labels: labels.map((label: Label | string) =>
        typeof label === 'string' ? label : label.name,
      ),
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
  private async listCollaborators(): Promise<Array<{ login: string }>> {
    let contributors: Array<{ login: string }> = [];
    for await (const page of this.listAllCollaboratorPages()) {
      contributors = [...contributors, ...page];
    }
    return contributors;
  }

  /**
   * Expose all the pages for collaborators in the repo as a generator so we can consume them all
   * @private
   */
  private async *listAllCollaboratorPages(): AsyncGenerator<Array<{ login: string }>, void, void> {
    let page = 1;
    let hasNextPage = true;
    while (hasNextPage) {
      const contributors = await this.queueCall(
        () =>
          this.api.repos.listCollaborators({
            owner: this.config.owner,
            per_page: 100,
            page,
            repo: this.config.repo,
          }),
        `Getting page ${page} of contributors for ${this.config.owner}/${this.config.repo}`,
      );
      hasNextPage = contributors.headers?.link?.includes('rel="next"') || false;
      page += 1;
      yield compose(
        map<{ login: string }, { login: string }>(pick(['login'])),
        prop('data'),
      )(contributors);
    }
  }

  /**
   * Check if a PR already exists for the specified branch
   * @param branchInfo Branch information
   */
  private async doesPullRequestExistForBranch(branchInfo: BranchInfo): Promise<boolean> {
    const pullRequest = await this.getPullRequest(branchInfo);
    return pullRequest !== undefined;
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
      await lastValueFrom(this.messenger.pause());
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
        summary: take(100, `${issues[0].key}: ${issues[0].summary}`),
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

  async getPullRequest(branchInfo: BranchInfo): Promise<PullRequest | undefined> {
    const pullRequests = await this.queueCall(
      () =>
        this.api.pulls
          .list({
            head: `${this.config.owner}:${branchInfo.name}`,
            owner: this.config.owner,
            repo: this.config.repo,
          })
          .then((response) => response.data),
      'Checking if there is a PR for %s',
      branchInfo.name,
    );
    return pullRequests.length > 0
      ? { issues: [], number: pullRequests[0].number, url: pullRequests[0].html_url }
      : undefined;
  }

  private isGithubRequestError(error: Error): error is GithubRequestError {
    return 'name' in error && error.name === 'HttpError';
  }

  /**
   * Transform errors coming from github
   * known errors
   * @param error Error
   */
  @boundMethod
  private mapError(error: Error): Error {
    if (this.isGithubRequestError(error)) {
      const { status, ...rest } = error;
      // Don't serialize at the top to avoid the deprecation warning avoid accessing error.code
      this.debug(serializeError(rest));
      if (error.message === 'Bad credentials') {
        return new GithubErrorImpl(
          'Could not authenticate with Github. Double check that you credentials are correct.',
          status,
        );
      }
      return new GithubErrorImpl(error.message, status);
    }
    this.debug(serializeError(error));
    return error;
  }
}
