jest.mock('request');
import 'jest';

import * as req from 'request';
import { HttpClient } from 'src/network/HttpClient';

const request = (req as unknown) as jest.Mock;

const mockRequestWithSuccess = (body: object, statusCode = 200): void =>
  void request.mockImplementation((_, cb) => {
    cb(null, { statusCode, body }, body);
  });

describe('HttpClient', () => {
  let client: HttpClient;
  beforeEach(() => {
    client = new HttpClient({ root: 'https://fotin.go' });
  });
  afterEach(() => {
    request.mockReset();
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
          expect(request).toHaveBeenCalledTimes(1);
          expect(request).toHaveBeenCalledWith(
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
          expect(request).toHaveBeenCalledWith(
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
        .catch((err) => {
          expect(err.body).toBe(body);
        });
    });
  });
});
