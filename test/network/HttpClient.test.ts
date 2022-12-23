import { beforeEach, describe, expect, test } from '@jest/globals';
import nock, { DataMatcherMap } from 'nock';
import { lastValueFrom } from 'rxjs';
import { HttpClient } from 'src/network/HttpClient';

const mockRequestWithSuccess = (
  api: nock.Scope,
  statusCode: number,
  body: Record<string, unknown>,
  query: DataMatcherMap | boolean = true,
): void => {
  api.get('/').query(query).reply(statusCode, body);
};

describe('HttpClient', () => {
  let client: HttpClient;
  let api: nock.Scope;

  beforeEach(() => {
    const root = 'https://fotin.go';
    api = nock(root);
    client = new HttpClient({ root: root });
  });

  describe('get', () => {
    test('fetches JSON from the server', () => {
      const body = { key: 'value' };
      mockRequestWithSuccess(api, 200, body);
      return lastValueFrom(client.get('/')).then((value) => {
        expect(value.body).toEqual(body);
        expect(api.isDone()).toBe(true);
      });
    });

    test('passes the query string to the server', () => {
      mockRequestWithSuccess(api, 200, {}, { val: true });
      return lastValueFrom(client.get('/', { qs: { val: true } })).then(() => {
        expect(api.isDone()).toBe(true);
      });
    });

    test('throws an error when the call fails', () => {
      const body = { error: 'message' };
      mockRequestWithSuccess(api, 400, body);
      return lastValueFrom(client.get('/')).catch((error) => {
        // eslint-disable-next-line jest/no-conditional-expect
        expect(api.isDone()).toBe(true);
        // eslint-disable-next-line jest/no-conditional-expect
        expect(error.body).toEqual(body);
      });
    });
  });
});
