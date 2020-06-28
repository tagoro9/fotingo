import { Command } from '@oclif/command';
import { IConfig } from '@oclif/config';
import { existsSync, mkdirSync } from 'fs';
import { concat, prop, zipObj } from 'ramda';
import { empty, merge, Observable, ObservableInput, of, zip } from 'rxjs';
import { catchError, map, reduce } from 'rxjs/operators';
import { readConfig } from 'src/config';
import { enhanceConfig, enhanceConfigWithRuntimeArgs } from 'src/enhanceConfig';
import { BranchInfo, Git } from 'src/git/Git';
import { Github } from 'src/git/Github';
import { Emoji, Messenger } from 'src/io/messenger';
import { Jira } from 'src/issue-tracker/jira/Jira';
import { JiraError } from 'src/issue-tracker/jira/JiraError';
import { Tracker } from 'src/issue-tracker/Tracker';
import { Config, Issue, LocalChanges } from 'src/types';
import { renderUi } from 'src/ui/ui';

/**
 * Abstract class every fotingo command must extend. It provides
 * defaults and helper methods to make it easier to
 * create a command
 */
export abstract class FotingoCommand<T, R> extends Command {
  protected messenger: Messenger;
  protected fotingo: Config;
  protected tracker: Tracker;
  protected github: Github;
  protected git: Git;

  constructor(argv: string[], config: IConfig) {
    super(argv, config);
    this.createConfigFolder();
    this.messenger = new Messenger();
  }

  /**
   * Initialize the fotingo configuration with the values
   * from the configuration file and the inferred configuration
   * from the environment
   */
  private async initializeFotingoConfig(): Promise<void> {
    this.fotingo = readConfig();
    const enhancedConfig = await enhanceConfig(this.fotingo);
    this.fotingo = await enhanceConfigWithRuntimeArgs(enhancedConfig, {
      // TODO We need the flags to do this, but the flags require calling parse
      // branch: this.flags.branch,
      branch: 'master',
    });
  }

  /**
   * Create the fotingo config folder if it doesn't exist
   */
  private createConfigFolder(): void {
    const path = `${this.config.home}/.fotingo_config`;
    if (!existsSync(path)) {
      mkdirSync(path);
    }
  }

  async init(): Promise<any> {
    await this.initializeFotingoConfig();
    this.tracker = new Jira(this.fotingo.jira, this.messenger);
    this.git = new Git(this.fotingo.git, this.messenger);
    this.github = new Github(this.fotingo.github, this.messenger, this.git);
    return super.init();
  }

  async run(): Promise<void> {
    renderUi({
      cmd: () => this.runCmd(of(this.getCommandData())),
      isDebugging: process.env.DEBUG !== undefined,
      messenger: this.messenger,
    });
  }

  /**
   * Given the information for a branch, fetch from the tracker the information about
   * all the fixed issues
   * @param data
   */
  protected getLocalChangesInformation(
    data: [BranchInfo, { issues?: string[]; tracker: { enabled: boolean } }] | BranchInfo,
  ): ObservableInput<LocalChanges> {
    const [branchInfo, { issues = [], tracker }] = Array.isArray(data)
      ? data
      : [data, { issues: [], tracker: { enabled: true } }];
    if (!tracker.enabled) {
      return of({ branchInfo, issues: [] }) as ObservableInput<LocalChanges>;
    }

    const allIssues = concat(branchInfo.issues.map(prop('issue')), issues);

    if (allIssues.length > 0) {
      this.messenger.emit(`Getting information for ${allIssues.join(', ')}`, Emoji.BUG);
    }
    // TODO Use forkJoin
    return (zip(
      of(branchInfo),
      merge(
        ...allIssues
          .filter((issue) => this.tracker.isValidIssueName(issue))
          .map((issue) =>
            this.tracker.getIssue(issue).pipe(
              // TODO Rename JiraError
              catchError((error: JiraError) => {
                if (error.code === 404) {
                  return empty();
                }
                throw error;
              }),
            ),
          ),
      ).pipe(reduce<Issue, Issue[]>((accumulator, value) => accumulator.concat(value), [])),
    ).pipe(map(zipObj(['branchInfo', 'issues']))) as unknown) as ObservableInput<LocalChanges>;
  }

  protected abstract getCommandData(): R;

  protected abstract runCmd(commandData$: Observable<R>): Observable<T>;
}
