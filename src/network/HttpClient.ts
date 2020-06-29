import debug, { Debugger } from 'debug';
import * as R from 'ramda';
import request from 'request';
import { from, Observable } from 'rxjs';
import { promisify } from 'util';

import { HttpErrorImpl } from './HttpError';

const requestAsPromise = promisify(request);

interface HttpClientOptions {
  allowConcurrentRequests?: boolean;
  auth?: HttpBasicAuth;
  root: string;
  slowDownRequests?: boolean;
}

interface HttpBasicAuth {
  pass: string;
  user: string;
}

interface GetOptions {
  auth?: HttpBasicAuth;
  qs?: Record<string, unknown>;
}

interface PostOptions extends GetOptions {
  body?: Record<string, unknown>;
  form?: Record<string, unknown>;
}

export interface HttpResponse<T> {
  body: T;
  response: Record<string, unknown>;
}

enum HttpMethod {
  DELETE = 'delete',
  GET = 'get',
  POST = 'post',
  PUT = 'put',
}

const headers = { accept: 'application/json' };

type HttpMethodCall<T> = (path: string, options?: PostOptions) => Observable<HttpResponse<T>>;

export class HttpClient {
  public get: <T>(
    url: string,
    options?: GetOptions,
  ) => Observable<HttpResponse<T>> = this.serverCall(HttpMethod.GET);
  public post: <T>(
    url: string,
    options?: PostOptions,
  ) => Observable<HttpResponse<T>> = this.serverCall(HttpMethod.POST);
  public put: <T>(
    url: string,
    options?: PostOptions,
  ) => Observable<HttpResponse<T>> = this.serverCall(HttpMethod.PUT);

  private options: HttpClientOptions;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private networkQueue: Promise<any>;
  private debug: Debugger;

  constructor(options: HttpClientOptions) {
    this.debug = debug(`httpClient:${options.root}`);
    this.options = options;
    this.networkQueue = Promise.resolve();
  }

  private serverCall<T>(method: HttpMethod): HttpMethodCall<T> {
    return <T>(path: string, options: PostOptions = {}): Observable<HttpResponse<T>> => {
      const createRequest = (): Promise<HttpResponse<T>> => {
        this.debug(`Making ${method} call to ${path}`);
        return new Promise((resolve, reject) =>
          R.composeP(
            R.ifElse(
              R.propSatisfies(R.gt(400), 'statusCode'),
              R.compose(
                (data: HttpResponse<T>) => {
                  if (this.options.slowDownRequests) {
                    setTimeout(() => {
                      resolve(data);
                    }, 1000);
                  } else {
                    resolve(data);
                  }
                },
                R.tap(() => {
                  this.debug(`Finished ${method} call to ${path}`);
                }),
                R.unapply((functionArguments) => ({
                  response: functionArguments[0],
                  body: functionArguments[0].body,
                })),
              ),
              R.compose((response: request.Response) => {
                throw new HttpErrorImpl(response.statusMessage, response.statusCode, response.body);
              }),
            ),
            requestAsPromise,
          )({
            auth: options.auth || this.options.auth,
            body: options.body,
            form: options.form,
            headers,
            json: true,
            method,
            qs: options.qs,
            url: `${this.options.root}${path}`,
          }).catch((error: NodeJS.ErrnoException) => {
            this.debug(`Failed ${method} call to ${path}`);
            // TODO Transform error
            reject(error);
          }),
        );
      };
      if (this.options.allowConcurrentRequests) {
        return from(createRequest());
      }

      let outerResolve: (value: HttpResponse<T>) => void;
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      let outerReject: (reason?: any) => void;

      const promiseToReturn = new Promise<HttpResponse<T>>((resolve, reject) => {
        outerReject = reject;
        outerResolve = resolve;
      });

      this.networkQueue = this.networkQueue.then(() => {
        return createRequest().then(outerResolve).catch(outerReject);
      });
      return from(promiseToReturn);
    };
  }
}
