import 'jest';

import { Messenger } from 'src/io/messenger';
import { getReleaseNotes } from 'src/templates/getReleaseNotes';
import { data } from 'test/lib/data';

describe('getReleaseNotes', () => {
  test('generates the release notes from the template', async () => {
    const notes = await getReleaseNotes(
      data.createReleaseConfig(),
      (jest.fn() as unknown) as Messenger,
      data.createRelease(),
      true,
    ).toPromise();
    expect(notes).toMatchSnapshot();
  });
});
