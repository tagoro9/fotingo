import open from 'open';
import { always } from 'ramda';
import { from, Observable, throwError } from 'rxjs';
import { map, switchMap, tap, withLatestFrom } from 'rxjs/operators';
import { FotingoCommand } from 'src/cli/FotingoCommand';
import { maybeAskUserToSelectMatches } from 'src/io/input';
import { Emoji } from 'src/io/messenger';
import { Issue, LocalChanges, OpenData, PullRequest } from 'src/types';

export class Open extends FotingoCommand<void, OpenData> {
  static description =
    'Open the pull request or the jira ticket from the fotingo context in a browser';

  static args = [
    {
      default: 'jira',
      description: 'Source place that you want to open',
      name: 'source',
      options: ['jira', 'pr'],
      required: false,
    },
  ];

  protected getCommandData(): Observable<OpenData> | OpenData {
    const { args } = this.parse(Open);
    return { source: args.source };
  }

  protected runCmd(commandData$: Observable<OpenData>): Observable<void> {
    const getIssue = (changes: LocalChanges) =>
      !changes.issues || changes.issues.length === 0
        ? throwError(new Error('No issues found in the current branch'))
        : from(
            maybeAskUserToSelectMatches<Issue>(
              {
                allowTextSearch: true,
                data: [changes.issues],
                getLabel: (issue) => `${issue.key} (${issue.type}) - ${issue.summary}`,
                getQuestion: () => 'What ticket would you like to open?',
                getValue: (issue) => issue.url,
                limit: 15,
                useDefaults: false,
              },
              this.messenger,
            ),
          ).pipe(
            map((issues) => issues[0]),
            tap((issue) =>
              this.messenger.emit(
                `Opening browser for ${issue.key} - ${issue.url}`,
                Emoji.EARTH_AFRICA,
              ),
            ),
            switchMap((issue) => from(open(issue.url))),
          );

    const getPullRequest = (changes: LocalChanges) =>
      from(this.github.getPullRequest(changes.branchInfo)).pipe(
        map((pr) => {
          if (!pr) {
            throw new Error(`No pull request found for branch '${changes.branchInfo.name}'`);
          }
          return pr;
        }),
        tap((pr: PullRequest) =>
          this.messenger.emit(
            `Opening browser for PR #${pr.number} - ${pr.url}`,
            Emoji.EARTH_AFRICA,
          ),
        ),
        switchMap((pr) => from(open(pr.url))),
      );
    return commandData$.pipe(
      switchMap(() => this.git.getBranchInfo()),
      switchMap(this.getLocalChangesInformation),
      withLatestFrom(commandData$),
      switchMap(([changes, commandData]) => {
        if (commandData.source === 'pr') {
          return getPullRequest(changes);
        }
        return getIssue(changes);
      }),
      map(always(undefined)),
    );
  }
}
