import { afterEach, beforeEach, describe, expect, it, jest } from '@jest/globals';
import { pick } from 'ramda';
import { lastValueFrom, of, throwError } from 'rxjs';
import { serializeError } from 'serialize-error';
import { Messenger } from 'src/io/messenger';
import { Jira } from 'src/issue-tracker/jira/Jira';
import * as httpClient from 'src/network/HttpClient';
import { data } from 'test/lib/data';

jest.mock('src/network/HttpClient');

const httpClientMock = jest.mocked(httpClient.HttpClient);

const httpClientMocks = {
  get: jest.fn(),
};

let jira: Jira;

describe('jira', () => {
  beforeEach(() => {
    httpClientMock.mockImplementation(() => httpClientMocks as unknown as httpClient.HttpClient);
    jira = new Jira(data.createTrackerConfig(), new Messenger());
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('getCurrentUser', () => {
    it('should get the current user', async () => {
      httpClientMocks.get.mockReturnValue(of(data.createHttpResponse(data.createJiraUser())));
      const user = await lastValueFrom(jira.getCurrentUser());
      expect(user).toMatchSnapshot();
      expect(httpClientMocks.get).toHaveBeenCalledWith('/myself', {
        qs: { expand: 'groups' },
      });
    });

    it('should map the jira errors', async () => {
      httpClientMocks.get.mockReturnValue(
        throwError({
          body: {
            errorMessages: ['There was an error'],
          },
          status: 404,
        }),
      );
      try {
        await lastValueFrom(jira.getCurrentUser());
      } catch (error) {
        // eslint-disable-next-line jest/no-conditional-expect
        expect(pick(['message', 'code'], serializeError(error))).toMatchSnapshot();
      }
    });

    it('should have a default message if there is none', async () => {
      httpClientMocks.get.mockReturnValue(
        throwError({
          body: {},
          status: 404,
        }),
      );
      try {
        await lastValueFrom(jira.getCurrentUser());
      } catch (error) {
        // eslint-disable-next-line jest/no-conditional-expect
        expect(pick(['message', 'code'], serializeError(error))).toMatchSnapshot();
      }
    });

    it('should forward any unknown error', async () => {
      httpClientMocks.get.mockReturnValue(throwError(new Error('Some error message')));
      try {
        await lastValueFrom(jira.getCurrentUser());
      } catch (error) {
        // eslint-disable-next-line jest/no-conditional-expect
        expect(pick(['message', 'code'], serializeError(error))).toMatchSnapshot();
      }
    });
  });

  describe('getIssue', () => {
    it('gets and transforms the issue from Jira', async () => {
      const jiraIssue = data.createJiraIssue({
        summary: `Issue with a lot of characters "$&'*,:;<>?@[]\`~‘’“”`,
      });
      httpClientMocks.get.mockReturnValue(of(data.createHttpResponse(jiraIssue)));
      const issue = await lastValueFrom(jira.getIssue(jiraIssue.key));
      expect(issue).not.toBeUndefined();
      expect(issue.sanitizedSummary).not.toContain(':');
      expect(issue).toMatchSnapshot();
      expect(httpClientMocks.get).toHaveBeenCalledWith(`/issue/${jiraIssue.key}`, {
        qs: { expand: 'transitions, renderedFields' },
      });
    });
  });
});
