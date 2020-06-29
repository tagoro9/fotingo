import { flags } from '@oclif/command';
import { compose, has, ifElse, pathEq, prop, tap as rTap, unapply, zipObj } from 'ramda';
import { Observable, of } from 'rxjs';
import { switchMap, tap, withLatestFrom } from 'rxjs/operators';
import { branch } from 'src/cli/flags';
import { FotingoCommand } from 'src/cli/FotingoCommand';
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
      required: true,
      description: 'Id / Title of the issue to start working with',
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
    create: flags.boolean({
      char: 'c',
      default: false,
      description: 'Create a new issue instead of searching for it',
      required: false,
      dependsOn: ['project', 'type'],
    }),
    project: flags.string({
      char: 'p',
      description: 'Name of the project where to create the issue',
      required: false,
      dependsOn: ['type', 'create'],
    }),
    type: flags.string({
      char: 't',
      description: 'Type of issue to be created',
      required: false,
      dependsOn: ['create', 'project'],
    }),
    description: flags.string({
      char: 'd',
      description: 'Description of the issue to be created',
      required: false,
      dependsOn: ['create', 'project', 'type'],
    }),
    labels: flags.string({
      description: 'Labels to add to the issue',
      char: 'l',
      multiple: true,
      required: false,
      dependsOn: ['create', 'project', 'type'],
    }),
  };

  getCommandData(): StartData {
    const { args, flags } = this.parse(Start);
    return {
      git: {
        createBranch: !flags['no-branch-issue'],
      },
      issue: args.create
        ? {
            description: flags.description,
            labels: flags.labels,
            // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
            project: flags.project!,
            title: args.issue,
            type: flags.type as IssueType,
          }
        : { id: args.issue },
    };
  }

  /**
   * Get or create an issue in the tracker based on the passed arguments
   */
  getOrCreateIssue = compose(
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
  );

  /**
   * Set an issue in progress
   */
  setIssueInProgress = compose(
    (id: string) => this.tracker.setIssueStatus(IssueStatus.IN_PROGRESS, id),
    prop('key'),
  );

  /**
   * Create a branch for the specified issue stashing all the changes
   */
  createBranch = compose(
    this.git.createBranchAndStashChanges,
    rTap((name) => {
      this.messenger.emit(`Creating branch ${name}`, Emoji.TADA);
    }),
    this.git.getBranchNameForIssue,
    prop('issue'),
  );

  shouldCreateBranch = pathEq(['commandData', 'git', 'createBranch'], true);

  justReturnTheIssue = compose((data: IssueAndStartData) => of(prop('issue', data)));

  runCmd(commandData$: Observable<StartData>): Observable<Issue | void> {
    return commandData$.pipe(
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
