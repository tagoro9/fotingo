import { concat, prop, zipObj } from 'ramda';
import { merge, ObservableInput, of, zip } from 'rxjs';
import { map, reduce } from 'rxjs/operators';
import { BranchInfo } from 'src/git/Git';
import { Emoji, Messenger } from 'src/io/messenger';
import { Tracker } from 'src/issue-tracker/Tracker';
import { Issue } from 'src/types';

export interface LocalChanges {
  branchInfo: BranchInfo;
  issues: Issue[];
}

/**
 * Given a tracker, generate a function that given the branch info and some extra issues,
 * it fetches the information from the tracker for all of them if the tracker is enabled
 * @param tracker Tracker
 */
export const getLocalChangesInformation = (issueTracker: Tracker, messenger: Messenger) => (
  data: [BranchInfo, { issues?: string[]; tracker: { enabled: boolean } }] | BranchInfo,
): ObservableInput<LocalChanges> => {
  const [branchInfo, { issues = [], tracker }] = Array.isArray(data)
    ? data
    : [data, { issues: [], tracker: { enabled: true } }];
  if (!tracker.enabled) {
    return of({ branchInfo, issues: [] }) as ObservableInput<LocalChanges>;
  }

  const allIssues = concat(branchInfo.issues.map(prop('issue')), issues);

  if (allIssues.length > 0) {
    messenger.emit(`Getting information for ${allIssues.join(', ')}`, Emoji.BUG);
  }
  // TODO Use forkJoin
  return (zip(
    of(branchInfo),
    merge(
      ...allIssues
        .filter(issue => issueTracker.isValidIssueName(issue))
        .map(issue => issueTracker.getIssue(issue)),
    ).pipe(reduce<Issue, Issue[]>((acc, val) => acc.concat(val), [])),
  ).pipe(map(zipObj(['branchInfo', 'issues']))) as unknown) as ObservableInput<LocalChanges>;
};
