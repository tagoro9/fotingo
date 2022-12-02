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
import {
  BranchSummary,
  DefaultLogFields,
  LogResult,
  PushResult,
  SimpleGit,
  simpleGit,
  StatusResult,
} from 'simple-git';
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
  private readonly git: SimpleGit;
  private readonly config: GitConfig;
  private readonly messenger: Messenger;

  constructor(config: GitConfig, messenger?: Messenger) {
    this.git = simpleGit();
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
          // TODO Improve this error
          throw new Error('There is already a branch for the issue');
        }
      })
      .then(() => this.maybeStashChanges())
      .then(() => this.findBaseBranch())
      .then((baseBranch) => this.getLatestCommit(baseBranch).then((log) => log.latest?.hash))
      .then((lastCommitHash) =>
        this.git.checkoutBranch(branchName, lastCommitHash || this.config.baseBranch),
      )
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
      try {
        await this.git.push(this.config.remote);
      } catch (error) {
        this.mapAndThrowError(error);
      }
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
        throw new GitErrorImpl('The repository does not have a remote', GitErrorType.NO_REMOTE);
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
      .then((branches: BranchSummary) => branches.all.includes(branchName));
  }

  /**
   * Check if the current branch has a tracking branch in the configured remote
   */
  public async doesCurrentBranchExistInRemote(): Promise<boolean> {
    return this.git
      .revparse(['--abbrev-ref', '--symbolic-full-name', '@{u}'])
      .then(() => true)
      .catch((error: Error) => {
        if (/no upstream configured for branch/.test(error.message)) {
          return false;
        }
        return this.mapAndThrowError(error);
      });
  }

  /**
   * Get the default branch name in the configured remote
   */
  public async getDefaultBranch(): Promise<string> {
    return this.git
      .remote(['show', this.config.remote])
      .then((remoteInfo: string) => {
        const headBranch = /.*HEAD branch: (.+)\n/g.exec(remoteInfo);
        if (headBranch && headBranch[1]) {
          return headBranch[1];
        }
        throw new Error(`Could not find the default branch for ${this.config.remote}`);
      })
      .catch(this.mapAndThrowError);
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

  private async publish(): Promise<PushResult> {
    const branchName = await this.getCurrentBranchName();
    // eslint-disable-next-line unicorn/no-null
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
              map((reference: CommitReference) => ({
                issue: reference.issue,
                raw: `${reference.prefix}${reference.issue}`,
              })),
              filter(
                propSatisfies<string, CommitReference>((string) => /fixes/i.test(string), 'action'),
              ),
              prop('references') as (c: ParsedCommit) => CommitReference[],
            ),
          ),
          nthArg(1),
        ),
      ],
    )(issueId, commits);
  }

  @boundMethod
  private transformCommits(commits: DefaultLogFields[]): ParsedCommit[] {
    return map(compose(sync, compose(join('\n'), props(['message', 'body']))))(commits);
  }

  /**
   * Get the name for the current branch
   */
  public getCurrentBranchName(): Promise<string> {
    return this.git.revparse(['--abbrev-ref', 'HEAD']).catch(this.mapAndThrowError);
  }

  /**
   * Get the latest commit for the specified git reference
   * @param what Git reference
   */
  private getLatestCommit(what: string): Promise<LogResult> {
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
      return undefined;
    });
  }

  /**
   * Transform errors coming from simple-git into
   * known errors
   * @param error Error
   */
  private mapAndThrowError(error: Error): never {
    if (/A branch named .* already exists/.test(error.message)) {
      throw new GitErrorImpl(error.message, GitErrorType.BRANCH_ALREADY_EXISTS);
    }
    if (/not a git repository/.test(error.message)) {
      throw new GitErrorImpl(error.message, GitErrorType.NOT_A_GIT_REPO);
    }
    if (
      /Updates were rejected because the tip of your current branch is behind/.test(error.message)
    ) {
      throw new GitErrorImpl(error.message, GitErrorType.FORCE_PUSH);
    }
    throw error;
  }

  /**
   * Get the list of commits between the current branch HEAD
   * and the fotingo configured remote
   */
  private async getBranchCommitsFromMergeBase(): Promise<ReadonlyArray<DefaultLogFields>> {
    const baseBranch = await this.findBaseBranch();
    const reference = await this.git.raw(['merge-base', 'HEAD', baseBranch]);
    return this.git
      .log({
        from: 'HEAD',
        to: compose(trim, replace('\n', ''))(reference),
      })
      .then((data: LogResult) => data.all);
  }
}
