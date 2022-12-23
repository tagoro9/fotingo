import { flags } from '@oclif/command';
import { from, Observable } from 'rxjs';
import { map, switchMap, tap } from 'rxjs/operators';
import { branch } from 'src/cli/flags';
import { FotingoCommand } from 'src/cli/FotingoCommand';

interface InspectData {
  branch?: string;
  issue?: string;
}

export class Inspect extends FotingoCommand<void, InspectData> {
  static description =
    'Output information about the specified element. If no element is specified, output information about the execution context';
  static flags = {
    branch,
    issue: flags.string({
      char: 'i',
      description: 'Specify more issues to include in the release',
      required: false,
    }),
  };

  /**
   * Override validations so that we don't check if the passed branch exists
   * @param commandData$
   * @protected
   */
  protected getValidations(
    commandData$: Observable<InspectData>,
  ): [() => Observable<boolean>, string][] {
    return [this.validations.isGitRepo(commandData$)];
  }

  /**
   * Use raw output since this cmd outputs JSON data
   * @protected
   */
  protected useRawOutput(): boolean {
    return true;
  }

  protected getCommandData(): Observable<InspectData> | InspectData {
    return this.parse(Inspect).flags;
  }

  protected runCmd(commandData$: Observable<InspectData>): Observable<void> {
    return commandData$.pipe(
      switchMap((data) => {
        if (data.issue) {
          return this.tracker.getIssue(data.issue);
        }
        if (data.branch) {
          return from(this.git.getBranchInfo(data.branch));
        }
        return from(this.git.getBranchInfo()).pipe(switchMap(this.getLocalChangesInformation));
      }),
      tap((data) =>
        this.messenger.emit(`${JSON.stringify(data, undefined, 2)}\n`, undefined, true),
      ),
      map((_) => undefined),
    );
  }
}
