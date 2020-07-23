import { Command } from '@oclif/command';
import { IConfig } from '@oclif/config';
import { boundMethod } from 'autobind-decorator';
import { existsSync, mkdirSync } from 'fs';
import { concat, lensPath, mergeDeepRight, path, prop, set, zipObj } from 'ramda';
import { empty, from, merge, Observable, ObservableInput, of, zip } from 'rxjs';
import { catchError, concatMap, map, reduce, tap } from 'rxjs/operators';
import { readConfig, requiredConfigs, writeConfig } from 'src/config';
import { enhanceConfig, enhanceConfigWithRuntimeArguments } from 'src/enhanceConfig';
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
    this.fotingo = await this.readConfig();
    const enhancedConfig = await enhanceConfig(this.fotingo);
    this.fotingo = await enhanceConfigWithRuntimeArguments(enhancedConfig, {
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

  /**
   * Read the configuration and ask the user to provide anny missing configuration
   */
  private async readConfig(): Promise<Config> {
    const initialConfig = readConfig();
    return new Promise((resolve) => {
      const ui = renderUi({
        cmd: () =>
          this.askForRequiredConfig(initialConfig).pipe(
            map((d) => mergeDeepRight(initialConfig, d) as Config),
            tap((config: Config) => {
              // TODO This is ugly but makes it that react state updates before unmounting
              setTimeout(() => {
                ui.unmount();
                resolve(config);
              }, 300);
            }),
          ),
        isDebugging: process.env.DEBUG !== undefined,
        messenger: this.messenger,
        showFooter: false,
      });
    });
  }

  /**
   * Ask the user for the required configurations that are currently missing and write them to the closest config
   * file
   */
  private askForRequiredConfig(config: Config): Observable<Partial<Config>> {
    // TODO requiredConfig should be configurable by each command
    return from(requiredConfigs.filter((cfg) => path(cfg.path, config) === undefined)).pipe(
      concatMap((requiredConfig) =>
        this.messenger
          .request(requiredConfig.request)
          .pipe(map((value) => [requiredConfig.path, value])),
      ),
      reduce<[string[], string], Partial<Config>>((accumulator, value) => {
        return set(lensPath(value[0]), value[1], accumulator);
      }, {}),
      map(writeConfig),
    );
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
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
  @boundMethod
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
