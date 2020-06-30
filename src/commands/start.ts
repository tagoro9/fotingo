import { flags } from '@oclif/command';
import { boundMethod } from 'autobind-decorator';
import { compose, has, ifElse, pathEq, prop, tap as rTap, unapply, zipObj } from 'ramda';
import { Observable, of } from 'rxjs';
import { map, switchMap, tap, withLatestFrom } from 'rxjs/operators';
import { branch } from 'src/cli/flags';
import { FotingoCommand } from 'src/cli/FotingoCommand';
import { maybeAskUserToSelectMatches } from 'src/io/input';
import { Emoji } from 'src/io/messenger';
import { Issue, IssueStatus, IssueType, StartData } from 'src/types';

interface IssueAndStartData {
  commandData: StartData;
  issue: Issue;
}

export class Start extends FotingoCommand<Issue | void, StartData> {
  static description = 'Start working on an issue';

  static args = [
    {
      name: 'issue',
      required: false,
      description: 'Id of the issue to start working with',
    },
  ];

  static flags = {
    branch,
    'no-branch-issue': flags.boolean({
      char: 'a',
      default: false,
      description: 'Do not create a branch with the issue name',
      name: 'no-branch-issue',
    }),
    title: flags.string({
      char: 't',
      description: 'Title of issue to create',
      required: false,
      dependsOn: ['project', 'kind'],
    }),
    project: flags.string({
      char: 'p',
      description: 'Name of the project where to create the issue',
      required: false,
      dependsOn: ['kind', 'title'],
    }),
    kind: flags.string({
      char: 'k',
      description: 'Kind of issue to be created',
      required: false,
      dependsOn: ['title', 'project'],
    }),
    description: flags.string({
      char: 'd',
      description: 'Description of the issue to be created',
      required: false,
      dependsOn: ['title', 'project', 'kind'],
    }),
    labels: flags.string({
      description: 'Labels to add to the issue',
      char: 'l',
      multiple: true,
      required: false,
      dependsOn: ['title', 'project', 'kind'],
    }),
  };

  getCommandData(): StartData {
    const { args, flags } = this.parse(Start);
    let issue = undefined;
    if (flags.title) {
      issue = {
        description: flags.description as string,
        labels: flags.labels as string[],
        project: flags.project as string,
        title: flags.title as string,
        type: flags.kind as IssueType,
      };
    } else if (args.issue) {
      issue = { id: args.issue as string };
    }
    return {
      git: {
        createBranch: !flags['no-branch-issue'],
      },
      issue,
    };
  }

  /**
   * Get or create an issue in the tracker based on the passed arguments
   */
  @boundMethod
  private getOrCreateIssue(data: StartData): Observable<Issue> {
    return compose(
      ifElse(
        has('id'),
        compose(
          this.tracker.getIssue,
          rTap((id) => {
            this.messenger.emit(`Getting ${id} from ${this.tracker.name}`, Emoji.BUG);
          }),
          prop('id'),
        ),
        compose(
          this.tracker.createIssueForCurrentUser,
          rTap(() => {
            this.messenger.emit(`Creating issue in ${this.tracker.name}`, Emoji.BUG);
          }),
        ),
      ),
      prop('issue'),
    )(data);
  }

  /**
   * Set an issue in progress
   */
  @boundMethod
  private setIssueInProgress(issue: Issue): Observable<Issue> {
    return compose(
      (id: string) => this.tracker.setIssueStatus(IssueStatus.IN_PROGRESS, id),
      prop('key'),
    )(issue);
  }

  /**
   * Create a branch for the specified issue stashing all the changes
   */
  @boundMethod
  private createBranch(issueAndData: IssueAndStartData): Promise<void> {
    return compose(
      this.git.createBranchAndStashChanges,
      rTap((name) => {
        this.messenger.emit(`Creating branch ${name}`, Emoji.TADA);
      }),
      this.git.getBranchNameForIssue,
      prop('issue'),
    )(issueAndData);
  }

  /**
   * Create a closure to ask the user to select an issue to start working
   * with (from the issues that are assigned to them). This only happens
   * if no issue was provided in the CLI
   */
  @boundMethod
  private maybeAskUserToSelectIssue(data: StartData): Observable<StartData> {
    if (data.issue === undefined) {
      return this.tracker.getCurrentUserOpenIssues().pipe(
        switchMap((issues: Issue[]) =>
          maybeAskUserToSelectMatches<Issue>(
            {
              allowTextSearch: true,
              data: [issues],
              getLabel: (issue) => `${issue.key} (${issue.type}) - ${issue.summary}`,
              getQuestion: () => 'What ticket would you like to start working on?',
              getValue: (issue) => issue.key,
              limit: 15,
              useDefaults: false,
            },
            this.messenger,
          ),
        ),
        map((issues) => issues[0]),
        map((issue) => ({ ...data, issue: { id: issue.key } })),
      );
    } else {
      return of(data);
    }
  }

  shouldCreateBranch = pathEq(['commandData', 'git', 'createBranch'], true);

  justReturnTheIssue = compose((data: IssueAndStartData) => of(prop('issue', data)));

  runCmd(commandData$: Observable<StartData>): Observable<Issue | void> {
    return commandData$.pipe(
      // TODO We already have the issue here, it should not be fetched in the next cmd call again
      switchMap(this.maybeAskUserToSelectIssue),
      switchMap(this.getOrCreateIssue),
      tap((issue: Issue) => {
        this.messenger.emit(`Setting ${issue.key} in progress`, Emoji.BOOKMARK);
      }),
      switchMap(this.setIssueInProgress),
      withLatestFrom(commandData$, unapply(zipObj(['issue', 'commandData']))),
      switchMap<IssueAndStartData, Observable<void | Issue>>(
        ifElse(this.shouldCreateBranch, compose(this.createBranch), this.justReturnTheIssue),
      ),
    );
  }
}
