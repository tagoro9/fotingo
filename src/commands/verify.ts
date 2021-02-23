import { from, Observable, zip } from 'rxjs';
import { tap } from 'rxjs/operators';
import { FotingoCommand } from 'src/cli/FotingoCommand';
import { Emoji } from 'src/io/messenger';
import { RemoteUser, User } from 'src/types';

export class Verify extends FotingoCommand<[RemoteUser, User], void> {
  static description = 'Verify that fotingo can authenticate with the remote services';

  protected getCommandData(): Observable<void> | void {
    return undefined;
  }

  protected runCmd(_: Observable<void>): Observable<[RemoteUser, User]> {
    return zip(from(this.github.getAuthenticatedUser()), this.tracker.getCurrentUser()).pipe(
      tap(([remoteUser, trackerUser]) => {
        this.messenger.emit(
          `Logged in: "${trackerUser.displayName}" @ ${this.tracker.name} and "${remoteUser.login}" @ Github`,
          Emoji.OK,
        );
      }),
    );
  }
}
