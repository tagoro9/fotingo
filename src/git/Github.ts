import { boundMethod } from 'autobind-decorator';
import {
  __,
  compose,
  concat as rConcat,
  flatten,
  head,
  join,
  map,
  pick,
  prop,
  replace,
  split,
  tail,
  take,
  uniqBy,
} from 'ramda';
import { cacheable, ONE_DAY } from 'src/io/cacheable';
import { editVirtualFile } from 'src/io/file-util';
import { Messenger } from 'src/io/messenger';
import { series } from 'src/io/promise-util';
import { parseTemplate } from 'src/io/template-util';
import { findMatches } from 'src/io/text-util';
import { Issue, Release } from 'src/issue-tracker/Issue';

import * as GithubApi from '@octokit/rest';

import { GithubConfig } from './Config';
import { BranchInfo } from './Git';
import { JointRelease, Label, PullRequest, PullRequestData, Remote, Reviewer } from './Remote';

interface AskToSelectMatchData<T> {
  data: T[][];
  options: string[];
  useDefaults: boolean;
  getQuestion: (item: string) => string;
  getLabel: (item: T) => string;
  getValue: (item: T) => string;
}

enum PR_TEMPLATE_KEYS {
  // TODO Use better names
  BRANCH_NAME = 'branchName',
  CHANGES = 'changes',
  FIXED_ISSUES = 'fixedIssues',
  FIRST_ISSUE_SUMMARY = 'firstIssue.summary',
  FIRST_ISSUE_DESCRIPTION = 'firstIssue.description',
  FOTINGO_BANNER = 'fotingo.banner',
}

export class Github implements Remote {
  private api: GithubApi;
  private config: GithubConfig;
  private messenger: Messenger;

  constructor(config: GithubConfig, messenger: Messenger) {
    this.messenger = messenger;
    this.api = new GithubApi({
      auth: `token ${config.authToken}`,
    });
    this.config = config;
  }

  @boundMethod
  public async createPullRequest({
    labels = [],
    reviewers = [],
    branchInfo,
    issues,
    useDefaults,
  }: PullRequestData): Promise<PullRequest> {
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
      reviewer =>
        findMatches({ fields: ['login', 'name', 'email'], data: ghReviewers }, [reviewer])[0],
    );

    const selectedReviewers = await this.maybeAskUserToSelectMatches({
      data: foundReviewers,
      getLabel: r => `${r.name} (${r.login})`,
      getQuestion: match =>
        `We couldn't find a unique match for reviewer "${match}", which one best matches?`,
      getValue: r => r.login,
      options: reviewers,
      useDefaults,
    });

    const selectedLabels = await this.maybeAskUserToSelectMatches({
      data: foundLabels,
      getLabel: l => `${l.name}`,
      getQuestion: match =>
        `We couldn't find a unique match for labels "${match}", which one best matches?`,
      getValue: l => String(l.id),
      options: labels,
      useDefaults,
    });

    const initialPrContent = this.getPullRequestContentFromTemplate(branchInfo, issues);
    this.messenger.inThread(true);
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
    getPrefix(this: Github) {
      return `${this.config.owner}_${this.config.repo}`;
    },
    minutes: ONE_DAY,
  })
  public getLabels(): Promise<Label[]> {
    return this.api.issues
      .listLabelsForRepo({
        owner: this.config.owner,
        repo: this.config.repo,
      })
      .then(
        compose(
          map(pick(['id', 'name']) as (label: any) => Label),
          prop('data'),
        ),
      );
  }

  @boundMethod
  public async createRelease(release: Release): Promise<JointRelease> {
    const ghRelease = await this.api.repos.createRelease({
      body: release.notes.body,
      name: release.notes.title,
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
    return this.listContributors().then(
      compose(
        (promises: Array<Promise<GithubApi.UsersGetByUsernameResponse>>) => Promise.all(promises),
        map(this.getUserInfo),
        map<{ login: string }, string>(prop('login')),
      ),
    );
  }

  /**
   * Get the information for a user given their user name
   * @param username User name
   */
  @boundMethod
  private getUserInfo(username: string): Promise<GithubApi.UsersGetByUsernameResponse> {
    return this.api.users.getByUsername({ username }).then(prop('data'));
  }

  /**
   * Given the options that a user selected and the found matches, ask the user to select
   * the best match out of the first 5 matches. Select the first match in the list if using defaults
   * or the list only has one element
   * @param options Options for selecting the matches
   * @param options.data Data found for the options introduced by the user
   * @param options.getLabel Function that given a match, returns the label to present to the user
   * @param options.getQuestion Function that returns the question to present to the user. It receives the option introduced by the user
   * @param options.getValue Function that given a match, returns its value (typically an id)
   * @param options.options List of options that the use introduced
   * @param options.useDefaults Flag indicating if the useDefaults options was set
   */
  private maybeAskUserToSelectMatches<T>({
    data,
    getLabel,
    getQuestion,
    getValue,
    options,
    useDefaults,
  }: AskToSelectMatchData<T>): Promise<T[]> {
    return series(
      data.map((matches, i) => () => {
        if (useDefaults || matches.length === 1) {
          return Promise.resolve(matches[0]);
        }
        return this.messenger
          .request(getQuestion(options[i]), {
            options: uniqBy<T, string>(getValue, take(5, matches)).map(r => ({
              label: getLabel(r),
              value: getValue(r),
            })),
          })
          .toPromise()
          .then(option => matches.find(r => String(getValue(r)) === String(option)));
      }),
    );
  }

  /**
   * Submit a pull request for review
   * @param content Content of the pull request
   * @param pullRequestHead Name of the branch to use as head of the pull request
   */
  private submitPullRequest(
    content: string,
    pullRequestHead: string,
  ): Promise<GithubApi.PullsCreateResponse> {
    return this.api.pulls
      .create({
        base: this.config.baseBranch,
        body: compose<string, string[], string[], string>(
          join('\n'),
          tail,
          split('\n'),
        )(content),
        head: pullRequestHead,
        owner: this.config.owner,
        repo: this.config.repo,
        title: compose<string, string[], string>(
          head,
          split('\n'),
        )(content),
      })
      .then(response => response.data);
  }

  /**
   * Add reviewers to a pull request
   * @param reviewers Reviewers
   * @param pullRequest Pull request
   */
  private async addReviewers(
    reviewers: Reviewer[],
    pullRequest: PullRequest,
  ): Promise<GithubApi.PullsCreateReviewRequestResponse> {
    return this.api.pulls
      .createReviewRequest({
        owner: this.config.owner,
        pull_number: pullRequest.number,
        repo: this.config.repo,
        reviewers: map(prop('login'), reviewers),
      })
      .then(response => response.data);
  }

  /**
   * Add labels to a Pull request
   * @param labels Labels
   * @param pullRequest Pull request
   */
  private async addLabels(labels: Label[], pullRequest: PullRequest) {
    return this.api.issues.addLabels({
      issue_number: pullRequest.number,
      labels: labels.map(label => label.name),
      owner: this.config.owner,
      repo: this.config.repo,
    });
  }

  /**
   * Get the list of contributors for the current repo. It includes the list of contributors
   * and collaborators
   */
  private async listContributors(): Promise<Array<{ login: string }>> {
    const groups = await Promise.all([
      this.api.repos.listCollaborators({
        owner: this.config.owner,
        per_page: 100,
        repo: this.config.repo,
      }),
      this.api.repos.listContributors({
        owner: this.config.owner,
        per_page: 100,
        repo: this.config.repo,
      }),
    ]);

    return compose(
      data => flatten<Array<Array<{ readonly login: string }>>>(data),
      map<{ data: Array<{ login: string }> }, Array<{ login: string }>>(
        compose(
          map<{ login: string }, { login: string }>(pick(['login'])),
          prop('data'),
        ),
      ),
    )(groups);
  }

  /**
   * Check if a PR already exists for the specified branch
   * @param branchName Branch name
   */
  private doesPullRequestExistForBranch(branchName: string): Promise<boolean> {
    // TODO check if this works with forks
    return this.api.pulls
      .list({
        head: `${this.config.owner}:${branchName}`,
        owner: this.config.owner,
        repo: this.config.repo,
      })
      .then(response => response.data.length > 0);
  }

  /**
   * Generate the pull request default content using the configured template with the data
   * from the branch and the issues
   * @param branchInfo Branch info
   * @param issues List of issues
   */
  private getPullRequestContentFromTemplate(branchInfo: BranchInfo, issues: Issue[]): string {
    return parseTemplate<PR_TEMPLATE_KEYS>({
      data: {
        [PR_TEMPLATE_KEYS.CHANGES]: branchInfo.commits
          .reverse()
          .map(c => `* ${c.header}`)
          .join('\n'),
        [PR_TEMPLATE_KEYS.FIXED_ISSUES]:
          issues.length > 0
            ? rConcat('Fixes ', issues.map(issue => `[#${issue.key}](${issue.url})`).join(','))
            : '',
        [PR_TEMPLATE_KEYS.BRANCH_NAME]: branchInfo.name,
        [PR_TEMPLATE_KEYS.FIRST_ISSUE_SUMMARY]:
          issues.length > 0
            ? take(60, `${issues[0].key}: ${issues[0].fields.summary}`)
            : branchInfo.name,
        [PR_TEMPLATE_KEYS.FIRST_ISSUE_DESCRIPTION]:
          issues.length > 0 ? replace(/\r\n/g, '\n', issues[0].fields.description || '') : '',
        [PR_TEMPLATE_KEYS.FOTINGO_BANNER]:
          '🚀 PR created with [fotingo](https://github.com/tagoro9/fotingo)',
      },
      template: this.config.pullRequestTemplate,
    });
  }
}
