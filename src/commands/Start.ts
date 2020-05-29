import { compose, has, ifElse, pathEq, prop, tap as rTap, unapply, zipObj } from 'ramda';
import { Observable, of } from 'rxjs';
import { map, switchMap, tap, withLatestFrom } from 'rxjs/operators';
import { FotingoArguments } from 'src/commands/FotingoArguments';
import { Git } from 'src/git/Git';
import { maybeAskUserToSelectMatches } from 'src/io/input';
import { Emoji, Messenger } from 'src/io/messenger';
import { Jira } from 'src/issue-tracker/jira/Jira';
import { Tracker } from 'src/issue-tracker/Tracker';
import { CreateIssue, GetIssue, Issue, IssueStatus, IssueType } from 'src/types';

interface StartData {
  git: {
    createBranch: boolean;
  };
  issue?: CreateIssue | GetIssue;
}

interface IssueAndStartData {
  commandData: StartData;
  issue: Issue;
}

const getCommandData = (args: FotingoArguments): StartData => {
  let issue = undefined;
  if (args.create) {
    issue = {
      description: args.description as string,
      labels: args.labels as string[],
      project: args.project as string,
      title: args.issueTitle as string,
      type: args.type as IssueType,
    };
  } else if (args.issueId) {
    issue = { id: args.issueId as string };
  }
  return {
    git: {
      createBranch: !args.noBranchIssue,
    },
    issue,
  };
};

const getOrCreateIssue = (
  tracker: Tracker,
  messenger: Messenger,
): ((data: StartData) => Observable<Issue>) =>
  compose(
    ifElse(
      has('id'),
      compose(
        tracker.getIssue,
        rTap((id) => {
          messenger.emit(`Getting ${id} from Jira`, Emoji.BUG);
        }),
        prop('id'),
      ),
      compose(
        tracker.createIssueForCurrentUser,
        rTap(() => {
          messenger.emit('Creating issue in Jira', Emoji.BUG);
        }),
      ),
    ),
    // TODO We know issue will be defined here, but probably there is a better way
    // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
    (data: StartData) => data.issue!,
  );

const setIssueInProgress = (tracker: Tracker): ((data: Issue) => Observable<Issue>) =>
  compose((id: string) => tracker.setIssueStatus(IssueStatus.IN_PROGRESS, id), prop('key'));

const shouldCreateBranch = pathEq(['commandData', 'git', 'createBranch'], true);

const createBranch = (
  git: Git,
  messenger: Messenger,
): ((data: IssueAndStartData) => Promise<void>) =>
  compose(
    git.createBranchAndStashChanges,
    rTap((name) => {
      messenger.emit(`Creating branch ${name}`, Emoji.TADA);
    }),
    git.getBranchNameForIssue,
    prop('issue'),
  );

/**
 * Create a closure to ask the user to select an issue to start working
 * with (from the issues that are assigned to them). This only happens
 * if no issue was provided in the CLI
 * @param tracker Issue tracker
 * @param messenger Messenger
 */
const maybeAskUserToSelectIssue = (tracker: Tracker, messenger: Messenger) => (
  data: StartData,
): Observable<StartData> => {
  if (data.issue === undefined) {
    return tracker.getCurrentUserOpenIssues().pipe(
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
          messenger,
        ),
      ),
      map((issues) => issues[0]),
      map((issue) => ({ ...data, issue: { id: issue.key } })),
    );
  } else {
    return of(data);
  }
};

const justReturnTheIssue: (
  data: IssueAndStartData,
) => Observable<Issue> = compose((data: IssueAndStartData) => of(prop('issue', data)));

export const cmd = (args: FotingoArguments, messenger: Messenger): Observable<void | Issue> => {
  const tracker: Tracker = new Jira(args.config.jira, messenger);
  const git: Git = new Git(args.config.git, messenger);
  const commandData$ = of(args).pipe(map(getCommandData));
  return commandData$.pipe(
    // TODO We already have the issue here, it should not be fetched in the next cmd call again
    switchMap(maybeAskUserToSelectIssue(tracker, messenger)),
    switchMap(getOrCreateIssue(tracker, messenger)),
    tap((issue: Issue) => {
      messenger.emit(`Setting ${issue.key} in progress`, Emoji.BOOKMARK);
    }),
    switchMap(setIssueInProgress(tracker)),
    withLatestFrom(commandData$, unapply(zipObj(['issue', 'commandData']))),
    switchMap<IssueAndStartData, Observable<void | Issue>>(
      ifElse(shouldCreateBranch, compose(createBranch(git, messenger)), justReturnTheIssue),
    ),
  );
};
