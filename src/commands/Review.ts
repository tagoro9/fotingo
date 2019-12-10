import { mergeAll, zipObj } from 'ramda';
import { merge, Observable, of, zip } from 'rxjs';
import { map, reduce, switchMap, tap, withLatestFrom } from 'rxjs/operators';
import { Git } from 'src/git/Git';
import { Github } from 'src/git/Github';
import { PullRequest, Remote } from 'src/git/Remote';
import { Emoji, Messenger } from 'src/io/messenger';
import { Jira } from 'src/issue-tracker/jira/Jira';
import { Tracker } from 'src/issue-tracker/Tracker';
import { Issue, IssueStatus } from 'src/types';

import { FotingoArguments } from './FotingoArguments';
import { getLocalChangesInformation } from './util';

interface ReviewData {
  labels?: string[];
  reviewers?: string[];
  branch?: string;
  useDefaults: boolean;
  tracker: {
    enabled: boolean;
  };
}

interface Review {
  pullRequest: PullRequest;
  comments: Issue[];
}

const getCommandData = (args: FotingoArguments): ReviewData => {
  return {
    branch: args.branch as string,
    labels: args.label as string[],
    reviewers: args.reviewer as string[],
    tracker: {
      enabled: !args.simple,
    },
    useDefaults: args.yes as boolean,
  };
};

const updateIssues = (jira: Tracker, messenger: Messenger) => (pullRequest: PullRequest) =>
  (zip(
    of(pullRequest).pipe(
      tap(pr => {
        if (pr.issues.length > 0) {
          messenger.emit(
            `Updating ${pr.issues.map(issue => issue.key).join(', ')} on Jira`,
            Emoji.BOOKMARK,
          );
        }
      }),
    ),
    merge(
      ...pullRequest.issues.map(issue =>
        merge(
          jira.setIssueStatus(IssueStatus.IN_REVIEW, issue.key),
          jira.addCommentToIssue(issue.key, `PR: ${pullRequest.url}`),
        ),
      ),
    ).pipe(reduce<Issue, Issue[]>((acc, val) => acc.concat(val), [])),
  ).pipe(map(zipObj(['pullRequest', 'comments']))) as unknown) as Observable<Review>;

export const cmd = (args: FotingoArguments, messenger: Messenger): Observable<Review> => {
  const git: Git = new Git(args.config.git, messenger);
  const jira: Tracker = new Jira(args.config.jira, messenger);
  const github: Remote = new Github(args.config.github, messenger, git);
  const commandData$ = of(args).pipe(map(getCommandData));
  return commandData$.pipe(
    switchMap(git.push),
    switchMap(git.getBranchInfo),
    withLatestFrom(commandData$),
    switchMap(getLocalChangesInformation(jira, messenger)),
    withLatestFrom(commandData$),
    map(mergeAll),
    switchMap(github.createPullRequest),
    switchMap(updateIssues(jira, messenger)),
    tap(data => messenger.emit(`Pull request created: ${data.pullRequest.url}`, Emoji.LINK)),
  );
};
