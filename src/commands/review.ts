import { flags } from '@oclif/command';
import { boundMethod } from 'autobind-decorator';
import { mergeAll, zipObj } from 'ramda';
import { merge, Observable, of, zip } from 'rxjs';
import { map, reduce, switchMap, tap, withLatestFrom } from 'rxjs/operators';
import { branch, yes } from 'src/cli/flags';
import { FotingoCommand } from 'src/cli/FotingoCommand';
import { PullRequest } from 'src/git/Remote';
import { Emoji } from 'src/io/messenger';
import { Issue, IssueStatus, Review as FotingoReview, ReviewData } from 'src/types';

export class Review extends FotingoCommand<FotingoReview, ReviewData> {
  static description = 'Submit current issue for review';

  static flags = {
    branch,
    labels: flags.string({
      description: 'Labels to add to the pull request',
      char: 'l',
      multiple: true,
      required: false,
    }),
    reviewers: flags.string({
      description: 'Request some people to review your pull request',
      char: 'r',
      multiple: true,
      required: false,
    }),
    simple: flags.boolean({
      char: 's',
      description: 'Do not use any issue tracker',
      default: false,
      name: 'simple',
    }),
    yes,
  };

  getCommandData(): ReviewData {
    const { flags } = this.parse(Review);
    return {
      branch: flags.branch,
      labels: flags.labels,
      reviewers: flags.reviewers,
      tracker: {
        enabled: !flags.simple,
      },
      useDefaults: flags.yes,
    };
  }

  /**
   * Given a pull request, update all the fixed issues in the issue tracker
   * @param pullRequest Pull request
   */
  @boundMethod
  updateIssues(pullRequest: PullRequest): Observable<FotingoReview> {
    return (zip(
      of(pullRequest).pipe(
        tap((pr) => {
          if (pr.issues.length > 0) {
            this.messenger.emit(
              `Updating ${pr.issues.map((issue) => issue.key).join(', ')} on ${this.tracker.name}`,
              Emoji.BOOKMARK,
            );
          }
        }),
      ),
      merge(
        ...pullRequest.issues.map((issue) =>
          merge(
            this.tracker.setIssueStatus(IssueStatus.IN_REVIEW, issue.key),
            this.tracker.addCommentToIssue(issue.key, `PR: ${pullRequest.url}`),
          ),
        ),
      ).pipe(reduce<Issue, Issue[]>((accumulator, value) => accumulator.concat(value), [])),
    ).pipe(map(zipObj(['pullRequest', 'comments']))) as unknown) as Observable<FotingoReview>;
  }

  protected runCmd(commandData$: Observable<ReviewData>): Observable<FotingoReview> {
    return commandData$.pipe(
      switchMap(this.git.push),
      switchMap(this.git.getBranchInfo),
      withLatestFrom(commandData$),
      switchMap(this.getLocalChangesInformation),
      withLatestFrom(commandData$),
      map(mergeAll),
      switchMap(this.github.createPullRequest),
      switchMap(this.updateIssues),
      tap((data) =>
        this.messenger.emit(`Pull request created: ${data.pullRequest.url}`, Emoji.LINK),
      ),
    );
  }
}
