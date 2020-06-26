import { boundMethod } from 'autobind-decorator';
import { CommitReference, ParsedCommit, sync } from 'conventional-commits-parser';
import createDebugger from 'debug';
import gitUrlParse from 'git-url-parse';
import {
  always,
  compose,
  concat,
  converge,
  filter,
  flatten,
  ifElse,
  isNil,
  join,
  map,
  nthArg,
  prop,
  props,
  propSatisfies,
  replace,
  trim,
  uniqBy,
} from 'ramda';
import { BranchSummary, ListLogSummary, SimpleGit, StatusResult } from 'simple-git';
import simpleGit from 'simple-git/promise';
import { maybeAskUserToSelectMatches } from 'src/io/input';
import { Emoji, Messenger } from 'src/io/messenger';
import { Issue } from 'src/types';
import { findMatches } from 'src/util/text';

import { getIssueId, getName } from './Branch';
import { GitConfig } from './Config';
import { GitErrorImpl, GitErrorType } from './GitError';
import { GitRemote } from './Remote';

const debug = createDebugger('fotingo:git');

interface Remote {
  name: string;
  refs: {
    fetch: string;
    push: string;
  };
}

interface GitLogLine {
  author_email: string;
  author_name: string;
  date: string;
  hash: string;
  message: string;
}
interface GitLog {
  all: ReadonlyArray<GitLogLine>;
  latest: GitLogLine;
  total: number;
}

export interface BranchInfo {
  commits: ParsedCommit[];
  issues: CommitIssue[];
  name: string;
}

interface CommitIssue {
  issue: string;
  raw: 'string';
}

export class Git {
  private git: SimpleGit;
  private config: GitConfig;
  private messenger: Messenger;

  constructor(config: GitConfig, messenger?: Messenger) {
    this.git = simpleGit().silent(true);
    this.config = config;
    // TODO This is error prone
    if (messenger) {
      this.messenger = messenger;
    }
  }

  /**
   * Given an issue, get the associated branch name
   * @param issue Issue
   */
  @boundMethod
  public getBranchNameForIssue(issue: Issue): string {
    debug(`creating branch name for ${issue.key}`);
    return getName(this.config, issue);
  }

  /**
   * Create a branch for the given name stashing any pending change
   * in the repo
   * @param branchName Branch na,e
   */
  @boundMethod
  public createBranchAndStashChanges(branchName: string): Promise<void> {
    return this.fetch()
      .then(() => this.doesBranchExist(branchName))
      .then((exists) => {
        if (exists) {
          // TODO Imrpove this error
          throw new Error('There is already a branch for the issue');
        }
      })
      .then(() => this.maybeStashChanges())
      .then(() => this.findBaseBranch())
      .then((baseBranch) => this.getLatestCommit(baseBranch).then((log) => log.latest.hash))
      .then((lastCommitHash) => this.git.checkoutBranch(branchName, lastCommitHash))
      .catch(this.mapAndThrowError);
  }

  /**
   * Push the current branch. If it doesn't exist on the remote, then publish it
   */
  @boundMethod
  public async push(): Promise<void> {
    const remoteExists = await this.doesCurrentBranchExistInRemote();
    if (remoteExists) {
      this.messenger.emit('Pushing branch', Emoji.ARROW_UP);
      this.messenger.inThread(true);
      await this.git.push(this.config.remote);
      this.messenger.inThread(false);
      return;
    }
    this.messenger.emit('Publishing branch', Emoji.ARROW_UP);
    this.messenger.inThread(true);
    await this.publish();
    this.messenger.inThread(false);
  }

  /**
   * Get full information about the current branch (including commits and fixed issues)
   */
  @boundMethod
  public async getBranchInfo(): Promise<BranchInfo> {
    const branchName = await this.getCurrentBranchName();
    const issueId = getIssueId(this.config, branchName);
    this.messenger.emit('Analyzing commit history', Emoji.MAG_RIGHT);
    const commits = await this.getBranchCommitsFromMergeBase().then(this.transformCommits);
    return {
      commits,
      issues: this.getIssues(commits, issueId),
      name: branchName,
    };
  }

  /**
   * Get the remote information for the given remote name
   * @param name Remote name
   */
  public getRemote(name: string): Promise<GitRemote> {
    return this.git.getRemotes(true).then((remotes: Remote[]) => {
      const origin = remotes.find((remote: Remote) => remote.name === name);
      const firstRemote = remotes[0];

      if (!origin && !firstRemote) {
        return Promise.reject();
      }

      return gitUrlParse((origin || firstRemote).refs.fetch);
    });
  }

  /**
   * Get the root dir of the repository
   */
  public getRootDir(): Promise<string> {
    return this.git
      .raw(['rev-parse', '--show-toplevel'])
      .then(compose(trim, replace('\n', '')))
      .catch(this.mapAndThrowError);
  }

  public async doesBranchExist(branchName: string): Promise<boolean> {
    return this.git
      .branchLocal()
      .then((branches: BranchSummary) => branches.all.some((branch) => branch === branchName));
  }

  /**
   * Check if the current branch has a tracking branch in the configured remote
   */
  public async doesCurrentBranchExistInRemote(): Promise<boolean> {
    return this.git
      .revparse(['--abbrev-ref', '--symbolic-full-name', '@{u}'])
      .then(() => true)
      .catch((e: Error) => {
        if (/no upstream configured for branch/.test(e.message)) {
          return false;
        }
        return this.mapAndThrowError(e);
      });
  }

  /**
   * Find the base branch based on the remote config and baseBranch prefix.
   * If none can be found, then throw an error
   */
  @boundMethod
  // TODO This is going to get called several times. It should be memoized
  public async findBaseBranch(removePrefix = false): Promise<string> {
    const branchPrefix = `remotes/${this.config.remote}`;
    const branches: Array<{ name: string }> = ((await this.git.branch(['-a'])) as BranchSummary).all
      .filter((b) => b.startsWith(branchPrefix))
      .map((branchName: string) => ({ name: branchName }));

    const matches = findMatches(
      {
        checkForExactMatchFirst: true,
        cleanData: (item) => item.replace(`${branchPrefix}/`, ''),
        data: branches,
        fields: ['name'],
      },
      [this.config.baseBranch],
    );

    if (matches.length === 0) {
      throw new Error(
        `Could not find a branch in ${this.config.remote} that matches ${this.config.baseBranch}. Make sure that the branch is published in the remote`,
      );
    }

    const baseBranch = await maybeAskUserToSelectMatches(
      {
        data: matches,
        getLabel: (branch) => branch.name.replace(`${branchPrefix}/`, ''),
        getQuestion: (item) =>
          `We couldn't find a unique match for the base branch "${item}", which one best matches?`,
        getValue: (branch) => branch.name,
        options: [this.config.baseBranch],
        useDefaults: false,
      },
      this.messenger,
    );

    const name = baseBranch[0].name;
    return removePrefix ? name.replace(`${branchPrefix}/`, '') : name;
  }

  private async publish(): Promise<void> {
    const branchName = await this.getCurrentBranchName();
    return this.git.push(this.config.remote, branchName, { '-u': null });
  }

  private getIssues(commits: ParsedCommit[], issueId?: string): CommitIssue[] {
    return converge(
      compose<CommitIssue[], CommitIssue[], CommitIssue[], CommitIssue[]>(
        uniqBy((issue: CommitIssue) => issue.issue),
        concat,
      ),
      [
        compose(
          ifElse(isNil, always([]), (key) => [{ raw: `#${key}`, issue: key }]),
          nthArg(0),
        ),
        compose(
          flatten,
          map(
            compose(
              map((ref: CommitReference) => ({
                issue: ref.issue,
                raw: `${ref.prefix}${ref.issue}`,
              })),
              filter(propSatisfies<string, CommitReference>((str) => /fixes/i.test(str), 'action')),
              prop('references') as (c: ParsedCommit) => CommitReference[],
            ),
          ),
          nthArg(1),
        ),
      ],
    )(issueId, commits);
  }

  @boundMethod
  private transformCommits(commits: GitLogLine[]): ParsedCommit[] {
    return map(compose(sync, compose(join('\n'), props(['message', 'body']))))(commits);
  }

  /**
   * Get the name for the current branch
   */
  private getCurrentBranchName(): Promise<string> {
    return this.git.revparse(['--abbrev-ref', 'HEAD']).catch(this.mapAndThrowError);
  }

  /**
   * Get the latest commit for the specified git reference
   * @param what Git reference
   */
  private getLatestCommit(what: string): Promise<GitLog> {
    return this.git.log(['-n1', what]);
  }

  /**
   * Fetch from the fotingo configured remote
   */
  private async fetch(): Promise<void> {
    this.messenger.inThread(true);
    await this.git.fetch(this.config.remote);
    this.messenger.inThread(false);
  }

  /**
   * Get the git status
   */
  private status(): Promise<StatusResult> {
    return this.git.status();
  }

  /**
   * Stash all the current changes (including untracked files). The stash name will
   * be auto generated by fotingo
   */
  private maybeStashChanges(): Promise<void> {
    return this.status().then((st) => {
      if (st.files.length > 0) {
        return this.git
          .stash(['save', '--include-untracked', 'Auto generated by fotingo'])
          .then(() => undefined);
      }
      return Promise.resolve();
    });
  }

  /**
   * Transform errors coming from simple-git into
   * known errors
   * @param e Error
   */
  private mapAndThrowError(e: Error): never {
    if (e.message.match(/A branch named .* already exists/)) {
      throw new GitErrorImpl(e.message, GitErrorType.BRANCH_ALREADY_EXISTS);
    }
    if (e.message.match(/not a git repository/)) {
      throw new GitErrorImpl(e.message, GitErrorType.NOT_A_GIT_REPO);
    }
    throw e;
  }

  /**
   * Get the list of commits between the current branch HEAD
   * and the fotingo configured remote
   */
  private async getBranchCommitsFromMergeBase(): Promise<ReadonlyArray<GitLogLine>> {
    const baseBranch = await this.findBaseBranch();
    const ref = await this.git.raw(['merge-base', 'HEAD', baseBranch]);
    return this.git
      .log({
        from: 'HEAD',
        to: compose(trim, replace('\n', ''))(ref),
      })
      .then((data: ListLogSummary) => data.all);
  }
}
