import { afterEach, beforeEach, describe, expect, jest, test } from '@jest/globals';
import * as req from 'request';
import { HttpClient } from 'src/network/HttpClient';

jest.mock('request');
const mockRequest = (req as unknown) as ReturnType<typeof jest.fn>;

const mockRequestWithSuccess = (body: unknown, statusCode = 200): void =>
  void mockRequest.mockImplementation(
    (_: unknown, callback: (a: unknown, b: unknown, c: unknown) => void) => {
      callback(undefined, { statusCode, body }, body);
    },
  );

describe('HttpClient', () => {
  let client: HttpClient;
  beforeEach(() => {
    client = new HttpClient({ root: 'https://fotin.go' });
  });
  afterEach(() => {
    mockRequest.mockReset();
  });

  describe('get', () => {
    test('fetches JSON from the server', () => {
      const body = { key: 'value' };
      mockRequestWithSuccess(body);
      return client
        .get('/')
        .toPromise()
        .then((value) => {
          expect(value).toEqual({
            body,
            response: { statusCode: 200, body },
          });
          expect(mockRequest).toHaveBeenCalledTimes(1);
          expect(mockRequest).toHaveBeenCalledWith(
            {
              headers: { accept: 'application/json' },
              json: true,
              method: 'get',
              url: 'https://fotin.go/',
            },
            expect.any(Function),
          );
        });
    });

    test('passes the query string to the server', () => {
      mockRequestWithSuccess({});
      return client
        .get('/', { qs: { val: true } })
        .toPromise()
        .then(() => {
          expect(mockRequest).toHaveBeenCalledWith(
            expect.objectContaining({
              qs: { val: true },
            }),
            expect.any(Function),
          );
        });
    });

    test('throws an error when the call fails', () => {
      const body = { error: 'message' };
      mockRequestWithSuccess(body, 400);
      return client
        .get('/')
        .toPromise()
        .catch((error) => {
          expect(error.body).toBe(body);
        });
    });
  });
});
