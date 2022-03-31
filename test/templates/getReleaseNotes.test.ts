import { describe, expect, jest, test } from '@jest/globals';
import { lastValueFrom } from 'rxjs';
import { Messenger } from 'src/io/messenger';
import { getReleaseNotes } from 'src/templates/getReleaseNotes';
import { data } from 'test/lib/data';

describe('getReleaseNotes', () => {
  test('generates the release notes from the template', async () => {
    const notes = await lastValueFrom(
      getReleaseNotes(
        data.createReleaseConfig(),
        jest.fn() as unknown as Messenger,
        data.createRelease(),
        true,
      ),
    );
    expect(notes).toMatchSnapshot();
  });
});
